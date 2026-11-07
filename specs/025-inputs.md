---
title: Inputs
status: complete
track: adversarial
updated: 2026-11-07
author: changkun
---

# Inputs

`input` gives an embedder the two inputs a debate needs that are not
the embedder's own: the coding agent's session transcript and the working-tree
diff. It is a public package so `latere agents review` and other embedders build a
`VerifyInput` (or fill an `Engine`) without re-implementing transcript location or
diff computation. Grounded in `transcript.go` and `diff.go`.

## Transcript location and ingest

Claude Code writes each session to
`~/.claude/projects/<encoded-cwd>/<sessionID>.jsonl`.

- `LocateTranscript(home, cwd, sessionID, explicit)` resolves the path: an
  explicit path wins, otherwise the encoded-cwd location is used. Returns
  `ErrTranscriptNotFound` when neither exists.
- `ValidateSessionID(id)` is the rule for a session id: ASCII letters, digits,
  `_` and `-`, at most 128 characters, no leading `-`. `LocateTranscript` (when
  no explicit path is given) and `FindSession` return `ErrInvalidSessionID` for
  an id outside it, so an id cannot name a file outside `~/.claude/projects`.
- `FindSession(home, sessionID)` scans `~/.claude/projects/*/` for a session when
  the cwd is unknown, returning the path and the encoded segment.
- `ReadTranscript(path)` streams the JSONL and returns a `Transcript` with `Path`,
  `SessionID`, `FirstUser` (the first user turn, used as the task context),
  `StartedAt`, and `LineCount`. It tolerates a small fraction of malformed lines
  and returns `ErrTranscriptMalformed` past that, or `ErrNoUserTurn` when there is
  no user turn.
- `ExtractFirstUser`, `EncodeCwd`, `DecodeCwd` are the helpers. `EncodeCwd` matches
  Claude Code's `/` and `.` to `-` mapping; `DecodeCwd` is best-effort because the
  encoding is lossy, so compare encoded forms rather than decoded ones.

There is no "most recent session" helper: the engine takes a session ID directly,
so an embedder that wants "newest under cwd" (as `latere agents review` does) implements
that selection itself.

## Working-tree diff

- `Compute(ctx, DiffSpec) (*Diff, error)` runs `git diff` for the spec.
  `DiffSpec{From, To, Cwd}`: `From="HEAD", To="."` is the working tree versus HEAD
  including untracked files; a committed range is `From=A, To=B`. `Diff` carries
  `Patch`, `ChangedLines`, and `Files`.
- `From` and `To` are revisions, never options. `Compute` refuses one that
  starts with `-`, and passes them to git after `--end-of-options`, which needs
  git 2.24 or newer. Untracked names are read NUL-separated (`ls-files -z`), so a
  name with non-ASCII bytes, quotes or a newline is diffed like any other.
- `Compute` refuses a diff over `MaxDiffBytes` (4 MiB) with `ErrDiffTooLarge`,
  and stops git as soon as its output passes the cap. Every critic round sends
  the whole diff in one prompt, and 4 MiB is about the most a model context of
  a million tokens holds.
- `Compute` runs git with the repository's own configuration, so a configured
  external diff or textconv driver runs. Do not compute a diff in a checkout you
  do not trust.
- `Trivial(d, threshold)` reports whether `ChangedLines < threshold`, the gate an
  embedder uses to skip a debate on a near-empty change.
- Errors are `ErrNotGitRepo` and `ErrGit{Args, Stderr, Err}`.

A common embedder pattern is to compute `HEAD` versus the tree, and when that is
empty (the agent already committed) fall back to `HEAD~1..HEAD`. That fallback is
the embedder's policy, not this package's.
