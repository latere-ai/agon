---
title: Backends
status: complete
track: adversarial
updated: 2026-11-07
author: changkun
---

# Backends

A backend produces the proposer's or critic's text. The module ships three
public backends behind the [Engine API](024-engine-api.md) interfaces: a claude
proposer, a claude critic and a model-call critic. An embedder chooses the
runtime without changing the [protocol](026-protocol.md). The subprocess drivers
live in `internal/agent`, and the public wrappers in `claude` and `critic`.

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
  worktree and must argue and concede without editing it.

The session id is the id of a Claude Code session, so it must be ASCII letters,
digits, `_` and `-`, at most 128 characters, with no leading `-`
(`input.ValidateSessionID`). `FirstRound` checks the caller's id and `NextRound`
checks the fork id that claude returned, before any process starts; a bad id
returns an error wrapping `input.ErrInvalidSessionID`. The rule keeps an id from
being read as a CLI option.

The proposer gets a short pointer to the critic's round file on the command
line. The file is named by its absolute path, because the proposer's working
directory is the reviewed repository, not the session directory.

## Critic: the claude and codex CLIs

- `claude.NewCritic(opts...)` invokes `claude -p`: stateless, one call per
  round, usable as a critic for any task harness. The prompt, which carries the
  whole diff, goes on the child's standard input, so its size is not bound by
  the operating system's argument limits. The critic runs in the engine's `Cwd`
  with `--tools Read,Grep,Glob`, so it has no other built-in tool, and with
  `--strict-mcp-config` and no `--mcp-config`, so it loads no MCP server. Bash,
  Write, Edit, MultiEdit, NotebookEdit, WebFetch and WebSearch are denied as
  well. A diff can steer the critic, and these flags keep it from changing the
  tree or reaching the network, whatever the user's settings, a project or a
  plugin configure.
- Each subprocess call keeps at most `MaxOutputBytes` (64 MiB) of stdout; a
  process that writes more is stopped and the call fails with
  `ErrOutputTooLarge`. Only the last 64 KiB of stderr are kept, for the error
  message.
- `internal/agent`'s `CodexCritic` runs `codex exec --sandbox
  read-only --json`. Its read-only sandbox lets it open files the diff does not
  touch, which the prompt-only critics cannot. It is internal: no public package
  constructs it yet (see the roadmap in the [index](README.md)).

The subprocess drivers can stream `stream-json` events so tool and thinking
activity is visible live while a call runs. The toggle is internal
(`Verbose` plus `EventOut` on the driver structs); the public wrappers expose
no option for it yet.

## Critic: one model call

`critic.NewCriticFactory(cfg)` runs each critic round as one model call instead
of a local subprocess. This is for embedders that reach models through the Lux
gateway or a provider API rather than a local CLI: through the gateway, secrets
stay in the gateway and billing is centralized.

`Config` carries `Model`, a `critic.Generator` (the SDK's `*llm.Client` from
`latere.ai/x/sdk-go/llm` or a provider-direct `*llmdirect.Client` from
`latere.ai/x/pkg/llmdirect`; tests supply a scripted one), and `Name`, the
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

Only `critic` may import the model client (`latere.ai/x/sdk-go/llm`), and no
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
