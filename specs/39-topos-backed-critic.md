---
title: Topos-backed critic
status: drafted
depends_on:
  - specs/37-pkg-public-api.md
  - specs/38-read-only-proposer.md
affects:
  - pkg/adversarial/topos/critic.go
  - pkg/adversarial/topos/critic_test.go
  - pkg/adversarial/topos/boundary_test.go
  - go.mod
  - go.sum
effort: medium
created: 2026-06-28
updated: 2026-06-28
author: changkun
dispatched_task_id: null
---

# Topos-backed critic

## Problem

agon's critics today shell out to local CLIs: `codex exec --sandbox read-only`
and `claude -p` (`internal/agent/critic.go`). That ties critic execution to the
host machine: it must have those binaries installed and authenticated, the model
calls bill against the user's own account, and there is no central sandbox or
governance over what a critic can touch.

Meanwhile the sibling latere products have standardized their agent runtime on
`latere.ai/x/topos`. wallfacer compiles its flows to a `topos.Region` and runs
them through `topos.NewRunner` (only `internal/agentgraph` imports topos, guarded
by a boundary test). The agents platform depends on topos's lower layers
(`runtime/loop`, `models`, `sandbox`, `harness/*`) to run agents in Cella
sandboxes with model routing through Lux, where secrets stay in the gateway and
billing is centralized.

When one of those products embeds agon as a `Verifier` (spec 37), the critic
forks still bypass that governed runtime and reach for local binaries that may
not exist in a server or pod context. Spec 37 deferred a non-Claude critic
backend "once the public API shape is validated by at least one real importer
(wallfacer)." That validation has happened, so the backend can land.

## Goal

Provide an `adversarial.Critic` implementation backed by topos, so an embedder
running inside the topos world (wallfacer, agents) can run critic forks through
the same governed runtime: model routing via Lux or Direct, a topos sandbox
(local or Cella), read-only tools, and a lineage record, instead of local CLI
subprocesses.

agon's standalone CLI behavior is unchanged. The topos critic is opt-in, supplied
by the embedder through the existing `CriticFactory` seam. The `cmd/agon` binary
does not import the new package and therefore does not link topos.

## Why critic-only (the proposer stays on the CLI)

The proposer is deliberately out of scope. agon's proposer depends on
`claude --resume <id> --fork-session` (`pkg/adversarial/claude`,
`internal/agent/claude_proposer.go`), which branches the developer's real Claude
Code session: its full transcript, the harness system context, the actual
tool-call results, and the working tree it ran in. The `claude` CLI is the source
of truth for that session format and reconstitutes all of it for free.

topos cannot express this today, and could only approximate it. Its
`runtime/loop` does carry an `InitialTranscript []models.Message` seed (added for
crash-resume, `runtime/loop/loop.go`) and its `models` layer accepts a full
`Messages` history, but:

1. the public `Runner.Run(ctx, region, task string)` does not expose the
   transcript seam at all, and
2. even with a new `RunResume`-style primitive, seeding it would mean converting
   Claude Code's own session into topos message format and re-creating the
   harness context and tree state. That is a re-implementation of Claude Code
   session reconstitution, not a thin adapter.

Critics have no such dependency. A critic is a stateless, one-shot, read-only
pass over `(diff, prior attacks)` that emits an attack block. That maps cleanly
onto a single read-only topos agent. So this spec moves the critic and leaves the
proposer on the CLI. Extending topos's public API for fork-session is a
topos-side change, tracked separately if it is ever pursued.

## Design

### Placement and boundary

New package `pkg/adversarial/topos`, a sibling of `pkg/adversarial/claude`, in
agon's existing module. Only this package imports `latere.ai/x/topos`; the
`pkg/adversarial` core stays topos-free, mirroring wallfacer's single-seam
convention. agon's `go.mod` gains a `require latere.ai/x/topos`, but because
`cmd/agon` never imports `pkg/adversarial/topos`, the compiler tree-shakes topos
out of the shipped binary. A boundary test asserts this seam (see Acceptance
Criteria).

### API

```go
package topos

import (
    "context"

    "latere.ai/x/agon/pkg/adversarial"
    xtopos "latere.ai/x/topos"
    "latere.ai/x/topos/models"
    "latere.ai/x/topos/sandbox"
)

// Config wires a topos-backed critic to a model and sandbox.
type Config struct {
    Model   xtopos.ModelOptions // Lux, Direct, or Fake
    Sandbox sandbox.Provider    // nil uses topos's local sandbox
    Brain   models.Model        // optional: overrides Model; used by tests to
                                 // inject a scripted model (topos Options.Brain)
    Tools   []string            // tool grant; nil means no tools (read-only by
                                 // exclusion, see "Read-only posture" below)
}

// NewCriticFactory returns an adversarial.CriticFactory whose critics run one
// read-only topos agent per round. forkIdx is threaded into the topos SessionID
// and AgentSpec name so each fork is a distinct lineage node.
func NewCriticFactory(cfg Config) adversarial.CriticFactory
```

### Per-round behavior

Each `Critic.Round(ctx, in adversarial.CriticInput)` call:

1. Assembles the agent prompt with the existing public
   `adversarial.AssemblePrompt(in)` (it lives in the `adversarial` package, not a
   separate `critic` package). The engine has already built the aspect +
   round-contract system text into `in.SystemPrompt`; `AssemblePrompt` combines
   that with `TaskContext`, `DiffPatch`, and `PriorRoundFiles` into a single
   string. So the protocol and output format are byte-for-byte the same contract
   the CLI critics follow (spec 13, 14, 15). The backend changes; the protocol
   does not.
2. Builds a single `xtopos.AgentSpec`. The whole assembled string is the agent's
   task (see step 3), so `SystemPrompt` is left empty (setting it to
   `in.SystemPrompt` would double the system text, which is already at the front
   of the assembled string). `Tools` is the read-only grant (see "Read-only
   posture").
3. Runs it through the public Runner:
   `xtopos.NewRunner(Options{SessionID, Model, Sandbox, Brain}).Run(ctx, Region{Autonomy: Pinned, Entry: spec}, adversarial.AssemblePrompt(in))`.
   A single-agent `Pinned` region with no peers runs the entry agent once and
   does no delegation; critics do not fan out internally.
4. Maps `RunResult.Final` to `CriticResult.Markdown`. Token usage is NOT mapped:
   topos's public `RunResult` is `{Lineage, Final}` and exposes no usage
   (`runtime/loop` accumulates `TotalUsage` internally but drops it at the public
   boundary), so `CriticResult.Usage`/`Tokens`/`USD` stay zero until topos
   surfaces usage (OQ-3).

The critic returns the model's text verbatim as `CriticResult.Markdown`; it does
NOT parse attacks. The engine parses the markdown later via the same parser used
for the claude and codex critics (`internal/critic.ParseAttacks`, spec 14). The
topos critic is a backend swap, verified by feeding a canned attack block through
a scripted `Brain` and confirming the returned markdown parses into the same
`Record` set as the other backends.

### Read-only posture

topos v0.0.5 registers exactly one builtin tool, `bash` (`harness/tools/bash.go`),
and no `read`/`grep`/`glob` tools. So the read-only equivalent of `CodexCritic`'s
`--sandbox read-only` is to grant the critic agent NO tools (an empty/non-`bash`
`AgentSpec.Tools`): with no `bash` grant the runtime gives the agent no way to
execute or mutate anything. The critic reasons over the diff, which is already in
the assembled prompt. If a future topos adds read-only file tools, `Config.Tools`
lets an embedder opt into them; the default stays "no tools" (OQ-4).

### Working tree

The embedder supplies the tree through the sandbox and cwd. For wallfacer that is
the throwaway worktree it already runs critics in (spec 38 non-goal note). The
local sandbox runs in `in.Cwd`; a Cella sandbox mounts the embedder's workspace.
Exact Cella workspace wiring is deferred (OQ-1).

## Non-Goals

- Moving the proposer to topos. See "Why critic-only".
- Adding topos to `cmd/agon` or to the shipped binary's link closure. The CLI
  keeps shelling to claude and codex.
- Changing the debate protocol, aspect prompts, attack format, or termination
  logic (spec 13 through 15, spec 20). Backend swap only.
- A separate codex-style topos adversary package. Model diversity comes from
  pointing `ModelOptions` (Direct or Lux) at a non-Claude provider, not from a
  new package.
- Extending topos's public API to support proposer fork-session.

## Open Questions

- OQ-1: Cella workspace wiring. How the embedder's worktree files reach a Cella
  sandbox cwd (mount versus copy). Moot for the local sandbox and for wallfacer's
  existing worktree path; defer until a Cella embedder needs it.
- OQ-2: Lineage surfacing. Whether agon's `Summary` or `ForkOutcome` should carry
  the topos lineage node IDs so an embedder can correlate critic forks with its
  own graph (wallfacer renders lineage in GraphCanvas). Likely a follow-up; out
  of scope here.
- OQ-3: Token usage. topos's public `RunResult` exposes no usage, so a topos
  critic reports zero tokens and zero USD. That excludes topos critics from the
  engine's cost-cap accounting (spec 20). Fixing it needs a topos-side change
  (surface `loop.Result.TotalUsage` on the public `RunResult`, or emit a usage
  event the embedder can sum via `Options.Observer`). Until then the topos critic
  is sound for correctness but not for cost-cap; tracked for Phase 2 / production.
- OQ-4: Read-only file tools. topos has only a `bash` builtin, so the read-only
  critic runs with no tools and sees only the prompt-embedded diff (it cannot
  open files the diff does not touch, unlike the codex critic's read-only
  sandbox). If that breadth matters, topos needs read-only file tools; defer
  until a real critic needs to read beyond the diff.

## Phasing / Acceptance Criteria

Phase 1 (package plus tests, no embedder wiring):

- `pkg/adversarial/topos.NewCriticFactory(cfg)` returns a `CriticFactory` whose
  critics implement `adversarial.Critic`. _(tested with a scripted `Brain` for
  determinism)_
- A critic round drives one topos agent and returns `CriticResult.Markdown` that
  `internal/critic.ParseAttacks` parses into the same `Record` shape as the
  claude and codex critics on identical input. _(tested with a scripted `Brain`
  returning a canned attack block)_
- The topos critic is granted no `bash` tool (no tools by default), so the
  runtime gives it no way to execute or mutate the tree. _(tested via the default
  tool grant)_
- A boundary test asserts that only `pkg/adversarial/topos` imports
  `latere.ai/x/topos` (root or subpackage); `pkg/adversarial` and `cmd/agon` do
  not. _(tested, adapts wallfacer's `boundary_test.go`)_
- `cmd/agon` build and the existing probe and unit suites are unchanged, and the
  binary does not link topos. _(existing tests pass; verified via `go list -deps
  ./cmd/agon | grep topos` returning empty)_

Phase 2 (validate from a real importer):

- wallfacer's verifier path constructs the topos critic factory (`ModelFake` in
  tests, Lux in prod) and hands it to agon's `Engine` or `Verifier`. This is the
  external-importer smoke test, mirroring spec 37 Phase 2.
