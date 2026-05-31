# agon

Adversarial review for Claude Code coding sessions.

After Claude finishes a task, `agon` forks the session for one or
more critic agents (Codex by default), runs a multi-round
cross-examination per critic, applies any concessions the proposer
makes, and surfaces only the unresolved disputes for human attention.
Each critic picks its own attack topic in round 1 (security, perf,
internal-consistency, evidence-gap, ...); later critics are told
which topics are taken and pick something else. No agon content
ever lands in the root Claude session - agon happens in branched
forks off the root.

Design and per-component contracts live under [specs/](specs/) -
start at [specs/README.md](specs/README.md) for the index.
Release-cut evidence (probe outcomes, smoke recordings) is committed
to [release-notes-v0.0.1.md](release-notes-v0.0.1.md).

## Installation

```sh
curl -fsSL https://raw.githubusercontent.com/latere-ai/agon/main/install.sh | sh
```

The script detects your OS / arch, fetches the latest release tarball
from
[github.com/latere-ai/agon/releases](https://github.com/latere-ai/agon/releases),
verifies the sha256 checksum, and installs `agon` to `/usr/local/bin`
(via `sudo` if needed). Knobs:

```sh
AGON_VERSION=v0.0.1-rc2  # pin a specific tag; default: latest
AGON_PREFIX=$HOME/.local # binary lands at $AGON_PREFIX/bin
```

From source (requires Go 1.26+):

```sh
go install latere.ai/x/agon/cmd/agon@latest
```

## Example usage

agon is a deliberate CLI: run it yourself after a coding session (see
"Run it on demand" below). You see one stdout line; the summary lives
on disk:

```text
$ # ...claude does its thing...
[agon] 2 unresolved; see /repo/.agon/sessions/20260506T140905Z-q3a9f1/summary.md

$ cat .agon/sessions/*/summary.md
# Agon review - terminated: steady-state

## Headline (most contested unresolved)
- [security/api.go:88] SQL injection via unparameterized LIKE
  - Critic: framework auto-escape doesn't cover LIKE patterns
  - Proposer: parameterized via SQLAlchemy
  - **Stake**: GET /search?q=%' OR 1=1--
  - Contention: 3 (re-attacked: true)

## Resolved (5)
- [conceded] Off-by-one in pagination → fixed at api.go:42
...

## Stats
critic-found-bug rate: 5/8 attacks led to a fix
agon cost: 38k tokens, 6 rounds, 4 critics
```

Trivial diffs (under `--changed-lines-min`, default 10) short-circuit
in milliseconds. No session folder, just one `kind:"skipped"` line in
`.agon/log.jsonl`.

### Run it on demand

There is no in-editor manual trigger: a slash command, skill, or
`UserPromptSubmit` sentinel all mutate the root transcript (probed -
see `specs/36-probe-userpromptsubmit-manual-trigger.md`). The
byte-identical way to trigger on demand is to run agon yourself in a
terminal - it only touches the live session via `--fork-session`, so
your Claude Code transcript is untouched, exactly as under the Stop
hook. This is also the path for CI gating, scripted batch runs, and
reviewing a saved session:

```sh
agon \
  --session-id <root-claude-session-id> \
  --side-count 4 \
  --max-turn 6
```

A shell alias keeps it one keystroke away (agon resolves the latest
session for the cwd when `--session-id` is omitted):

```sh
alias agon-attack='agon --side-count 4 --max-turn 6'
```

Each of the four critics picks its own topic in R1; the orchestrator
passes prior critics' topics to each later critic as anti-duplication
signal. `agon --help` lists every flag. Exit codes: 0 clean,
1 unresolved leaves, 130 interrupted, 100s pre-flight failure.

## Design architecture

```
                  user's claude session (the "root")
                            │
                            │  --fork-session (per critic)
                ┌───────────┼───────────┐
                ▼           ▼           ▼
          fork-1            fork-2 …    fork-N
          ┌──────┐          ┌──────┐    ┌──────┐
          │ pro- │ <══════> │ ...  │    │ ...  │
          │poser │  rounds  │      │    │      │
          │clone │          │      │    │      │
          └──┬───┘          └──────┘    └──────┘
             │
             ▼ writes round files to disk
   .agon/sessions/<id>/forks/critic-i/rounds/r{1,2,3,…}-{critic,proposer}.md
                                            │
                                            ▼
                                    summary.md  (contention-scored headline + leaves)
```

Five load-bearing pieces (full design in
[spec 01](specs/01-overview.md)):

- **Forked agon, no agon content in root.** Each critic gets its
  own claude fork via `--fork-session`. agon runs as a separate
  process and only ever touches the live session through that fork,
  writing results to disk — the user's root transcript never sees an
  agon turn.
- **Verbatim channel.** Critic output reaches the proposer-clone as a
  plain user turn pointing at a file: `Some comments at @<path>.
  Please resolve or respond.` No skill, slash-command, or
  plugin-template wrapping that would distort the proposer's normal
  defense behavior.
- **Self-declared topics.** Each critic chooses its own attack topic
  in R1 (security, perf, internal-consistency, evidence-gap, ...) and
  later critics are told which topics are already claimed so they
  pick something else. No fixed catalog. The debate-theoretic
  property - one competent honest player suffices for soundness -
  means a lazy critic on one topic doesn't break the others.
- **Persisted ledger.** Every attack carries a stable id (`c<critic>-<seq>`),
  every transition is appended to `attacks.jsonl`. Headlines are
  picked by a pure contention score (`rounds_survived + (1 if
  re-attacked)`) - no LLM judging at this layer.
- **Best-effort critic isolation.** v0 enforces "artifact + task
  only" by critic system prompt and `codex --sandbox read-only`, not
  OS isolation; strict per-fork sandbox dirs are v1.

## Related work

- [agents-byzantine-tolerance](https://github.com/changkun/agents-byzantine-tolerance)
  - research repo studying multi-agent Byzantine fault tolerance,
  including
  [spec 07 / Adversarial Debate](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/07-adversarial-debate.md),
  the architecture this tool productizes. Specs 08–13 explore
  protocol variants (compute asymmetry, recursive sub-debate,
  stochastic systems, PCP-style leaves, Prover-Estimator,
  DQC scaling). None drive v0; each one's empirical result *could*
  license a specific change here if it goes a particular way - see
  [specs/README.md §Related research](specs/README.md#related-research)
  for the conditional mapping.
- Irving, Christiano & Amodei,
  [*AI Safety via Debate*](https://arxiv.org/abs/1805.00899) (2018) -
  one agent proposes, another finds flaws, a judge inspects only the
  single disputed claim that decides the debate. The
  complexity-theoretic intuition (debate ≈ PSPACE under optimal
  play) motivates the architecture.
- Brown-Cohen, Irving & Piliouras,
  [*Scalable AI Safety via Doubly-Efficient Debate*](https://arxiv.org/abs/2311.14125)
  (2023) - extends 2018 to stochastic systems and proves soundness
  under unbounded compute asymmetry between the players. The formal
  license for applying debate to LLMs at all.
