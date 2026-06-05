# Spec 34 - Real-claude end-to-end smoke

> **Status: ✅ implemented** (G16 PASS at 181s/fork on v0.0.1 rc binary. Auth was unblocked by `unset ANTHROPIC_API_KEY` so claude falls back to the OAuth token from `/login`; the shipping hook script already does this. The release-blocker gate this spec closed was retracted in the 2026-05-08 simplification of [27](27-release.md); the smoke remains as a developer sanity check.)
> Implementation spec for `agon`. See [01-overview.md](01-overview.md) §"v0 release blockers" for design intent.

**Depends on:** [17](17-claude-proposer.md), [18](18-critic-drivers.md), [19](19-round-loop.md).
**Consumed by:** [27](27-release.md).

## What we're proving

[27-release.md](27-release.md) G16: a real claude session followed by a 47-line diff, then `agon --session-id <id>` run against it, goes through to summary on disk and exits cleanly. Wall time ≤ 5 minutes per fork on default `--max-turn 6`.

This is distinct from [32](32-real-e2e-suite.md) (Path A) - that suite is automated and covers one fork end-to-end against the binary directly with mock agents. G16 covers the **integrated UX with real agents**: a human ends a real claude conversation, then runs agon against that session and the summary appears on disk. No automation replaces this; the mock suite bypasses real claude/codex.

## Execution

1. `make build`. Note `bin/agon`'s sha256.
2. In a fresh git repo with one trivial commit:
   ```
   mkdir -p ~/tmp/agon-g16 && cd $_
   git init && echo seed > seed.txt && git add . && \
     git -c user.email=t@e.com -c user.name=t commit -m init
   ```
3. Run claude interactively and ask it to make a 47-line edit (any plausible task; the diff size is what matters, not the content). Realistic prompt:
   > "Add a small Go HTTP handler in `server.go` with input validation and a couple of unit tests. Aim for around 50 lines."
4. End the session normally (Ctrl-D / `/quit`). Note its session id (the newest file under `~/.claude/projects/<encoded-cwd>/`).
5. From the same repo cwd, run agon against that session:
   ```
   ./bin/agon --session-id <id> --max-turn 6
   ```
6. Wait. Each fork has up to 5 minutes; default is 4 aspects → 4 forks → up to 20 minutes total in the worst case. Note the **per-fork** wall-time budget is what G16 asserts, not the total.
7. Inspect `.agon/sessions/<latest>/` and capture:
   - `summary.md` exists and is non-empty.
   - `end.json` exists; its `termination` field is one of `steady-state | max-turn | cost-cap`.
   - `forks/critic-*/` directories exist for each default aspect.
   - For each fork, `(end_time - start_time) ≤ 300s` from the round files' timestamps.

## Recording format

```
gate: real-claude-end-to-end
host_os: darwin|linux
claude_version: <`claude --version`>
codex_version: <`codex --version`>
session_dir: .agon/sessions/<id>
termination: steady-state | max-turn | cost-cap | malformed-output | interrupted
forks: [<aspect, wall_seconds>, ...]
max_per_fork_wall: <max of forks[*].wall_seconds>
verdict: PASS | FAIL    # PASS iff max_per_fork_wall ≤ 300s and a summary file exists
```

## Disposition

- **PASS:** [27-release.md](27-release.md) G16 cites the recording. The recorded session is *not* committed (it includes real diffs); only the recording block above is.
- **FAIL on wall time:** investigate which fork stalled. If a single fork runs >300s, the issue is either model latency (record + revisit) or a hung subprocess (root-cause via [21-signals.md](21-signals.md), fix, re-run).
- **FAIL on missing summary:** GA blocked; this is the load-bearing UX claim. Root-cause via [23-summary-render.md](23-summary-render.md) and re-run.
- **`termination: interrupted`:** treat as a flake; re-run once. If it repeats, GA blocked - a clean Stop-hook trigger should not interrupt.
- **SKIP (environment):** if `claude --print` returns HTTP 401 on the maintainer's host (ANTHROPIC_API_KEY missing or rejected, OAuth-only Claude Pro/Max account), this gate is deferred to a host with working API auth. SKIP must be re-cleared before tagging GA: a maintainer with valid auth runs the gate and edits the recording from SKIP to PASS or FAIL. SKIP alone does not unblock GA, but it does unblock other release-cut work in the meantime.

## Out of scope

- Repeating G16 across multiple model versions. One claude/codex pairing is enough for v0; the recording captures versions for audit.
- Cleanup of the throwaway repo or `.agon/sessions/` entry. Maintainer's call.

## Acceptance criteria

- [x] One real-claude session ran to completion; recording captured. (Run via `bin/agon --session-id <real-id>` against a fixture with a 46-line diff - the only trigger path.)
- [x] `verdict: PASS` and `max_per_fork_wall ≤ 300s`. Measured: 181 s.
- [x] ~~[27-release.md](27-release.md) G16 cites the recording.~~ *(retracted: G16 no longer exists as a release blocker.)*
- [x] Disposition updated to allow SKIP when `claude --print` is unauthenticated (HTTP 401), with the escape-hatch wording above. Now superseded by the PASS recording, but the escape hatch stays for future maintainers on hosts without working auth.
