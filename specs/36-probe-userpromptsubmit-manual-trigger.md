# Spec 36 - Probe outcome: no-output UserPromptSubmit manual trigger

> **Status: ✅ probed - FAIL.** A no-stdout `UserPromptSubmit` hook
> that exits 2 to "erase" a sentinel prompt does **not** leave the
> root JSONL byte-identical. The blocked sentinel is still recorded.
> Implementation spec for `agon`. See [01-overview.md](01-overview.md)
> §"Trigger via Stop hook" and [25-probes.md](25-probes.md) for design
> intent.

**Depends on:** [25](25-probes.md), [28](28-probe-no-output-stop-hook-outcome.md).
**Consumed by:** [24](24-stop-hook.md), [01-overview.md](01-overview.md).

## Question

Can a user-typed manual trigger (`/agon-attack`-style) run the
orchestrator out-of-band without polluting the root session, the way
the Stop hook does (spec 28, probe-verified byte-identical)?

The only user-typed mechanism that runs out-of-band is a
`UserPromptSubmit` hook keyed on a sentinel: it receives the same
stdin payload shape as the Stop hook (`session_id`,
`transcript_path`, `cwd`), `setsid`-detaches `agon`, writes nothing
to stdout (UserPromptSubmit stdout is itself injected into context,
unlike Stop-hook stdout), and exits 2 so the sentinel never becomes
a real turn the proposer sees. A custom slash command / skill is
ruled out independently: its body renders into the conversation as a
persistent turn (non-zero footprint by construction) and re-violates
the channel constraint already settled in [01-overview.md](01-overview.md).

So the load-bearing fact is: **does a no-stdout UserPromptSubmit
hook that exits 2 leave the root JSONL byte-identical?**

## Recording format

```
probe: no-output-userpromptsubmit
claude_version: <output of `claude --version`>
host_os: darwin|linux
exit_code: 0 | 1 | 3
verdict: PASS | FAIL | INCONCLUSIVE
mutation: <verbatim from probe stdout>
```

## Recording (2026-05-16)

```
probe: no-output-userpromptsubmit (exit 2, no stdout)
claude_version: 2.1.143 (Claude Code)
host_os: darwin
exit_code: 1
verdict: FAIL
hook fired: yes
before: sha 445ce57f… size 10047 lines 8
after:  sha 6110c00c… size 11168 lines 12
sentinel in JSONL lines: 2
hook_* attachments: (none)
```

The blocked sentinel added **4 lines / ~1.1 KB** to root JSONL:

1. `{"type":"queue-operation","operation":"enqueue",…,"content":"<sentinel>"}` - the sentinel prompt logged verbatim.
2. `{"type":"queue-operation",…}` - dequeue/clear bookkeeping.
3. `{"type":"system","subtype":"informational","content":"UserPromptSubmit operation blocked by hook:\n[<full hook command string>]"}` - records the block **and the hook command verbatim**.
4. `{"type":"last-prompt",…}` - last-prompt marker.

There were **no `hook_*` attachments and no agon/review content** -
the orchestrator's output never reaches root (it forks via
`--fork-session` and writes to disk, exactly as under the Stop hook).
"Erase the prompt" in the Claude Code docs means erased from the
model's view and the UI, **not** absent from the on-disk transcript.

## Disposition

A **byte-identical zero-pollution manual trigger inside Claude Code
is not achievable.** The Stop hook (spec 28) remains the only
probe-verified byte-identical path, and it is auto-only by nature.
Two honest options for an on-demand trigger:

- **CLI-only manual (byte-identical).** `agon --session-id <id>
  --max-turn N` (or a shell alias) run out-of-band in a terminal -
  already the documented manual path in [01-overview.md](01-overview.md)
  §"Manual / out-of-band". True byte-identical root; no in-editor
  ergonomics.
- **UserPromptSubmit sentinel (bounded, content-free footprint).**
  Detaches `agon`, exits 2. Root JSONL gains the 4 housekeeping
  lines above - the sentinel string + a "blocked by hook" system
  line - but **no review content and no proposer-visible template**.
  The channel constraint (the property [01-overview.md](01-overview.md)
  actually relies on for proposer behavior) is still satisfied; only
  the stricter byte-identical claim is lost.

The choice is a product call: byte-identical-but-terminal vs.
in-editor-but-4-housekeeping-lines.

**Decision (2026-05-16): CLI-only.** Byte-identical wins; no
in-editor trigger ships. The manual on-demand path is running the
`agon` CLI in a terminal (a shell alias for ergonomics) - already
the documented out-of-band path, now elevated to *the* on-demand
trigger in [01-overview.md](01-overview.md) §"Manual invocation".
No `UserPromptSubmit`/slash mechanism is implemented.

**Follow-on (2026-05-16, same day):** the question "is the auto Stop
hook even wanted?" was then answered **no** - the Stop hook was
removed *entirely*. agon has no auto path at all now; the CLI/alias
is the *only* trigger. Specs [24](24-stop-hook.md), [28](28-probe-no-output-stop-hook-outcome.md),
[33](33-install-hook-smoke.md) retired; [01](01-overview.md) /
[06](06-preflight.md) Stop-hook sections retracted.

## Acceptance criteria

- [x] `scripts/probes/no-output-userpromptsubmit.sh` written, mirrors
      [28](28-probe-no-output-stop-hook-outcome.md)'s harness, and ran
      once on the maintainer machine; full stdout captured above.
- [x] Mutation characterized line-by-line (queue-operation + system
      "blocked by hook" + last-prompt; no `hook_*`, no review content).
- [x] Product decision recorded: **CLI-only** (2026-05-16).
      [01-overview.md](01-overview.md) §"Rejected options" + §"Manual
      invocation" and `README.md` updated; no in-editor trigger ships.
