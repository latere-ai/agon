# agon specs

agon is a Go **adversarial-review engine and protocol**. After a coding agent
produces a change, agon forks the session, runs one or more independent critics
that attack the diff, lets the proposer defend or concede, and surfaces only the
disputes that survive. It ships as an importable library (`pkg/adversarial`),
not a standalone tool: the CLI is `latere agon` in
[latere-cli](https://github.com/latere-ai/latere-cli), and the same engine is
embedded by wallfacer and the agents platform.

These specs are the current-state contracts for the engine and its protocol.
They describe what the code does today, not a build history. The roadmap at the
bottom describes what comes next.

## Contracts

- [01 Architecture](01-architecture.md) - what agon is, the fork/debate model, the component map, and its consumers.
- [02 Debate protocol](02-protocol.md) - the wire contract: roles, rounds, the critic attack format, dispositions, the attack ledger, termination, and headline surfacing.
- [03 Engine API](03-engine-api.md) - the public Go embedder contract (`pkg/adversarial`): `Engine`, `Proposer`, `Critic`, `Verifier`, and the result types.
- [04 Backends](04-backends.md) - proposer and critic backends: the claude and codex CLIs, and the topos/Lux governed runtime.
- [05 Inputs](05-inputs.md) - `pkg/adversarial/input`: locating the Claude transcript and computing the working-tree diff.
- [06 Session format](06-session-format.md) - the on-disk `.agon/sessions/<id>/` layout, artifacts, and schema versions.
- [07 Landing site](07-site.md) - `agon.latere.ai`: the `agon-web` server, the embedded SPA, and its deploy.

## Conventions

Each spec opens with YAML frontmatter:

```yaml
---
title: <human-readable title>
status: current | proposed | exploratory
updated: YYYY-MM-DD
author: changkun
---
```

`current` means the spec describes shipped behavior; `proposed` and `exploratory`
appear in the roadmap below. Prose is plain and explanatory; do not use em dashes.

## Roadmap

Where agon goes next, as an engine and a protocol. The contracts above describe
what ships today; this describes what is proposed and what is being explored.
Nothing here is a commitment; each item names what would move it forward.

### Engine and integration (near-term)

Finishing the topos/Lux backend and hardening the embedder surface. Concrete items
carried over from the topos critic work (`pkg/adversarial/topos`):

- **Token usage from topos** (blocked on topos). topos's public `RunResult`
  exposes no usage, so topos critics report zero and fall outside the engine's
  cost-cap accounting. Needs a topos-side change to surface `loop.Result.TotalUsage`
  (or a usage event summable via an observer). This is the one gap that keeps
  topos critics "sound for correctness but not for cost".
- **Read-only file tools for topos critics** (blocked on topos). topos's only
  builtin is `bash`, so a read-only topos critic sees only the prompt-embedded
  diff and cannot open untouched files, unlike the codex critic's read-only
  sandbox. Needs topos-side read-only file tools.
- **Cella workspace wiring.** How an embedder's worktree reaches a Cella sandbox
  cwd (mount versus copy). Moot for the local sandbox and wallfacer's existing
  worktree; needed when a Cella embedder arrives.
- **Lineage surfacing.** Optionally carry topos lineage node IDs on `Summary` /
  `ForkOutcome` so an embedder can correlate critic forks with its own graph.
- **A codex backend package.** Promote the codex critic into a
  `pkg/adversarial/codex` sibling of `claude`/`topos` once the API shape is
  settled.
- **API stabilization.** Move `pkg/adversarial` toward a stable, semver-committed
  surface. The protocol is more stable than the Go surface; the Go surface catches
  up.
- **Per-critic model configuration** and **parallel forks** via per-fork git
  worktrees (frozen snapshots) to eliminate cross-fork outcome leakage, with a
  concession-merge story.

### Protocol and research directions

agon productizes adversarial-debate theory. The soundness case rests on the 2023
*Doubly-Efficient Debate* result, which extends the 2018 PSPACE intuition to
stochastic systems and proves soundness under compute asymmetry: the formal
license for applying debate to LLMs at all. The
[agents-byzantine-tolerance](https://github.com/changkun/agents-byzantine-tolerance)
research line asks "is agon sound under condition X?"; this repo asks "given the
answer, what does the engine become?" Each direction below is gated on an
empirical result.

Lighter changes, if the result goes a particular way:

- **Compute-asymmetric knobs.** If soundness holds across a compute-asymmetry
  range, expose per-role compute controls (retries, per-role round cap, per-role
  model).
- **Temperature guard.** If soundness drops above some temperature, add a
  temperature control and refuse to run against agent configs that override it.
- **Bounded summary length.** If judge-read tokens scale polynomially rather than
  logarithmically, cap the `summary.md` body and make the contention headline the
  only doubly-efficient channel.
- **Structured critic leaves (PCP).** Tighten the critic's reproduction field from
  freeform prose toward a structured tuple (`{file, line_range, expected_pattern}`)
  where soundness needs it. A schema change to the [protocol](02-protocol.md).

Heavier, architecture-class changes:

- **Recursive sub-debate.** Spawn a sub-debate per unresolved leaf, where the
  proposer's rebuttal becomes the new claim, instead of only running more flat
  rounds. Justified if recursion beats flat-K at matched compute.
- **Scalar disposition (Prover-Estimator).** If obfuscation is a real LLM attack
  class that plain debate loses to, the binary concede/rebut/withdraw contract is
  unsound on pathological diffs and would be replaced by a scalar plausibility
  estimator.

### Multi-agent extensions

Beyond the proposer-versus-critic asymmetry, two protocol shapes are candidates
(design input imported from wallfacer's oversight work):

- **N-agent debate.** A generalized multi-round deliberation (opening, rebuttal,
  closing, convergence detection) with configurable turn order and a lightweight
  convergence judge, distinct from the asymmetric review protocol.
- **Consensus and voting.** A voting protocol (single / cross-provider / unanimous
  / majority) framed in Byzantine-fault-tolerance terms (3f+1, with the
  correlated-failure caveat that same-vendor models share blind spots), with
  deterministic verifiers (linters, type-checkers) as votes, arbiter and human
  escalation, and per-dimension agreement maps. A designated red-teaming mode and
  empirical measurement of cross-provider independence are the adversarial pieces
  most relevant to agon's charter.

### Deployment futures

- **Hosted Verifier service.** The [`Verifier` interface](03-engine-api.md) is the
  seam for a hosted adversarial-review service at `agon.latere.ai` (beyond the
  landing site). Named, not built; the local track (`latere agon`) is done.
