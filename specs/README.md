# Adversarial Review specs

Adversarial Review is an **adversarial-review engine and protocol**, the Go
module `latere.ai/x/agon`. After a coding agent produces a change, it forks the
session, runs one or more independent critics that attack the diff, lets the
proposer defend or concede, and surfaces only the disputes that survive. It is
imported as a library, not run as a standalone tool: the developer CLI is
`latere agents review` in [latere-cli](https://github.com/latere-ai/latere-cli), and
the same engine is embedded by wallfacer.

These specs are the current-state contracts for the engine and its protocol. They
describe what the code does today, not a build history. The roadmap at the bottom
describes what comes next.

## Contracts

| Spec | Status | What it covers |
|---|---|---|
| [Architecture](022-architecture.md) | complete | What the engine is, the fork/debate model, the component map, and its consumers |
| [Backends](023-backends.md) | complete | Proposer and critic backends: the claude and codex CLIs, and the model-call critic |
| [Engine API](024-engine-api.md) | complete | The public Go embedder contract (`agon`): `Engine`, `Proposer`, `Critic`, `Verifier`, and the result types |
| [Inputs](025-inputs.md) | complete | `input`: locating the Claude transcript and computing the working-tree diff |
| [Debate protocol](026-protocol.md) | complete | The wire contract: roles, rounds, the critic attack format, dispositions, the attack ledger, termination, and headline surfacing |
| [Session format](027-session-format.md) | complete | The on-disk `<StateDir>/sessions/<id>/` layout, artifacts, and schema versions |

The [`.archive/`](.archive/) directory holds 013 to 018, the specs that folded
the engine into the Topos runtime, where it lived until it became this module.
They are kept as a historical record and read `superseded`.

The numbers are the ones these specs carried in the topos repository. Commits
and other specs cite them, so they are kept. The gaps are topos's own: its
runtime specs hold 001 to 012 and 028 onward, and 019 to 021 were never used. A
new spec here takes the next number after 027.

## Conventions

Each spec opens with YAML frontmatter:

```yaml
---
title: <human-readable title>
status: drafted | complete | superseded
updated: YYYY-MM-DD
author: changkun
---
```

`complete` means the spec describes shipped behavior, `drafted` a proposal not
yet built, and `superseded` a record replaced by later work, which lives in
`.archive/`. Prose is plain and explanatory; do not use em dashes.

## Roadmap

Where Adversarial Review goes next, as an engine and a protocol. The contracts
above describe what ships today; this describes what is proposed and what is being
explored. Nothing here is a commitment; each item names what would move it forward.

### Engine and integration (near-term)

Hardening the embedder surface:

- **A codex backend package.** Promote the codex critic into a
  `codex` sibling of `claude` and `critic` once the API shape is settled.
- **API stabilization.** Move `agon` toward a stable, semver-committed
  surface. The protocol is more stable than the Go surface; the Go surface catches
  up.
- **Per-critic model configuration** and **parallel forks** via per-fork git
  worktrees (frozen snapshots) to eliminate cross-fork outcome leakage, with a
  concession-merge story.

### Protocol and research directions

Adversarial Review productizes adversarial-debate theory. The soundness case rests
on the 2023 *Doubly-Efficient Debate* result, which extends the 2018 PSPACE
intuition to stochastic systems and proves soundness under compute asymmetry: the
formal license for applying debate to LLMs at all. The
[agents-verification](https://github.com/changkun/agents-verification)
research line asks "is Adversarial Review sound under condition X?"; the capability
asks "given the answer, what does the engine become?" Each direction below is gated
on an empirical result.

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
  where soundness needs it. A schema change to the [protocol](026-protocol.md).

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
  most relevant to the capability's charter.

### Deployment futures

- **Hosted Verifier service.** The [`Verifier` interface](024-engine-api.md) is the
  seam for a hosted adversarial-review service beyond the local CLI track. Named,
  not built; the local track (`latere agents review`) is done.
