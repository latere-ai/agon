---
title: Backends
status: current
updated: 2026-07-05
author: changkun
---

# Backends

A backend produces the proposer's or critic's text. agon ships three, behind the
[03 Engine API](03-engine-api.md) interfaces, so an embedder chooses the runtime
without changing the [protocol](02-protocol.md). The subprocess drivers live in
`internal/agent`; the public wrappers in `pkg/adversarial/{claude,topos}`.

## Proposer: the claude CLI

`pkg/adversarial/claude.NewProposer(sessionID, cwd, opts...)` drives the
implementation agent through `claude --resume <sessionID> --fork-session`. This
is the only proposer, and deliberately so: forking the real Claude Code session
reconstitutes its full transcript, harness context, tool results, and working
tree for free. That fidelity is the point of the debate, and no other runtime can
reproduce it without re-implementing Claude Code session reconstitution.

Options:

- `WithProposerModel(model)` - override the claude model.
- `WithProposerDeadline(d)` - per-round call deadline (default 5m).
- `WithProposerReadOnly()` - disable the mutating tools (Write, Edit, MultiEdit,
  NotebookEdit, Bash). Use it when the proposer shares the embedder's real
  worktree and must argue and concede without editing it (wallfacer's harness).

## Critic: the claude and codex CLIs

- `pkg/adversarial/claude.NewCritic(opts...)` invokes `claude -p` - stateless, one
  call per round, usable as a critic for any task harness.
- `internal/agent`'s `CodexCritic` runs `codex exec --sandbox read-only --json`.
  Its read-only sandbox lets it open files the diff does not touch, which the
  prompt-only critics cannot.

The subprocess drivers stream `stream-json` events so an embedder can surface
tool and thinking activity live while a call runs.

## Critic: the topos/Lux runtime

`pkg/adversarial/topos.NewCriticFactory(cfg)` runs each critic as a single
read-only agent in the topos runtime instead of a local subprocess. This is for
embedders already inside the topos world (wallfacer, the agents platform): model
routing goes through Lux or Direct, execution runs in a topos sandbox (local or
Cella), and every fork is a distinct lineage node. Secrets stay in the gateway
and billing is centralized.

`Config` carries `Model` (`xtopos.ModelOptions`: Lux, Direct, or Fake), `Sandbox`
(nil uses the local sandbox), `Brain` (a scripted model for tests, overriding
`Model`), and `Tools`. The read-only posture is the default: with `Tools` nil the
agent gets no grant, and since topos's only builtin is `bash`, withholding it
leaves the critic no way to execute or mutate. The critic reasons over the diff
embedded in the assembled prompt. Each round runs one `Pinned` single-agent region
over `AssemblePrompt(in)` and returns the agent's final text as
`CriticResult.Markdown`.

topos critics report zero token usage today, because topos's public `RunResult`
exposes none; that excludes them from cost-cap accounting until topos surfaces
usage (see the [roadmap](README.md#roadmap)).

## The boundary

Only `pkg/adversarial/topos` may import `latere.ai/x/topos`, enforced by
`pkg/adversarial/topos/boundary_test.go`. The engine core and every other package
stay topos-free, so topos is an opt-in backend and `latere.ai/x/topos` is the
module's single direct dependency. An embedder that does not use the topos backend
does not link topos.

## Model diversity

Cross-examination is stronger when proposer and critic have independent failure
modes. Diversity comes from pointing the critic backend at a different model or
provider (a non-Claude model via `ModelOptions`, or the codex CLI), not from a
separate adversary package. The proposer stays on claude regardless.
