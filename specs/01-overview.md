# Spec 01 - Design Overview

> **Trigger model (decided 2026-05-16):** agon is a **deliberate
> CLI**. There is no auto-trigger, no Stop hook, no recursion guard,
> no `--hook-mode`. You run the binary (or a shell alias) in a
> terminal after a coding session; it forks via `--fork-session` and
> writes results to disk, so the root transcript is untouched by
> construction. This document has been pruned to that design; see
> [36](36-probe-userpromptsubmit-manual-trigger.md) for the evidence
> (no in-editor trigger can be byte-identical) and the decision trail.

The full design for `agon`. This document is the authoritative spec; the project README is a usage reference.

## What this is

A tool that productizes the debate architecture from [agents-byzantine-tolerance/specs/07-adversarial-debate.md](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/07-adversarial-debate.md). The upstream spec 07 measures whether debate works on coding tasks; this repo specifies a tool that bets it does, in a way that fails gracefully if 07's H6 (lazy critic collapses the architecture) bites.

## Goal

After a Claude Code coding session, run agon: a critic (Codex by default) produces concrete adversarial comments on the diff. Claude either fixes or defends each comment. Up to N rounds of cross-examination. Unresolved leaves surface to the human at the end as a structured review. The human inspects only what wasn't resolved by agon, not the full output.

## Versioning

The spec uses **v0** and **v1** as concrete release tiers, not vague handwaves.

**v0 - minimum viable, shippable artifact. Everything above the line.**

- `agon` CLI binary.
- **Claude-as-proposer mode only.** Codex-as-proposer is fully *described* in this spec (so the architecture has a target) but its implementation is v1.
- Manual CLI invocation (run the binary, or a shell alias, after a coding session). No auto-trigger.
- Aspect-specialized multi-critic with the four-aspect default (`functional-logic`, `security`, `code-quality`, `performance`).
- File-pointer channel; **no agon-content** in root JSONL; `--fork-session` always. agon only ever touches the live session through a fork and writes results to disk; it never resumes or injects into the root, so the root transcript is byte-unchanged by construction.
- Session persistence layout: `start.json`, per-fork `proposer-state.json` + round files, `attacks.jsonl`, `transcript.jsonl`, `summary.md`, `end.json`, plus cross-session `log.jsonl`.
- Stable `attack_id` ledger; contention-scored headline.
- `--changed-lines-min` trivial-diff gate.
- Cross-family default (claude proposer, codex critic); same-family requires explicit and different `--main-model`/`--side-model`.
- Forks run serially against the shared working tree.
- Conservative UX: no in-session live progress; summary on disk; user can `tail -f` round files in another terminal if they want progress.

**v0 release blockers (must clear before GA, not just before merge).**

- **Upstream 07a per-aspect critic-found-bug rate** ≥ 60% on at least two aspects (see Relationship to upstream research). Without this, the tool is hollow.

**v1 - natural enhancements once v0 is proven and 07a's per-aspect critic-found-bug rates are positive.**

- **Codex-as-proposer.** Stateless rounds with re-supplied context (architecture already in spec).
- **Per-fork git worktrees** via `claude --worktree`. Frozen working-tree snapshot per fork; eliminates outcome leakage between serial critics; enables parallel forks. Needs a concession-merge story.
- **Per-critic model configuration.** v0 uses one `--side-model` for all critics; v1 lets each critic specify its own model alongside its aspect.
- **Resume an old agon session.** `agon resume <session-id>` - re-open a prior agon's unresolved leaves after the human addresses them, run more rounds.
- **Live-progress UI / streaming TUI.** v0 is silent during the run and writes the summary to disk; a streaming progress view is a natural v1 enhancement.
- **Strict critic isolation** via per-fork sandbox temp directory. v0 enforces "diff + task only" by aspect prompt + `codex --sandbox read-only`; OS-level isolation (or restricted-cwd discipline that holds against misbehaving agents) is v1 work. See Critic isolation.

**Forever out of scope (non-goals, not "later"):**

- Skill / slash-command / plugin-template entry points (channel-constraint violation).
- Injecting an assistant turn or `systemMessage`/`additionalContext`-style notification into the root session via any mechanism (root-preservation invariant; probe-verified that `systemMessage` writes into the root JSONL).
- Auto-applying critic-suggested fixes. The proposer-clone makes any changes; the critic never edits.
- Style-only attacks.
- Plugin packaging unless multiple unrelated users ask for it.
- Training a better critic - use the off-the-shelf side agent; weak aspects get dropped from defaults.
- Adding extra critic tools beyond agent defaults - no MCP servers, no custom skills, no orchestrator-granted file access. The defaults that codex / claude already ship with are constrained by prompt + `codex --sandbox read-only`, not by OS isolation in v0 (see Critic isolation).

## Architecture

- **Proposer (P)** - the Claude Code session that just wrote code. Sees the original task and its own diff.
- **Critic(s) (C₁..Cₖ)** - k independent adversarial agents (Codex by default), each scoped to a different aspect (functional-logic, security, performance, …). Receives the diff + task context as prompt inputs; the "diff + task only" contract is enforced by aspect prompt discipline (and `codex --sandbox read-only` for codex critics) - see [Critic isolation](#critic-isolation-best-effort-in-v0). Strict OS-level isolation is v1.
- **Judge** - the human, but only at the end and only on unresolved leaves. Optional LLM-judge mode for triage.
- **Mediator** - a small orchestrator process that routes messages between P and the Cᵢ and tracks per-claim resolution state. This is the actual binary we ship.

### Fork model (load-bearing)

The agon must not pollute the root session. The user's original Claude Code conversation is the **root** - when agon ends, the user resumes from that root, with the working tree updated (any concession-fixes have landed) and a summary on disk. The conversation log of the root session never sees a agon turn.

Each critic gets its own **agon fork**: a clone of the root session paired with one critic process. Forks are isolated:

- **No transcript leakage between critics.** Each fork has its own (proposer-clone, critic) conversation pair. Critic A never sees critic B's attacks or the proposer's responses inside critic B's fork - the conversation logs are isolated.
- **Outcome leakage through the working tree is real and accepted.** When critic A's fork concedes a fix, that fix lands in the shared working tree. Critic B (running later, since v0 is serial) is shown the working-tree diff *as it stands at critic-B's fork start* (i.e. including critic-A's concessions), and that snapshot is captured to `forks/critic-<i>/diff.patch` for audit - so each critic's record reflects what it actually saw, not what was in `start.json`. This is the conservative trade-off: full per-fork isolation would require frozen working-tree snapshots per fork (e.g. `claude --worktree`), which is deferred (see Out of scope). Treat critic-B as reviewing "the code as it stands now," not "the code Claude wrote initially."
- **No agon-content pollution of the root.** The proposer in a fork is a clone of the root produced via Claude Code's built-in `--fork-session` flag. No agon turn, no agon text, no proposer-clone reply ever lands in the root's transcript. agon never runs `claude --resume <root>` without `--fork-session`, never injects a turn, and writes all output to disk + its own stdout - so the root JSONL is byte-identical after a run by construction. (agon is a separate process the user runs deliberately; nothing hooks into the live session.)
- **Serial execution in v0.** Forks run one at a time to avoid working-tree races. Parallel forks via per-fork git worktrees would also frozen-snapshot the tree per fork, eliminating outcome leakage; deferred until v0 is proven.

### When the proposer is Codex instead of Claude

Codex (0.128+) has session persistence and supports interactive `/fork` and `/resume` slash commands inside the TUI, plus `codex exec resume <id> "<prompt>"` for non-interactive resume. **But it has no non-interactive fork.** Verified by probe (2026-05, codex 0.128.0):

- `codex fork <id> "<prompt>"` errors with `"Error: stdin is not a terminal"` when invoked from a script - TUI-only entry point.
- `codex exec resume <id> "<prompt>"` works non-interactively but **modifies the resumed session in place** (the session file grows; `thread_id` does not change).
- `--ephemeral` does not save us: it suppresses persistence for *fresh* sessions only, not resumed ones. Resume-with-ephemeral still pollutes the resumed file.

|                                       | Claude (2.1.x)                                  | Codex (0.128+)                                 |
|---------------------------------------|-------------------------------------------------|------------------------------------------------|
| Non-interactive fork                  | `claude --resume <id> --fork-session -p ...` ✅ | None ❌ (`codex fork` requires TTY)            |
| Non-interactive resume                | `claude --resume <id> -p ...` ✅                | `codex exec resume <id> "<prompt>"` ✅         |
| Resume modifies resumed session?      | Only the fork (root preserved)                  | **Yes** - session file grows in place          |
| `--ephemeral` prevents that?          | N/A                                             | No (applies only to fresh sessions)            |
| Sandbox flag on resume                | `--permission-mode` accepted                    | `--sandbox` rejected on `exec resume` (inherits from parent) |

**Implication.** When `--main codex`, the orchestrator cannot create a non-mutating fork off the user's codex session. Instead, each defense round is a **stateless `codex exec`** with the full context re-supplied as the prompt:

```
codex exec --skip-git-repo-check --sandbox <mode> --json \
  "<task + current diff + all prior rounds in this fork>"
```

- Each round produces a fresh `thread_id` (no continuation of the prior round). The orchestrator carries fork state on disk in the round files and re-feeds it each round.
- No cache amortization across rounds - input tokens paid in full each time. This is the dominant cost difference vs. claude mode.
- The user's existing codex session (if any) is not touched: the orchestrator's `codex exec` calls don't resume any prior session.
- Capture `thread_id` from the first JSON event on stdout (`{"type":"thread.started","thread_id":"<uuid>"}`); the final response is in the `item.completed` event with `type: "agent_message"`.

The CLI's `--main` and `--side` flags remain symmetric across modes - any pairing is architecturally supported. **v0 ships only with `--main claude`; codex-as-proposer is v1** (per the Versioning section). Architectural contracts (channel constraint, file-pointer payload, contention scoring, headline output) are identical in both modes; only the proposer-driving mechanism differs.

#### Tree shape

```
root session  (user → assistant: "code written")  ←─── user resumes here when done
   │
   ├── fork-1: proposer-clone-1 ↔ critic-1
   │     └── rounds/r{1,2,3,...}-{critic,proposer}.md
   │
   ├── fork-2: proposer-clone-2 ↔ critic-2
   │     └── rounds/r{1,2,3,...}-{critic,proposer}.md
   │
   └── fork-3: proposer-clone-3 ↔ critic-3
         └── rounds/r{1,2,3,...}-{critic,proposer}.md
                                  │
                                  ▼
                            summary.md  (aggregated across forks)
                                  │
                                  ▼
                            stdout to user
```

Branches do not see each other. The user, looking at their Claude Code session, sees only the root. The agon lives entirely on disk and in the orchestrator's stdout. The proposer-clone sessions are artifacts - they remain in the session picker grouped under root (Claude Code groups forks under their root automatically) but the user can ignore them.

### Mechanism - how forks are created and driven

The orchestrator uses three Claude Code primitives, all already documented:

1. **Forking the proposer.** For each critic, the orchestrator runs:

   ```
   claude --resume <root-id> --fork-session --output-format json \
          -p "Some comments at @<path-to-rounds/r1-critic-i.md>. Please resolve or respond. If you disagree, please raise it."
   ```

   `--fork-session` creates a copy of the root conversation and switches into it. The pointer message is the first user turn in the fork. The JSON output includes the new fork session ID, which the orchestrator captures for subsequent rounds in this fork.

2. **Continuing within a fork.** Each subsequent round in the same fork uses:

   ```
   claude --resume <fork-id> -p "Critic re-attacks at @<path>. Please resolve or respond."
   ```

   No new fork - same fork, next user turn.

3. **Driving the critic.** Both Codex and Claude critics run **stateless per round**: each round is a fresh process; per-round inputs and outputs live on disk in the round files, not in a session. The `forks/critic-<i>/proposer-state.json` file holds proposer (not critic) continuity state.

   - **Codex critic** (the cross-family default). `codex exec --skip-git-repo-check --sandbox read-only --json "<aspect prompt + task + diff + pointers to prior round files>"`. `read-only` blocks writes/network; **it does not isolate which files codex can read** - see Critic isolation. Codex has no fork concept; each invocation is a fresh process. Reads prior proposer round files via codex's file-access tool. Capture `thread_id` from the first `thread.started` event for audit; round-to-round continuity is on disk, not via that id.

   - **Claude critic** (used in same-family `claude/claude` mode, see Heterogeneity). `claude -p "<aspect prompt + task + diff + pointers to prior round files>" --output-format json`. Fresh session per round - **no `--resume`, no `--fork-session`**. Freshness blocks the critic from inheriting any other session's conversation; it does **not** isolate which workspace files claude's file-access tools can read (the agent runs from the repo cwd; see Critic isolation). The "diff + task only" contract is enforced by aspect prompt discipline, not by OS isolation. Anthropic's 5-minute prompt cache amortizes the system-prompt prefix across rounds within one agon. Persistence: the critic writes its output to `r<n>-critic.md` (after the orchestrator parses + normalizes ids - see R1 attack); no per-critic session-id file is needed since each round is a fresh process.

The wrap-up step prints to stdout. agon never injects anything into the root session - output is the orchestrator's stdout plus files on disk, nothing else. (This is also why no in-editor trigger is offered: every Claude Code mechanism that could inject a turn or notification mutates the root transcript - probe-verified in [36](36-probe-userpromptsubmit-manual-trigger.md).)

The summary lives at `summary.md` and on the orchestrator's stdout; that's the contract. Users who want live progress can `tail -f` the session's `transcript.jsonl` (or the per-fork round files) in another terminal - that doesn't touch the root.

### Verified primitives (2026-05, claude 2.1.129)

The mechanism above was probed against a real Claude Code installation before this spec was finalized. Findings:

- **`claude --resume <root> --fork-session -p "..." --output-format json` works.** Returns a JSON result with a new `session_id` (different from root). Root JSONL is bit-identical preserved (mtime and size unchanged) across fork creation. ✅
- **`claude --resume <fork-id> -p "..."` continues the fork in place.** Same session_id across rounds (no fork-of-fork); fork JSONL grows; root JSONL still bit-identical. ✅
- **The fork inherits root's full conversation history.** When the fork is asked "what have you said so far," the response includes both the root's prior turns and the new fork turns. The proposer-clone has the original task + code in context, exactly as required. ✅
- **The fork JSONL is stored alongside root** under `~/.claude/projects/<encoded-cwd>/<fork-id>.jsonl`. ✅

#### Constraints uncovered by the probe (must inform implementation)

- **`--resume` is cwd-scoped.** Running `claude --resume <id>` from any cwd other than the one the session was created in returns *No conversation found with session ID*. agon must be run from the cwd that owns the root session; preflight enforces this (it errors out if the cwd doesn't match the session's). This is not optional.
- **`ANTHROPIC_API_KEY` env var pollution.** If set (even to a stale/invalid value), `claude -p` uses it instead of the OAuth keychain and fails with 401. agon unsets `ANTHROPIC_API_KEY` for the child agent processes it spawns. (This otherwise bites subprocesses inheriting env from a parent shell with a stale key.)
- **JSON output may contain control characters in `result`** that break naive parsers. The orchestrator must use a proper JSON library (Python `json`, Go `encoding/json`), not `jq` with raw shell pipes.
- **First-call cost.** First `claude -p` invocation in a fresh window primes a large system-prompt cache (~32k tokens, ~$0.20). Subsequent calls within the 5-minute cache window are cheap. Run all rounds of a agon in close succession to amortize.

### Channel constraint (load-bearing)

**Critic output must reach the proposer as a verbatim user message - no system-prompt wrapping, no skill framing, no slash-command template.** The proposer should react to critic comments the same way it would react to the human pasting a code review: full normal defense behavior, no "I'm in skill mode following steps" override.

This rules out skills, slash commands, and any plugin-layer prompt template as the delivery channel.

### Critic isolation (best-effort in v0)

The "diff + task context only" claim above is the architectural intent; in v0 it is **enforced by prompt discipline, not by OS isolation**. Both `codex exec` and `claude -p`, when invoked from the repo cwd, ship with file-access tools that can read any workspace file the OS permits - so a misbehaving critic could in principle read code well outside the diff.

What v0 does:

- **Aspect prompt discipline.** Each critic is system-prompted to focus on its single aspect for this diff and is instructed not to wander into unrelated code. The mediator drops attacks whose `location` falls outside the diff or files the diff directly references (heuristic, not perfect).
- **Codex critic: `--sandbox read-only`.** Blocks writes and network. Does *not* limit which files codex can read; this is defense in depth against side-effects, not file-scope isolation.
- **Claude critic: cwd + prompt only.** Claude has no CLI-level sandbox flag. Same-family `claude/claude` runs lean entirely on prompt discipline; a misbehaving claude critic could read anywhere the OS allows.

What v0 does **not** do:

- **Per-fork temp-dir sandbox** (running the critic from a directory that contains only `task.md`, `diff.patch`, and prior round files, with the prompt referencing files by relative path). Would enforce file-scope isolation against well-behaved agents. Deferred to v1.
- **OS-level isolation** (chroot, container, seccomp). Out of scope; the tool runs in the user's environment, not a sandbox.

Implication for the trust model: **the critic's "context starvation" is a property of the prompt, not the runtime.** A misbehaving critic could produce attacks against unrelated code; the reproduction-required rule (Rounds → R1 Attack) drops most rabbit-holes at parse time. The Risks section flags the residual.

### Payload via file reference

The verbatim user message stays short. The critic's full critique is written to disk, and the message just points at it:

```
Some comments at @.agon/sessions/<id>/rounds/r1-critic-1.md. Please resolve or respond. If you disagree, please raise it.
```

This is exactly how a human hands an agent a code review in normal Claude Code usage - `@file, please address`. The proposer's defense behavior is whatever Claude does with that pattern in any other session. No special protocol, no system-prompt-shaped instruction.

Three reasons this matters:

- **Token economy.** Long critiques don't bloat every defense round's context. The file is read on demand.
- **Audit trail.** Each round's critique is a durable artifact under `.agon/sessions/<id>/rounds/`, not a chat-buffer scrap.
- **Naturalism preserved.** The pointer message is short and natural; the critique text is whatever the critic wrote, untouched.

The mechanism end-to-end:

- **Critic → proposer.** Orchestrator captures the critic's raw output, parses it and normalizes attack ids (see Rounds → R1 Attack), writes the **normalized** version to `forks/critic-<i>/rounds/r<n>-critic.md`, then dispatches the pointer to the fork's proposer-clone (via `--fork-session` on the first round, `--resume <fork-id>` on subsequent rounds). The proposer always sees the same ids that the orchestrator's `attacks.jsonl` ledger uses.
- **Proposer → critic.** Symmetric. Orchestrator captures the proposer-clone's chat response (and notes which files were modified - concession-fixes show up as a diff in the working tree), writes them to `forks/critic-<i>/rounds/r<n>-proposer.md`, then dispatches a short pointer to the critic: `"Proposer responses at <path>. Review the defenses; for any unresolved attack, decide whether to re-attack or withdraw."` Codex reads the file via its file-access tool.

The pointer messages are the only orchestrator-authored text the agents see. Everything substantive - critique, defense, re-attack - lives in files and travels by reference.

### Rounds

Per fork (forks run serially in v0):

1. **R0 - Setup.** Orchestrator extracts task context from the root session's transcript and computes the initial working-tree diff (snapshot in `start.json`); a per-fork `forks/critic-<i>/diff.patch` is re-computed at each critic's R1 to capture what that critic actually saw (see Lifecycle invariants). In claude-as-proposer mode it will create the fork via `claude --resume <root-id> --fork-session ...` together with R1 (see below). In codex-as-proposer mode there is no fork; each round is a fresh `codex exec`.
2. **R1 - Attack.** Critic Cᵢ produces a structured attack list against the diff. Each attack carries:
   - `attack_id` - stable across rounds. The critic prompt asks for an id of the form `c<critic-index>-<seq>` (e.g. `c1-3`); the orchestrator validates and re-assigns deterministically if the critic skips or duplicates ids. The id is **the** identity of an attack across re-attacks, defenses, withdrawals, contention scoring, and the headline.
   - The leaf fields `{location, claim, expected violation, reproduction}`. Attacks lacking a reproduction are dropped at parse time.

   Orchestrator parses the critic's raw output, normalizes attack ids (deterministically reassigning skipped or duplicate ids), drops style-shaped attacks and attacks lacking a reproduction, and **only then** persists the normalized version to `forks/critic-<i>/rounds/r1-critic.md` - that file is what the proposer reads via the pointer message, so its ids must match `attacks.jsonl`. Appends one record per surviving attack to `attacks.jsonl` (status: `open`, rounds_survived: 0, re_attacked: false). Same parse-normalize-persist ordering applies to every later critic round (R3, R5, …).
3. **R2 - Defense.** Proposer-clone responds per attack-id with one of: `concede` (and apply fix in the working tree), `rebut` (with specific counter-evidence), `push-back` (request clarification - only allowed once per attack-id). Orchestrator persists the response to `forks/critic-<i>/rounds/r2-proposer.md` and updates each attack's record in `attacks.jsonl`.
4. **R3..R(max_turn) - Cross-examination.** Critic and proposer-clone alternate, addressing attacks by id. Re-attacks reuse the original `attack_id` and set `re_attacked = true`; new attacks get new ids; withdrawn attacks transition to `status = withdrawn`. Each round persisted to its file.
5. **Fork-wrap.** When this fork's termination condition fires, the per-fork ledger is final.

After all forks complete: aggregate across forks (attack_id is unique per critic-index, so cross-fork IDs don't collide), write `summary.md` and `end.json`, print to stdout. Root receives no agon content (see Fork model and Lifecycle invariants).

## Critic specialization

Multi-critic isn't a frill - it's the structural mitigation for lazy-critic risk. The debate-theoretic property (Irving 2018, building on PCP intuitions): **soundness needs one competent honest player, not all players honest**. Translated to multi-aspect critics: bugs in aspect *i* need only the critic on aspect *i* to be honest and competent. A lazy critic on aspect *j* doesn't break aspect *i* coverage.

This pushes the binding empirical question from "is THE critic competent?" (whole-tool risk) to "is the critic on aspect *X* competent?" (per-aspect risk). The latter is much more tractable: a weak aspect gets dropped from defaults without killing the tool.

### Default aspect set

The default `--aspect` list is a production-quality bar, not a pure bug-detection set:

- **functional-logic** - does it do what the task asked? Off-by-ones, missing branches, silent-failure paths, edge cases the spec implies but the code missed.
- **security** - input validation, authn/authz, injection (SQL, shell, template), data exposure, secrets in logs, unsafe deserialization.
- **code-quality** - maintainability red flags: long functions, swallowed exceptions, unclear naming *where it bites readability*, dead branches. **Not** style preferences.
- **performance** - algorithmic complexity, N+1 IO, unnecessary allocations or copies, blocking calls in hot paths.

Each critic is prompted with exactly one aspect, and its system prompt explicitly forbids cross-aspect attacks. The mediator drops attacks tagged with the wrong aspect at parse time. This narrows each critic's mandate, which is where lazy rubber-stamping hides - a generalist critic asked "review this" is much more likely to say "looks fine" than a specialist critic asked "find concrete security flaws in this diff."

### Aspect ≠ style

`code-quality` is not a license for stylistic gripes:

- **Code quality**: "This function silently swallows the exception, breaking the calling contract." (real correctness/maintainability impact)
- **Style**: "Use single quotes instead of double quotes." (no impact)

The default critic prompt requires every attack to name a concrete behavior or maintainability impact, not a preference. Style-shaped attacks are dropped at parse time.

### Aspect list is open

The four defaults cover typical backend coding work. Other teams might add `concurrency-safety` for systems code, `accessibility` for frontend, `api-compatibility` for libraries, `migration-safety` for schema/state changes. Aspect names are free-form and become part of the critic's system prompt; the mediator just uses them as routing labels and headline tags.

## Heterogeneity (proposer vs. critic)

The default pairing is cross-family: claude proposer, codex critic. Family asymmetry is the strongest form of independence - different training corpora, different RLHF objectives, different priors. It's the cleanest way to keep proposer and critic from rubber-stamping each other.

But sometimes only one agent is available, or a user wants same-family agon (claude/claude, codex/codex). The tool supports this with one hard constraint:

**When `--main` and `--side` are the same agent family, `--main-model` and `--side-model` must both be set and must differ.** Same model on both sides collapses to "the model debating itself" - same priors, same blind spots, no heterogeneity. The CLI errors out if either is unset or if they match.

Recommended same-family pairings:

- **claude/claude**: e.g. `--main-model claude-sonnet-4-6 --side-model claude-opus-4-7`. Different capability tiers or training generations.
- **codex/codex**: e.g. `--main-model gpt-5 --side-model o3`. Different model families inside codex.

Cross-family pairings don't require explicit model flags - the family difference already provides independence. Model flags are optional in those cases and default to each agent's CLI default.

For multi-critic (`--side-count > 1`), `--side-model` applies to all critics; aspect specialization provides the per-critic diversity. Per-critic model config is out of scope for v0.

## Build: the CLI binary

The channel constraint above eliminates most of the design space. Anything that wraps the critic's output in framing (skills, slash commands, plugin command templates) is out. agon is a single standalone CLI binary the user runs deliberately:

```
agon --max-turn=10 --main claude --side codex --side-count=3 \
       --aspect functional-logic,security,performance --session-id <claude-session>
```

In claude-as-proposer mode, the CLI **always injects into a fork, never the root**: `claude --resume <root> --fork-session -p "..."` for R1 (creates the fork and processes R1 in one shot), then `claude --resume <fork-id> -p "..."` for subsequent rounds in that same fork. Plain `claude --resume <root>` (without `--fork-session`) would append turns to the root and is forbidden. In codex-as-proposer mode the orchestrator runs fresh `codex exec` per round; codex has no non-mutating fork (see codex section). The user invokes it after a coding session (a shell alias keeps it one keystroke away), in CI, in batch scripts, or against a saved session - see Manual invocation.

### Rejected options (no in-editor trigger)

- **Slash command (`/agon`)**: violates the channel constraint - slash commands inject a template into the conversation. Even if the template only said "run the agon process," that's still a system-prompt-shaped artifact the proposer sees before the critic's text. (Current Claude Code unifies slash commands with skills; the rendered body is a persistent turn that survives compaction - non-zero footprint by construction.)
- **Skill (`agon-review` or `agon-defense`)**: same reason. Skills carry instructions Claude follows. The whole point is that Claude follows its *normal* coding-feedback instincts when responding to the critic, not a skill-specific methodology.
- **`UserPromptSubmit` sentinel hook (in-editor manual trigger)**: the only user-typed mechanism that runs out-of-band like the Stop hook. Probed (spec [36](36-probe-userpromptsubmit-manual-trigger.md), claude 2.1.143): a no-stdout hook that exits 2 to "erase" the sentinel still records the sentinel + a "blocked by hook" system line in root JSONL (~4 housekeeping lines, no review content, no `hook_*`). It satisfies the channel constraint but **cannot be byte-identical**. Conclusion: there is no in-editor byte-identical manual trigger; the on-demand path is CLI-only (see Trigger).
- **Plugin packaging**: premature productization. A single CLI binary doesn't need a plugin manifest. Revisit if a second user shows up.

## CLI surface

```
agon [--main claude] [--side codex] [--side-count 4]
       [--main-model <model>] [--side-model <model>]
       [--max-turn 6]
       [--aspect functional-logic,security,code-quality,performance]
       [--session-id <root-claude-session-id>]
       [--transcript <path-to-jsonl>]
       [--diff-from HEAD] [--diff-to .]
       [--task-context "<original task>"]
       [--judge none|llm|human]
       [--cost-cap 50000]
       [--changed-lines-min 10]
       [--state-dir .agon]
       [--format markdown|json]
```

Notes:

- `--session-id` is the **root** session ID (Claude-as-proposer mode only). The orchestrator forks from it for each critic via `claude --resume <root> --fork-session`. The root session is never modified. Without `--session-id`, the orchestrator falls back to fresh `claude -p` invocations per round (no proposer continuity within a fork - much more expensive). When `--main codex` (v1), this flag is ignored (codex has no non-mutating fork; see codex section).
- `--main-model` and `--side-model` are optional when `--main` and `--side` are different agent families (cross-family asymmetry suffices). When the families match, both flags are required and must differ - see Heterogeneity section. CLI errors out otherwise.
- The orchestrator must be invoked from the cwd that owns the root session - `claude --resume <id>` is cwd-scoped. The CLI errors out (preflight) if invoked from a different cwd than the one that owns the session.
- `--transcript` is optional but useful: pass the root session's JSONL path so the orchestrator can extract task context cheaply (no second `claude` call to inspect the session).
- `--side-count` and `--aspect` interact: if `--aspect a,b,c` is given with `--side-count 3`, each critic gets one aspect. If counts mismatch, error.
- `--max-turn` counts P+C exchanges combined per fork. 6 = 3 attack rounds + 3 defense rounds within one fork. With `side-count=3` and max-turn=6, the worst case is 18 round-exchanges total (serial across forks).
- `--task-context` is mandatory when neither `--session-id` nor `--transcript` is given. Otherwise the orchestrator extracts it from the first user turn in the transcript.
- `--cost-cap` is always enforced (default 50k tokens); when hit, the orchestrator aborts gracefully (surfaces partial review). Multi-critic multi-turn debates blow token budgets fast.
- `--changed-lines-min` is the trivial-diff gate. Below the threshold, the orchestrator prints one status line (`[agon] skipped: trivial diff`) and exits fast - cheap to run against a near-empty diff.
- Exit code 0 if zero unresolved leaves, 1 otherwise. Lets it gate CI.

## Trigger (manual, deliberate)

agon has **one** trigger: the user runs the CLI. There is no
auto-trigger and no in-editor trigger. Probe [36](36-probe-userpromptsubmit-manual-trigger.md)
(claude 2.1.143) established that *every* in-editor mechanism - slash
command, skill, or a `UserPromptSubmit` sentinel hook - mutates the
root JSONL (a slash/skill body is a persistent turn; a blocked
sentinel still records ~4 housekeeping lines). Running the binary
yourself in a terminal is the only way to trigger on demand without
touching the root: agon only ever reaches the live session through
`--fork-session` and writes everything else to disk, so the root
transcript is byte-identical after a run by construction.

```
agon --session-id <root-claude-session-id> --max-turn 6
```

A shell alias keeps it one keystroke away (agon resolves the latest
session for the cwd when `--session-id` is omitted):

```sh
alias agon-attack='agon --side-count 4 --max-turn 6'
```

Or with codex as proposer (v1; not available in v0):

```
agon --main codex --side claude --main-model gpt-5 --side-model claude-sonnet-4-6 \
       --task-context "$(< task.md)" --diff-from HEAD~1
```

Same path for CI gating, scripted batch runs, and reviewing a saved
session out-of-band.

### What the user sees

- **During the run.** No in-session UI (agon is a separate process,
  not attached to the Claude TUI). The user's terminal shows the
  orchestrator's per-fork / per-round progress on stderr unless
  `--log-mode silent` was passed; the run takes ~30s–3min.
- **At exit.** The orchestrator's last stdout line names the summary
  path (`.agon/sessions/<ts>-<id>/summary.md`) and a one-line entry
  is appended to `.agon/log.jsonl`. Zero unresolved → one quiet
  line, no surfaced summary. ≥1 unresolved → the summary path + the
  headline contention signal (see Surfacing rule).
- **Trivial diffs.** `--changed-lines-min` short-circuits before any
  agent runs: one status line, one `kind:"skipped"` `log.jsonl`
  entry, no session folder.
- **Cancellable.** Ctrl-C sends SIGINT through the process tree; the
  signal handler writes `end.json` with `terminated: interrupted`
  and exits non-zero, leaving a valid (truncated) record.
- **Live progress (optional).** `tail -f .agon/sessions/<latest>/forks/critic-*/rounds/r*-{critic,proposer}.md`
  (or `transcript.jsonl`) in another terminal. Outside the claude
  session; doesn't touch root.

## Session persistence

Each agon run is a session with explicit start and end markers and an on-disk record. This is what makes the orchestrator auditable and the "review unresolved later" workflow possible.

### Layout

Each invocation creates a folder under `--state-dir` (default `.agon/`):

```
.agon/
  log.jsonl                          # one line per agon run, appended at end
  sessions/
    <ISO8601>-<short-id>/
      start.json                     # timestamp, proposer-agent, root-session-id (claude only), task-context, *initial* diff snapshot (before any fork runs), config
      forks/                         # one folder per critic; "fork" is the conceptual unit even when the proposer is codex (no real session fork)
        critic-1/
          proposer-state.json        # agent-neutral; see schema below
          diff.patch                 # working-tree diff captured at THIS fork's start (after prior critics' concessions). What the critic actually saw.
          rounds/
            r1-critic.md             # critic 1's R1 attack list (full text)
            r2-proposer.md           # proposer's R2 defense
            r3-critic.md             # critic's response to R2
            ...                      # one file per (round, role)
        critic-2/
          proposer-state.json
          diff.patch
          rounds/
            ...
      transcript.jsonl               # append-only index across forks: pointers to round files
      attacks.jsonl                  # per-attack records aggregated across forks (keyed by attack_id)
      summary.md                     # human-facing summary, written at termination
      end.json                       # termination condition, stats, exit code
```

#### `proposer-state.json` schema

Agent-neutral file capturing whatever is needed to drive the proposer in subsequent rounds. The `agent` field discriminates:

```jsonc
// Claude-as-proposer
{
  "agent": "claude",
  "model": "claude-sonnet-4-6",         // null/absent = CLI default
  "fork_session_id": "<uuid>",          // captured from --fork-session JSON return on R1
  "root_session_id": "<uuid>"           // for audit; root is never resumed
}
```

```jsonc
// Codex-as-proposer (no native fork; each round is a fresh exec)
{
  "agent": "codex",
  "model": "gpt-5",                     // null/absent = CLI default
  "round_thread_ids": [                 // captured from each codex exec's "thread.started" event
    {"round": 2, "thread_id": "<uuid>"},
    {"round": 4, "thread_id": "<uuid>"}
    // ... one entry per even (proposer) round actually executed
  ]
}
```

In claude mode `fork_session_id` is load-bearing - round 3+ dispatch via `claude --resume <fork_session_id> -p ...`. In codex mode the field doesn't exist; `round_thread_ids` is purely informational (rounds run forward via fresh `codex exec`), useful for audit and for matching the persisted codex session files at `~/.codex/sessions/<date>/rollout-*-<thread_id>.jsonl` if a user wants to inspect them.

#### `attacks.jsonl` schema

One JSON object per line, one entry per state transition for an `attack_id`:

```jsonc
{
  "attack_id": "c1-3",                  // critic-index + sequence; stable across rounds
  "critic_index": 1,
  "aspect": "security",
  "round_introduced": 1,
  "round_last_touched": 3,
  "location": "src/api.py:88",
  "claim": "input not sanitized before LIKE pattern",
  "expected_violation": "SQL injection via search parameter",
  "reproduction": "GET /search?q=%' OR 1=1--",
  "status": "open",                      // open | conceded | rebutted | withdrawn | unresolved
  "rounds_survived": 2,
  "re_attacked": true,                   // critic re-engaged after a defense round
  "concession_files": ["src/api.py"],    // populated when status -> conceded
  "timestamp": "..."
}
```

Append-only - newer entries supersede older ones for the same `attack_id`. The orchestrator computes the final per-attack state by reading the file forward.

### Lifecycle invariants the orchestrator must enforce

The general rule: any file referenced by an `@<path>` pointer must exist on disk before the agent receives the pointer.

- `start.json` written atomically before any agent process spawns. Its `diff snapshot` is the *initial* working-tree diff, captured once. It is **not** the diff each critic saw.
- **Per-fork diff snapshot.** Before each fork's R1 critic run, the orchestrator computes the current working-tree diff (which includes any concession-fixes from prior critics in v0's serial mode) and writes it to `forks/critic-<i>/diff.patch`. This file is what the critic prompt references via `@diff.patch`-style pointers, and what audit tools should consult to know "what did critic-i actually attack." `start.json`'s diff is for run-level provenance only.
- **R1 ordering is the unique case (claude-as-proposer).** Orchestrator runs the critic, parses + normalizes ids, then writes the normalized `forks/critic-<i>/rounds/r1-critic.md`. Only then does the orchestrator run `claude --resume <root> --fork-session -p "<pointer to r1-critic.md>" --output-format json`, which creates the fork *and* delivers the pointer to R1 in one shot. The fork's session ID is captured from that call's JSON output and written to `forks/critic-<i>/proposer-state.json` (with `agent: "claude"`, `fork_session_id: <uuid>`, etc.) *immediately on return* - there is no earlier moment to write it, since the ID does not exist before the call.
- **Codex-as-proposer R1 ordering**: also write `r1-critic.md` first; then run a fresh `codex exec` with the prompt referencing it. Capture the `thread_id` from the first `thread.started` event and append `{round: 2, thread_id: ...}` to `proposer-state.json`'s `round_thread_ids`. No fork to track.
- The proposer's R2 defense text is in the call's JSON `result` (claude) or final `agent_message` event (codex); persist to `r2-proposer.md` after the call returns. Update `attacks.jsonl` with one entry per attack-id status transition.
- For R3 onward: each round file is written before the orchestrator dispatches the corresponding pointer. In claude mode the dispatch is `claude --resume <fork_session_id> -p "<pointer>"`; the fork already exists, ordering is straightforward. In codex mode each round is a fresh `codex exec` re-supplied with prior round files; capture the new `thread_id` and append to `round_thread_ids`.
- `transcript.jsonl` and `attacks.jsonl` are append-only - never rewrite, never seek-back. A killed process leaves a valid (truncated) record.
- `summary.md` and `end.json` are written only at termination (clean or interrupted).
- `log.jsonl` is appended last, after `end.json` is durable. A run with `end.json` missing is an interrupted session; user can inspect `forks/<i>/rounds/` directly.
- **Root session is never modified by agon content.** No `claude --resume <root-id>` without `--fork-session`. The proposer-clone runs in the fork; agon turns never reach the root's transcript. Because agon is a separate process the user runs deliberately - nothing hooks into the live session and agon never injects a turn - the root JSONL is byte-identical after a run by construction. (The reason no in-editor trigger ships is precisely that every such mechanism would break this; see [36](36-probe-userpromptsubmit-manual-trigger.md).)

### Surfacing rule

- **Zero unresolved leaves at termination**: orchestrator is silent on stdout except for one line referencing the `log.jsonl` entry. No summary file is opened or surfaced. `summary.md` is still written for audit, but the user is not interrupted.
- **≥ 1 unresolved leaves**: orchestrator prints the path to `summary.md` plus the *headline contradicting signal* (see below) on stdout. Exit code 1.
- **Interrupted (Ctrl-C, cost-cap, malformed-output)**: same as ≥ 1 unresolved - surface what's there.

### Headline contradicting signal

Among unresolved leaves, the headline is the attack with the highest **contention score**:

```
contention(attack) = rounds_survived + (1 if critic re_attacked_after_defense else 0)
```

Tie-break by first appearance. This is deliberately a cheap rule with no LLM scoring - adding semantic-scoring would re-introduce the lazy-judge problem at the headline step. If contention turns out to be a poor proxy in practice, upgrade later by having both sides self-report confidence each round and weighting by `confidence_critic × confidence_proposer`.

### `.gitignore`

`.agon/` should not be committed. Orchestrator checks `.gitignore` on first run and prints a warning (not a hard error) if `.agon/` is missing from it. Doesn't auto-edit the file - that's the user's call.

## Configuration

Project-level `.agon.toml` overrides defaults:

```toml
max_turn = 6
side_count = 4
aspects = ["functional-logic", "security", "code-quality", "performance"]
cost_cap_tokens = 50000
allow_style_attacks = false  # default: code-quality critic attacks impact, not preference

# Models. Optional when main and side are different agent families.
# Required (and must differ) when main and side are the same family.
# main = "claude"
# side = "codex"
# main_model = "claude-sonnet-4-6"  # only used when main == side family
# side_model = "claude-opus-4-7"
```

## Termination conditions

The mediator terminates and writes `summary.md` + `end.json` when any of these fire:

1. **Steady state** - no new attacks two rounds running.
2. **Max turn** - hard cap reached.
3. **Cost cap** - token budget hit.
4. **Malformed output** - critic produces ill-formed attacks two rounds running (defensive: model is broken or prompt collapsed).
5. **User interrupt** - Ctrl-C; the orchestrator's signal handler still writes `end.json` before exiting.

The summary header names which condition fired, so the human knows whether to trust "0 unresolved" (steady state) or treat it as truncation (max-turn / cost-cap). Only steady-state termination with zero unresolved is "clean" - every other condition surfaces.

## Output format

`summary.md` structure:

```
# Agon review - terminated: steady-state | max-turn | cost-cap | malformed-output | interrupted

## Headline (most contested unresolved)
- [security/api.py:88] Input sanitization
  - Critic: framework auto-escape doesn't cover `LIKE` patterns.
  - Proposer: parameterized via SQLAlchemy.
  - **Stake**: input `'; DROP TABLE--%` against the search endpoint.
  - Contention: 3 rounds survived, critic re-attacked after defense.

## Other unresolved (2, sorted by contention)
- ...

## Resolved (12)
- [conceded] Off-by-one in pagination loop → fixed at src/api.py:42
- [rebutted] Race condition claim → critic withdrew after seeing lock at api.py:18

## Stats
critic-found-bug rate: 7/15 attacks led to a fix
agon cost: 38k tokens, 6 turns, 2 critics
session: .agon/sessions/2026-05-05T14-22-31-a3f9b1/
```

When unresolved count is zero, `summary.md` still has the Resolved + Stats sections (no Headline, no Unresolved) and is written but not surfaced. The user only sees one line on stdout pointing at `.agon/log.jsonl`.

The Headline section is the entire justification for the tool. If it's noise across many sessions, the tool fails - and the cross-session `log.jsonl` makes that measurable rather than vibes-based. The Stats block lets the user spot-check whether the critic is actually working: if `critic-found-bug rate` trends near 0, stop running it (or drop the weak aspect).

## Risks

- **Per-aspect lazy-critic risk** (was previously framed as "the binding one"; that overgeneralized the single-critic case). A lazy critic produces no value in its aspect. With multi-critic across distinct aspects, no single lazy critic collapses the whole tool - only its aspect goes uncovered. The binding question becomes **per-aspect**: for each aspect we ship as default, is the critic prompt + model competent enough to find real instances? Mitigation: measure critic-found-bug rate *per aspect* in upstream 07a; aspects below threshold get dropped from defaults rather than the tool being abandoned. The debate-theoretic intuition (one competent honest player suffices for soundness) is what makes this work.
- **Cost.** Multi-critic multi-turn debates 5–10x a coding session's token bill. Cost cap is always enforced (no uncapped mode); default it conservatively (50k tokens).
- **Trivial-diff noise.** Running agon on a near-empty diff (typo fix, single-line change) wastes tokens for no signal - the failure mode of "always-on review tools" if someone scripts agon into a tight loop. Mitigation is structural: `--changed-lines-min` (default 10) gates at the orchestrator entry point, so it returns in milliseconds with one status line for sub-threshold diffs instead of running. (agon is manual/deliberate by design, which already removes the auto-fire-on-every-edit failure mode entirely.)
- **Critic context starvation (and the inverse: rabbit-holes).** The aspect prompt asks the critic to focus on diff + task; in v0 this is enforced by prompt discipline rather than OS isolation (see Critic isolation), so a well-behaved critic produces context-starved attacks ("this function isn't called!" - yes it is, elsewhere) and a misbehaving one produces rabbit-hole attacks against unrelated code. Mitigations: critic prompt requires concrete reproduction (drops most rabbit-holes at parse time); the proposer is allowed to rebut with `file:line` references the critic is forbidden from re-attacking.
- **Stylistic-gripe drift (especially in `code-quality` aspect).** Critic drifts from real maintainability impact into formatting/naming preferences. `code-quality` is the most exposed aspect because the line between "real quality issue" and "preference" is fuzzier than for `security` or `performance`. Mitigation: critic prompt requires every attack to name a concrete behavior or maintainability impact; mediator drops style-shaped attacks at parse time (heuristic: attack contains "should be" + naming/formatting language without a concrete behavior claim).
- **Asymmetric truth.** Proposer has more context than critic; may over-defend when actually wrong. Mitigation: `--judge llm` mode triages unresolved leaves; default `none` just surfaces them and trusts the human.
- **Critic colludes with proposer (same family + same model).** Same agent family + same model = the model debating itself, with the same priors and same blind spots. The CLI rejects same-model configurations when families match (`--main-model` and `--side-model` must differ). Same-family/different-model is allowed but weaker than cross-family - use it only when cross-family isn't available (single-agent install). The default `--main claude --side codex` provides cross-family heterogeneity automatically.

## Out of scope

This section is the canonical list. The Versioning section above summarises the same partition; this section gives the reasons.

### Forever out (non-goals)

- **Skill or slash-command entry points.** Both wrap critic output in framing that distorts the proposer's response. The channel constraint says verbatim user-message via `claude --resume` only.
- **Plugin packaging** (Claude Code plugin manifest). A single CLI binary doesn't need it. Revisit only if multiple unrelated users adopt the tool.
- **Injecting into the root session.** Claude Code provides no way to add an assistant turn to an existing session, and every alternative (slash/skill body, `systemMessage`, `additionalContext`, a blocked `UserPromptSubmit` sentinel) mutates the root transcript - probe-verified in [36](36-probe-userpromptsubmit-manual-trigger.md). Wrap-up is stdout + disk only.
- **Auto-applying critic-suggested fixes.** Concession-fixes are written by the proposer-clone within its fork, not by the critic. The critic never edits.
- **Training a better critic.** Use whatever Codex (or whichever side) gives us; if it's bad, the tool fails per-aspect (and that's the right outcome - drop the aspect from defaults).
- **Adding extra tools to the critic.** The orchestrator does not register MCP servers, install custom skills, or otherwise expand the critic's tool surface. What the critic can do via its agent-default tools (file reads, bash, etc.) is constrained by aspect prompt + `codex --sandbox read-only`, not by OS isolation in v0 (see Critic isolation). Strict per-fork sandbox-dir isolation is a v1 enhancement.
- **Style-only attacks.** Critic prompt requires concrete behavior or maintainability impact. Mediator drops style-shaped attacks at parse time.

### Deferred to v1 (architecturally in scope, just not in v0)

- **Codex-as-proposer.** Architecture documented above; implementation is v1 - different round driver, fresh `codex exec` per round.
- **Parallel forks via per-fork worktrees** (`claude --worktree`). Frozen working-tree snapshots per fork eliminate the serial-outcome-leakage problem and enable parallel execution. Needs a concession-merge story when two critics' fixes conflict.
- **Streaming TUI / live in-session progress.** v0 is batch (no in-session UI; progress on stderr only); a streaming TUI would be v1 work.
- **Persistent agon state across user sessions.** Each v0 invocation is fresh against the current diff. `agon resume <session-id>` is v1.
- **Per-critic model configuration.** v0 shares one `--side-model` across all critics (aspect specialization carries the per-critic diversity). v1 may add per-critic model overrides.

## Relationship to upstream research

This tool is the productization of the debate architecture studied in [agents-byzantine-tolerance](https://github.com/changkun/agents-byzantine-tolerance). Specifically it bets on the architecture from [spec 07](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/07-adversarial-debate.md) and inherits its risks.

**This tool should not be built before upstream spec 07a returns positive results, measured per aspect.** What "positive" means with aspect-specialized critics:

1. Run upstream 07a per aspect (functional-logic, security, code-quality, performance) - aspect-specialized critics, not one generalist.
2. For each aspect, measure critic-found-bug rate on seeded bugs of that aspect.
3. **Default aspects = aspects where the rate is ≥ 60%.**
4. **Acceptable thresholds**: at least two aspects pass; otherwise the tool is hollow. If only one aspect passes, it's a single-aspect linter, not a agon tool.
5. If H1 also holds (debate beats voting at equal compute, on per-aspect tasks): the tool's core value proposition is confirmed.

The previous order-of-work conflated all critics into one threshold. With aspect specialization, the failure of one aspect doesn't kill the tool - it just narrows the default set. Upstream 07's H6 ("lazy critic collapses the architecture") is real but only at the per-aspect level.
