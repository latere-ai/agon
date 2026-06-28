---
title: Public pkg/adversarial/input package
status: complete
depends_on:
  - specs/37-pkg-public-api.md
affects:
  - pkg/adversarial/input/transcript.go
  - pkg/adversarial/input/diff.go
  - pkg/adversarial/input/doc.go
  - internal/cli/preflight.go
  - cmd/agon/main.go
created: 2026-06-28
updated: 2026-06-28
author: changkun
dispatched_task_id: null
---

# Public pkg/adversarial/input package

## Problem

Spec 37 promoted the engine cluster to `pkg/adversarial` but deliberately left
`internal/input` private: "callers supply `DiffPatch` as a pre-computed string;
computing it is out of scope for the engine library." That was the right call
for wallfacer, the first embedder, because wallfacer already has the diff in
hand (it runs critics in a worktree it controls) and never needs to locate a
Claude Code session.

The new local-embed path breaks that assumption. We are making `pkg/adversarial`
the shared protocol spine for two deployments: a local `latere agon` subcommand
that runs the full-fidelity debate on the developer's machine, and a hosted
`Verifier` service. The local path is the one that needs input handling: before
it can build a `VerifyInput`, `latere agon` must (1) locate the developer's
Claude Code session transcript under `~/.claude` and read the first user turn
for task context, and (2) compute the working-tree git diff and apply the
trivial-diff fast path. Today that logic is `internal/input`, importable only
from within the agon module. latere-cli would have to reimplement transcript
location, cwd-encoding, JSONL parsing, and `git diff` capture, duplicating
load-bearing, already-tested code that must stay byte-compatible with the
proposer's `claude --resume` cwd-scoping (spec 17).

So the input layer needs the same treatment spec 37 gave the engine: promote it
to a public package, unchanged in behavior.

## Goal

Promote `internal/input` to `pkg/adversarial/input` so external Go modules
(latere-cli first) can locate a Claude transcript and compute a working-tree
diff with the same code agon uses, instead of reimplementing it. The standalone
`cmd/agon` binary rewires to import the promoted package and its observable
behavior must be bit-for-bit identical to today. No user-facing change to `agon`
itself, mirroring the spec 37 promotion.

## Current State

`internal/input` (`doc.go`: "reads the claude session transcript and computes
the working-tree diff. Filled by specs 07, 08") has two halves and three
importers in the agon module.

Transcript half (`transcript.go`, spec 07):

- `Transcript` (struct)
- `EncodeCwd(cwd string) string`, `DecodeCwd(encoded string) string`
- `LocateTranscript(home, cwd, sessionID, explicit string) (string, error)`
- `FindSession(home, sessionID string) (path, encodedSegment string, err error)`
- `ReadTranscript(path string) (*Transcript, error)`
- `ExtractFirstUser(records [][]byte) (string, error)`

Diff half (`diff.go`, spec 08):

- `DiffSpec` (struct), `Diff` (struct)
- `Compute(ctx context.Context, s DiffSpec) (*Diff, error)`
- `Trivial(d *Diff, threshold int) bool`
- `ErrNotGitRepo` (sentinel), `ErrGit` (struct)

Importers (`grep -rln 'agon/internal/input'`):

- `cmd/agon/main.go`
- `internal/cli/preflight.go`
- `internal/cli/preflight_more_test.go`

Test files `transcript_test.go` and `diff_test.go` live alongside and cover both
halves (round-trip encode/decode, transcript location by encoded cwd, session
discovery, empty/untracked/bad-range diffs, the trivial gate).

## Design

### Placement and boundary

Move the package wholesale to `pkg/adversarial/input`, a sibling of
`pkg/adversarial/claude` and `pkg/adversarial/topos`. The import path becomes
`latere.ai/x/agon/pkg/adversarial/input`. This is a pure relocation: no API
shape change, no new types, no behavior change. The package keeps its two-file
split (`transcript.go`, `diff.go`) and its tests move with it unchanged except
for the package's own internal references (there are none across the two files;
they share only the package name).

The package stays free of `pkg/adversarial` itself: it produces a `*Diff` and a
`*Transcript`, and the caller maps those into a `VerifyInput.DiffPatch` /
`TaskPrompt`. Keeping `input` independent of the engine package preserves the
spec 37 layering (the engine never imports input) and lets an embedder pull in
just the input helpers without the engine, or vice versa.

No new external dependencies. `go.mod` is unchanged; this is a tree move plus
import rewrites, exactly the spec 37 mechanics.

### Migration of importers

Rewrite the three importers to import `pkg/adversarial/input` instead of
`internal/input`:

- `cmd/agon/main.go`
- `internal/cli/preflight.go`
- `internal/cli/preflight_more_test.go`

The call sites are unchanged beyond the import path because the exported surface
is identical. Delete `internal/input` after the `pkg/` copy passes all existing
tests, mirroring spec 37's "delete the `internal/` originals after the `pkg/`
replacements pass."

## Non-Goals

- Changing any input behavior: transcript format, cwd encoding, session
  discovery order, diff range semantics, or the trivial-diff threshold. Relocation
  only. Specs 07 and 08 remain the behavioral contract.
- Adding input helpers to the engine (`pkg/adversarial` does not import
  `input`; the boundary from spec 37 stands).
- Wiring `latere agon` to call this package. That is the local-track CLI spec in
  latere-cli; this spec only makes the package importable.
- Promoting any other `internal/` package (e.g. `internal/agent`,
  `internal/cli`). Out of scope.

## Acceptance Criteria

- `pkg/adversarial/input` exports `Transcript`, `EncodeCwd`, `DecodeCwd`,
  `LocateTranscript`, `FindSession`, `ReadTranscript`, `ExtractFirstUser`,
  `DiffSpec`, `Diff`, `Compute`, `Trivial`, `ErrNotGitRepo`, and `ErrGit` with
  signatures identical to today's `internal/input`. _(compile + existing tests)_
- The promoted `transcript_test.go` and `diff_test.go` pass unchanged under the
  new package path. _(tested)_
- `internal/input` no longer exists; `grep -rln 'agon/internal/input'` returns
  nothing. _(verified)_
- `cmd/agon`, `internal/cli/preflight.go`, and `preflight_more_test.go` import
  `pkg/adversarial/input` and build. _(compile)_
- `go test ./...` passes identically before and after, and `agon` produces
  bit-for-bit identical output on the existing probe suite. _(existing tests
  pass; spec 37 acceptance pattern)_
- An external module can import `latere.ai/x/agon/pkg/adversarial/input` and call
  `LocateTranscript` + `Compute`. _(smoke: covered when latere-cli's local
  subcommand lands; not blocking this spec)_

## Outcome

Implemented in `e68d072`. The five files moved with `git mv` (tracked as pure
renames, no content change) to `pkg/adversarial/input`; the three importers
(`cmd/agon/main.go`, `internal/cli/preflight.go`, `preflight_more_test.go`)
rewired to the new path. `cmd/agon`'s import group was re-sorted (the new path
sorts after the `internal/*` imports). `internal/input` no longer exists.
`go build ./...`, `go vet ./...`, `go test ./...`, and gofumpt all pass; exported
surface and behavior are unchanged. The local-track input blocker is cleared.
