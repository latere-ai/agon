# agon specs

The implementation specs for `agon`. Each one is the authoritative
contract for a slice of the system; the code is meant to follow what
the spec says, not the other way around. Read in order; each builds on
the ones before it.

## Design

- [01 Overview](01-overview.md) - architecture, fork model, v0/v1 split, lifecycle invariants

## Foundations

- [02 Go module + layout](02-go-module.md)
- [03 CI, lint, release pipeline](03-ci-lint-release.md)
- [04 CLI flags](04-cli-flags.md)
- [05 `.agon.toml` config](05-config-file.md)
- [06 Preflight](06-preflight.md)

## Inputs

- [07 Claude transcript ingest](07-claude-transcript.md)
- [08 Diff capture](08-diff.md)

## On-disk layout

- [09 State directory](09-state-dir.md)
- [10 Run-level artifacts](10-run-artifacts.md)
- [11 Fork-level artifacts](11-fork-artifacts.md)
- [12 Attacks ledger](12-attacks-ledger.md)

## Critic protocol

- [13 Critic output format](13-critic-output-format.md)
- [14 Attack parser](14-attack-parser.md)
- [15 Aspect / topic prompts](15-aspect-prompts.md)

## Subprocess + agents

- [16 Subprocess infra](16-subprocess-infra.md)
- [17 Claude proposer](17-claude-proposer.md)
- [18 Critic drivers](18-critic-drivers.md)

## Orchestration

- [19 Round loop](19-round-loop.md)
- [20 Termination](20-termination.md)
- [21 Signals](21-signals.md)

## Output

- [22 Contention headline](22-contention-headline.md)
- [23 Summary render](23-summary-render.md)

## Triggering + verification

- [24 Stop hook](24-stop-hook.md) - ❌ RETIRED (hook removed; see [36](36-probe-userpromptsubmit-manual-trigger.md))
- [25 Probes](25-probes.md)
- [26 Tests](26-tests.md)
- [27 Release process and GA gates](27-release.md)

## Release-cut follow-ups (v0.0.1)

These specs were added during the v0.0.1 release cut. Each one closes
a numbered GA gate from spec 27 and writes its outcome into
[`release-notes-v0.0.1.md`](../release-notes-v0.0.1.md).

- [28 G4 probe: no-output Stop hook](28-probe-no-output-stop-hook-outcome.md) - ❌ RETIRED (hook removed)
- [29 G5 probe: signal latency](29-probe-signal-latency-outcome.md) - SIGINT to exit < 5s
- [30 G6 probe: trivial-diff fast path](30-probe-trivial-diff-perf-outcome.md) - hook returns < 200ms median
- [31 G7 probe: interactive stdout](31-probe-interactive-stdout-outcome.md) - non-blocking, manual
- [32 G13 real-e2e suite](32-real-e2e-suite.md) - e2e/real test behind real_e2e build tag
- [33 G15 install-hook smoke](33-install-hook-smoke.md) - ❌ RETIRED (install-hook removed)
- [34 G16 real-claude end-to-end](34-real-claude-end-to-end-smoke.md) - agon runs with real claude+codex
- [35 Release-notes channel](35-release-notes-channel.md) - decides where probe/gate outcomes live
- [36 Probe: UserPromptSubmit manual trigger](36-probe-userpromptsubmit-manual-trigger.md) - FAIL; manual trigger can't be byte-identical

## Public API + embedding

- [37 Public `pkg/adversarial` API](37-pkg-public-api.md) - importable engine, interfaces, result types
- [38 Read-only proposer option](38-read-only-proposer.md) - `WithProposerReadOnly` for embedders sharing the real tree
- [39 Topos-backed critic](39-topos-backed-critic.md) - `pkg/adversarial/topos` critic over `latere.ai/x/topos` (Lux/Cella); proposer stays on the CLI
- [40 Public `pkg/adversarial/input`](40-pkg-input-public.md) - promote `internal/input` (transcript + diff) so embedders can build a `VerifyInput` without reimplementing it

## Status

Specs 01-38 are marked ✅ implemented; specs 39 and 40 are `drafted`
(post-release proposals, not yet contracts). Spec status lines
individually record what's verified, what's deferred, and what was
changed during the release cut.

## Related research

**Nothing in v0 is driven by post-07 research.** v0 ships the
practical baseline: flat K-round agon, equal-protocol compute
between proposer and critic, default agent-CLI temperature, freeform
prose stakes, and a binary critic disposition (concede / rebut /
withdraw / push-back). The papers in this section are theoretical
context, not a roadmap.

`agon` productizes the architecture from
[agents-byzantine-tolerance](https://github.com/changkun/agents-byzantine-tolerance)
spec 07 ([Adversarial
Debate](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/07-adversarial-debate.md)).
Of the six follow-up specs (08–13) only one is load-bearing today:

- **The 2023 *Doubly-Efficient Debate* result is what licenses
  applying debate to LLMs at all** - it extends the 2018 PSPACE
  intuition from deterministic to stochastic systems and proves
  soundness under compute asymmetry. v0's "two LLMs cross-examining
  each other about a diff" is theoretically motivated, not
  hand-waved, *because of* this paper. Cited and operationalized
  across BFT specs 08–13.

The other specs describe protocol variants and extensions. None of
them currently mandate any change here; each one's empirical result
*could* license a specific change, conditional on the result going a
particular way:

| BFT spec | Empirical question | What a positive result *could* license here |
|---|---|---|
| [08 Compute-asymmetric](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/08-compute-asymmetric-agon.md) | Does soundness hold under {1×, 5×, 10×, 50×} compute asymmetry? | Per-role compute knobs (retries, per-role max-turn, per-role model). Minor change. |
| [09 Depth/recursion](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/09-debate-depth-recursion.md) | Flat-K or recursive sub-debate at matched compute? | "More rounds" (raise `--max-turn` defaults) vs. recursive sub-debates spawned per unresolved leaf. The latter is a v2-class architectural change. |
| [10 Stochastic-system](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/10-stochastic-system-soundness.md) | Does soundness survive the LLM temperature range? | If soundness drops above some T\*: a `--temperature` flag and refusal to run against agent configs that override it. Otherwise: nothing. |
| [11 PCP leaf format](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/11-pcp-leaf-spot-check.md) | Where does soundness collapse on the freeform-to-structured Pareto? | Tightening the critic's "Reproduction" field from freeform prose to structured tuples (`{file_path, line_range, expected_byte_pattern}`). Schema change for spec 13/14. |
| [12 Prover-Estimator](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/12-prover-estimator-obfuscation.md) | Is obfuscation a real LLM attack class? Does Prover-Estimator beat plain debate on it? | If yes to both: the binary disposition contract is unsound on pathological diffs and would be replaced by a scalar plausibility estimator. v2-class architectural change. |
| [13 DQC scaling](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/13-dqc-scaling.md) | Do judge-read tokens scale as O(log *n*) or O(*n*^a)? | If polynomial: an explicit length cap on `summary.md` body, with the contention headline (spec 22) as the only doubly-efficient channel. Otherwise: nothing. |

The general pattern: the BFT-repo line asks "is agon sound under
condition X?"; this repo asks "given the answer, what does the
production tool look like?" Today the answer for most rows is "we
don't know yet, and v0 doesn't depend on it" - which is fine,
because v0 is the practical baseline that the papers' positive
results are licensing in the first place.
