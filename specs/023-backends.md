---
title: Backends
status: complete
track: adversarial
updated: 2026-09-28
author: changkun
---

# Backends

A backend produces the proposer's or critic's text. Adversarial Review ships
three, behind the [Engine API](024-engine-api.md) interfaces, so an embedder chooses
the runtime without changing the [protocol](026-protocol.md). The subprocess drivers
live in `internal/agent`; the public wrappers in
`claude`, `critic`.

## Proposer: the claude CLI

`claude.NewProposer(sessionID, cwd, opts...)` drives the
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

- `claude.NewCritic(opts...)` invokes `claude -p` - stateless, one
  call per round, usable as a critic for any task harness.
- `internal/agent`'s `CodexCritic` runs `codex exec --sandbox
  read-only --json`. Its read-only sandbox lets it open files the diff does not
  touch, which the prompt-only critics cannot.

The subprocess drivers can stream `stream-json` events so tool and thinking
activity is visible live while a call runs. The toggle is internal
(`Verbose` plus `EventOut` on the driver structs); the public wrappers expose
no option for it yet.

## Critic: one model call

`critic.NewCriticFactory(cfg)` runs each critic round as one model call instead
of a local subprocess. This is for embedders that reach models through the Lux
gateway or a provider API rather than a local CLI: through the gateway, secrets
stay in the gateway and billing is centralized.

`Config` carries `Model`, a `luxsdk.Caller` (the gateway's `luxsdk.Client` or a
provider-direct `luxsdk.Direct`; tests supply a scripted caller), and `Name`, the
model id each request names; a round's `CriticInput.Model` overrides it. Each
round sends `AssemblePrompt(in)` as one user turn with no tools, so the critic
reasons over the diff embedded in the prompt, under a fixed output cap. The text
blocks of the response, joined in order, are returned verbatim as
`CriticResult.Markdown`.

The response's usage fills `CriticResult.Usage` (input, output, cache read and
cache write tokens), `Tokens` (input plus output, as the subprocess critics count
it) and `Duration`, and `USD` when the gateway reports a cost. The critic
therefore counts against the engine's cost cap like the subprocess critics.

## The boundary

Only `critic` may import the model client (`latere.ai/x/pkg/luxsdk`), and no
package of the module may depend on an agent runtime (`latere.ai/x/topos`),
both enforced by `critic/boundary_test.go`. The engine core and every other
backend stay free of a model client, so the model-call critic is an opt-in
backend: an embedder that does not use it never pulls a model client into its
critic path.

## Model diversity

Cross-examination is stronger when proposer and critic have independent failure
modes. Diversity comes from pointing the critic backend at a different model or
provider (a non-Claude model through `critic.Config`, or the codex CLI), not
from a separate adversary package. The proposer stays on claude regardless.
