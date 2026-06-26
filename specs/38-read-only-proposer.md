---
title: Read-only proposer option
status: complete
depends_on: []
affects:
  - internal/agent/claude_proposer.go
  - pkg/adversarial/claude/claude.go
effort: small
created: 2026-06-27
updated: 2026-06-27
author: changkun
dispatched_task_id: null
---

# Read-only proposer option

## Problem

When agon is embedded as a verifier (spec 37), the fork-session proposer runs
via `claude --resume <id> --fork-session` in a working directory. Because
`--resume` is cwd-scoped (spec 17: `ErrCwdMismatch`), that directory must be the
*same tree the implementation agent ran in* — i.e. the embedder's real working
tree.

The proposer's job in the debate is to read the diff and critic attacks and then
rebut or concede. It does not need to edit files. But agon passed no tool
restriction, so whether the proposer could modify the tree depended entirely on
the host claude's default permission behavior. An embedder that runs the
proposer with elevated permissions (e.g. wallfacer, whose harness adds
`--dangerously-skip-permissions` on other paths) would have no way to guarantee
the proposer cannot mutate the very tree the embedder is about to commit.

## Goal

Give embedders an explicit, hard guarantee that the proposer cannot write to the
tree it runs in, without changing agon's standalone CLI behavior.

## Design

`agent.ClaudeProposer` gains a `DisallowedTools []string` field. When non-empty,
`FirstRound`/`NextRound` append `--disallowedTools <comma-joined>` to the claude
argv. Empty (the default) preserves today's behavior, so `cmd/agon` is
unchanged.

The public `pkg/adversarial/claude` package adds:

```go
// WithProposerReadOnly restricts the proposer to read-only tools (no Write,
// Edit, MultiEdit, NotebookEdit, or Bash).
func WithProposerReadOnly() ProposerOption
```

Bash is included in the denylist because it can write to the tree. The proposer
can still Read/Grep/Glob to investigate and argue; it simply cannot edit.

This is opt-in: agon's own CLI does not set it (its proposer may legitimately
apply fixes); embedders that share the real tree call it.

## Non-Goals

- Restricting the critic's tools. Critics are the embedder's concern — wallfacer
  runs them in a throwaway worktree where full tools are safe.
- A read-only *mode* beyond tool restriction (no sandboxing, no FS guard); this
  is defense-in-depth via claude's own `--disallowedTools`.

## Acceptance Criteria

- `ClaudeProposer.toolArgs()` returns nil by default and
  `--disallowedTools a,b,c` when set. _(tested)_
- `WithProposerReadOnly()` disables Write, Edit, MultiEdit, NotebookEdit, Bash.
  _(tested)_
- `cmd/agon` behavior is unchanged (no option set). _(existing tests pass)_
