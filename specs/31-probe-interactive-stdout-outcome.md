# Spec 31 - Probe G7 outcome: interactive stdout rendering

> **Status: ✅ implemented** (G7 SKIP - non-blocking; recorded reason. README and spec 01 wording stays at the conservative "stdout best-effort" form. The release-blocker gate this spec closed was retracted in the 2026-05-08 simplification of [27](27-release.md); the probe is preserved in `scripts/probes/` for opt-in re-runs.)
> Implementation spec for `agon`. See [25-probes.md](25-probes.md) for the probe and [24-stop-hook.md](24-stop-hook.md), [01-overview.md](01-overview.md) for the wording it informs.

**Depends on:** [24](24-stop-hook.md), [25](25-probes.md).
**Consumed by:** [27](27-release.md), [README.md](../README.md).

## Scope

In: a recorded execution of `scripts/probes/interactive-stdout.sh` against an interactive `claude` PTY, and the disposition for README/spec wording.

Out: changes to the probe script or to the Stop-hook script ([24](24-stop-hook.md)).

## What we're proving

[01-overview.md](01-overview.md) and the README currently hedge with "stdout best-effort - may surface in interactive mode." This is the conservative wording chosen because the probe hadn't been run. The probe runs `claude` under `script` (or `expect`), fires a Stop hook that prints `[agon] hello` to stdout, and checks whether that line appears in the captured PTY output.

## Execution

1. `make build`.
2. Run `scripts/probes/interactive-stdout.sh`. The probe needs `script(1)` (BSD or util-linux variant) on `$PATH`.
3. If the probe fails to start a PTY for environment reasons (CI containers, missing `script`), record `SKIP` with the reason - this gate is **non-blocking**.

## Recording format

```
probe: interactive-stdout
host_os: darwin|linux
claude_version: <output of `claude --version`>
verdict: PASS | FAIL | SKIP
captured_line_present: yes|no|n/a
skip_reason: <free text, only if SKIP>
```

## Disposition

- **PASS:** README and [01-overview.md](01-overview.md) drop the "best-effort" / "may surface" qualifier; rephrase as "stdout surfaces in interactive mode." Files to edit: README install/usage section ([24-stop-hook.md](24-stop-hook.md) refers), [01-overview.md](01-overview.md) §"Surfacing rule" or wherever the qualifier appears, [23-summary-render.md](23-summary-render.md) if its `Stdout` notes hedge.
- **FAIL:** README and specs reword to "stdout does **not** surface in interactive mode; users must read `summary.md`." Stronger than the current wording, not weaker.
- **SKIP:** Wording stays as-is. Recorded as deliberately deferred; not a GA blocker, but the next release should retry.

## Acceptance criteria

- [x] Probe ran or was deliberately skipped with a reason. (SKIP, recorded reason.)
- [x] If PASS or FAIL, README and [01-overview.md](01-overview.md) wording updated accordingly. (N/A under SKIP; conservative wording stays.)
- [x] ~~[27-release.md](27-release.md) G7 cites the recording (including SKIP reason if applicable).~~ *(retracted: G7 no longer exists as a release blocker.)*
