---
title: Public pkg/adversarial API
status: complete
depends_on: []
affects:
  - pkg/adversarial/adversarial.go
  - pkg/adversarial/state/state.go
  - pkg/adversarial/ledger/ledger.go
  - pkg/adversarial/critic/critic.go
  - cmd/agon/main.go
  - go.mod
effort: medium
created: 2026-06-26
updated: 2026-06-26
author: changkun
dispatched_task_id: null
---

# Public pkg/adversarial API

## Problem

All of agon's load-bearing logic lives under `internal/` — `round`, `agent`, `ledger`,
`state`, `input`, `critic`. Nothing is importable by external Go modules. External
tools that want to embed agon's adversarial debate protocol must either shell out to
the `agon` binary (losing structured results) or vendor a private copy.

The only reason these packages are `internal/` is the default Go convention for new
code, not an intentional API-boundary decision. The packages are already cleanly
parameterized: `round.Engine` accepts `Proposer` and `CriticFactory` as interfaces
with no CLI dependency in sight.

## Goal

Extract the core engine cluster into a public `pkg/adversarial` API surface so that
external Go modules can:

1. Import and run the debate engine without shelling out to `agon`.
2. Supply their own `Proposer` and `Critic` implementations (multi-harness extension
   point).
3. Read structured debate results (`ledger.Record`, `Summary`) without parsing text.

The standalone `cmd/agon` binary rewires to import from `pkg/` instead of `internal/`
and its observable behavior must be bit-for-bit identical to today. No user-facing
change to `agon` itself.

## Design

### Packages to promote

**`pkg/adversarial`** — engine, core interfaces, result types

```go
package adversarial

import (
    "context"
    "io"
    "time"
)

// Proposer drives the implementation agent across forks.
// The zero-value pointer string is used for the first fork; subsequent
// rounds receive the fork-session ID returned by FirstRound.
type Proposer interface {
    FirstRound(ctx context.Context, pointer string) (*ProposerResult, error)
    NextRound(ctx context.Context, forkID, pointer string) (*ProposerResult, error)
}

// ProposerResult carries what a proposer returned in one round.
type ProposerResult struct {
    ForkID  string
    Text    string
    Usage   TokenUsage
}

// Critic drives one critic agent for one fork.
type Critic interface {
    Round(ctx context.Context, in CriticInput) (*CriticResult, error)
}

// CriticFactory creates a Critic for the given fork index.
type CriticFactory func(forkIdx int) Critic

// CriticInput is the per-round input for a Critic.
type CriticInput struct {
    Aspect          string
    CriticIndex     int
    Round           int
    SystemPrompt    string
    TaskContext     string
    DiffPatch       string
    PriorRoundFiles []string
    Cwd             string
    Deadline        time.Duration
    Model           string
}

// CriticResult is what a Critic produced in one round.
type CriticResult struct {
    Text  string
    Usage TokenUsage
}

// TokenUsage is a token-count summary.
type TokenUsage struct {
    Input       int
    Output      int
    CacheCreate int
    CacheRead   int
}

// Engine orchestrates the multi-fork debate. Callers must set at least
// Sess, Cwd, Proposer, and NewCritic before calling Run.
type Engine struct { ... } // promoted from internal/round.Engine

// Run executes all forks and returns a Summary.
func (e *Engine) Run(ctx context.Context) (*Summary, error)

// Summary is the post-run result.
type Summary struct {
    Termination string // "cost_cap" | "rounds" | "all_resolved" | "context"
    Forks       []ForkOutcome
    TokensUsed  int
    Usage       TokenUsage
    USD         float64
    WallSeconds int
    Headline    string // markdown of the highest-contention unresolved attack
    Unresolved  int
}

// ForkOutcome describes the result of one fork.
type ForkOutcome struct {
    ForkID     string
    Rounds     int
    Unresolved int
}

// Verifier is the top-level interface for adversarial post-run verification.
// Callers supply a VerifyInput and get back a VerifyResult without needing to
// assemble an Engine directly. This is the integration seam for tools (e.g.
// wallfacer) that want to embed adversarial verification as a plugin step.
// The no-op implementation returns (nil, nil) immediately (skip path).
type Verifier interface {
    Verify(ctx context.Context, in VerifyInput) (*VerifyResult, error)
}

// VerifyInput parameterizes one verification run.
type VerifyInput struct {
    TaskPrompt    string        // task description / intent
    Criteria      string        // acceptance criteria; empty means no bar
    SessionID     string        // implementation-agent session ID (proposer path)
    DiffPatch     string        // pre-computed git diff patch
    Cwd           string        // working directory
    StateDir      string        // where .agon/sessions/ is written
    ForkCount     int           // number of critic forks (caller sets default)
    MaxRounds     int           // debate rounds per fork (caller sets default)
    CostCapTokens int           // soft token budget
}

// VerifyResult is what a Verify call produced.
type VerifyResult struct {
    Unresolved int    // open attacks not conceded or rebutted
    Headline   string // markdown summary of highest-contention unresolved attack
    SessionDir string // path to .agon/sessions/<id>/
    USD        float64
}
```

**`pkg/adversarial/state`** — session persistence (re-export or thin wrapper over
`internal/state`)

Exports: `Session`, `NewSession`, and the load/save helpers. Path conventions stay
identical.

**`pkg/adversarial/ledger`** — attack records (re-export or thin wrapper over
`internal/ledger`)

Exports: `Record`, `Status` (open / conceded / rebutted / withdrawn / unresolved),
`Aggregate`.

**`pkg/adversarial/critic`** — aspect types and prompt assembly

Exports: `Aspect`, `AssemblePrompt(in CriticInput, aspects []Aspect) string`,
`ParseAttacks(text string) []Record`. These are the seams that let external Critic
implementations produce valid debate output without duplicating the protocol.

**`pkg/adversarial/claude`** — Claude proposer (the only harness that supports
fork-session cloning)

```go
package claude

// NewProposer returns a Proposer that drives claude --resume/--fork-session.
// rootSessionID is the claude session that ran the implementation; cwd is
// the working directory for subsequent claude invocations.
func NewProposer(rootSessionID, cwd string) adversarial.Proposer

// NewCritic returns a Critic backed by claude -p (one-shot, stateless).
func NewCritic(model string) adversarial.Critic
```

Note: `pkg/adversarial/codex` is out of scope for this spec. The codex critic
in `internal/agent/` can be promoted in a follow-up once the public API shape
is validated by at least one real importer (wallfacer).

### Migration of cmd/agon

`cmd/agon/main.go` and its sub-packages that import `internal/round`, `internal/agent`,
`internal/ledger`, `internal/state`, `internal/critic` are rewritten to import the
promoted `pkg/` equivalents. The `internal/` originals are deleted after the `pkg/`
replacements pass all existing tests.

Acceptance criterion: `go test ./...` passes identically before and after the
rename; `agon` binary produces bit-for-bit identical output on the existing probe suite.

### go.mod

No new external dependencies. The only change is the addition of the `pkg/adversarial`
subtree and deletion of corresponding `internal/` packages.

## Non-Goals

- Changing the debate protocol, aspect prompts, or termination logic.
- Adding a `pkg/adversarial/codex` package (deferred).
- Stabilizing a semantic-versioning guarantee for the `pkg/` API (v0, semver exempt).
- Exposing `internal/input` as a public package — callers supply `DiffPatch` as a
  pre-computed string; computing it is out of scope for the engine library.

## Phasing / Acceptance Criteria

Phase 1 — promote packages. Create `pkg/adversarial`, `pkg/adversarial/state`,
`pkg/adversarial/ledger`, `pkg/adversarial/critic`, `pkg/adversarial/claude`.
Delete the `internal/` originals. `cmd/agon` imports from `pkg/`. All existing
probe and unit tests pass unchanged.

Phase 2 — validate import. Wallfacer's `go.mod` adds `latere.ai/x/agon` as a
`replace` directive pointing to the local path `../../agon`. A wallfacer package
imports `pkg/adversarial` and compiles. This is the external-importer smoke test.
