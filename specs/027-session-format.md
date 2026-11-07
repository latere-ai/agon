---
title: Session format
status: complete
track: adversarial
updated: 2026-11-07
author: changkun
---

# Session format

A debate writes an auditable record to disk under a caller-provided `StateDir`.
The engine invents no default path and writes nothing into the reviewed repo; each
consumer owns its location. latere-cli writes under `$XDG_STATE_HOME/latere/reviews/<repo>/`
(fallback `~/.local/state/latere/reviews/<repo>/`). The layout is a stable, versioned contract: an embedder or a tool
can read it without linking the capability. Written by `internal/state`
(atomic) and `internal/summary` (the terminal render); a run lives
under `sessions/<id>/` where the id is `<YYYYMMDDTHHMMSSZ>-<rand6>`.

## Layout

```
<StateDir>/
  sessions/<id>/
    start.json                   # adversarial.start.v0: task, diff path, config, cwd, Go version
    diff.patch                   # the diff the run reviewed
    end.json                     # termination, stats, headline reference, exit code, summary path
    summary.md                   # contention-scored headline + open leaves + resolved set
    attacks.jsonl                # the attack ledger, one line per transition (adversarial.attack.v0)
    transcript.jsonl             # one line per round: fork, round, role, path, ms
    forks/critic-<n>/
      stats.json                 # adversarial.fork-stats.v0: topic, rounds, termination, per-role usage
      proposer-state.json        # adversarial.proposer-state.v0: agent, fork_session_id
      rounds/r<k>-critic.md      # each critic round (the attack document)
      rounds/r<k>-proposer.md    # each proposer round (defense + modified-files footer)
      attacks/<attack-id>.json   # a record body over 64 KiB, spilled from the ledger
```

## The key files

- **`start.json`** (`adversarial.start.v0`) - the run's inputs, written before
  the first agent call: the session id, the start time, the task context, the
  diff's patch path (`diff.patch`), the config (round cap, fork count, cost cap),
  the working directory and the Go version. The schema also has proposer and
  critic agent references, the diff's range, changed lines and files, the models
  and the engine version; the engine leaves those empty, because the
  `Proposer` and `Critic` interfaces do not report them.
- **`diff.patch`** - the unified diff the run reviewed, once per run. Every fork
  reviews the same diff.
- **`end.json`** - the terminal record: the end time, the termination reason,
  aggregate stats (attacks by status, rounds executed by each fork, token usage,
  the cost cap, wall seconds), headline reference, the exit-code decision and
  the `summary.md` path. The exit code is 0 for a run that ended steady-state
  with nothing unresolved, 130 for an interrupted run, and 1 otherwise. The
  `termination` object's `fork_index` and `round` are null. Its presence is how
  a reader tells a finished run from a running one: `Engine.Run` returns an
  error when it cannot write it.
- **`summary.md`** - the human-facing output: the highest-contention unresolved
  attack, the remaining open leaves, and the resolved set with dispositions.
- **`attacks.jsonl`** - the ledger (schema `adversarial.attack.v0`), one append per
  attack transition; see [Debate protocol](026-protocol.md). Bodies over 64 KiB spill
  to `forks/critic-<n>/attacks/` and the record keeps a path reference.
- **`forks/critic-<n>/`** - per-fork detail: `stats.json`
  (`adversarial.fork-stats.v0`: topic, rounds, termination, per-role token usage),
  the proposer fork-session id, and every round's markdown.

## Write discipline

State files are written atomically: a temp file is written and fsynced, then
renamed over the target and the parent directory is fsynced, so a crash never
leaves a half-written `start.json`/`end.json`/round file. The append-only
`attacks.jsonl` and `transcript.jsonl` are not fsynced per line, trading a small
durability window for throughput on the hot path. Every record carries a `schema`
field (`adversarial.<kind>.v0`) so the format can evolve without silently
misinterpreting old sessions.
