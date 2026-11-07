# Changelog

Every tag has a section here, and the section is the body of the GitHub
release. A tag without one is refused at the pre-push and fails the release
workflow. Write under `Unreleased` as work lands; `lateregate release vX.Y.Z`
turns that into the tag's section, commits, tags and pushes.

A section says what changed for whoever uses the release, not what was
committed: the commit log already holds that.

The Go module proxy holds versions of `latere.ai/x/agon` through v0.2.6
from an earlier, unrelated tree, and a published version cannot be
withdrawn. The first release of this tree is therefore v0.3.0 or later, so
that it sorts above them and `@latest` resolves to it.

## Unreleased

### Added

- **Each session records what it reviewed.** `start.json` holds the task,
  the budgets and the start time, and `diff.patch` holds the diff. Both are
  written before the first agent call, so a run that fails midway still
  shows its inputs.
- `input.ValidateSessionID` and `input.ErrInvalidSessionID`, the rule for a
  Claude Code session id.

### Changed

- **`Engine.Run` returns an error when it cannot write the session.** A
  failed write of `summary.md`, `end.json`, the attack ledger or the
  transcript used to be ignored, and the run returned a summary for a
  session that looked unfinished on disk.
- **A nil `Proposer`, `NewCritic` or result is an error,** returned before
  anything is written, instead of a panic.
- **`input.Compute` needs git 2.24 or newer.** It passes revisions after
  `--end-of-options`.
- **Untracked files with a non-ASCII name appear unquoted** in `Diff.Files`
  and in the patch headers.

### Fixed

- **The proposer can read the critic's comments.** The message to the
  proposer named the critic's round file relative to the session
  directory, but the proposer runs in the repository, where that path
  names nothing. A `claude` proposer then searched for the file and could
  read the comments of another review. The message now gives the absolute
  path.
- **The `claude -p` critic takes a diff of any size.** The prompt, diff
  included, was one command-line argument, so a large diff failed with
  `argument list too long` and ended the run. The prompt now goes on
  standard input.
- **One critic reply without the attack header no longer ends the run.**
  It counts as a malformed round and is kept as the round file. Two in a
  row end the fork with `malformed-output`, as the protocol says.
- **A large attack stays in the summary.** A ledger line over 4 MiB, such
  as an attack with a huge location, stopped the reading of the ledger, so
  that attack and every later one vanished, and the run read clean. The
  location now counts toward the 64 KiB at which a record moves to its own
  file, and a ledger that cannot be read is an error.
- **A long claim shows in the headline.** A claim over 64 KiB moved to its
  own file and was never read back, so the headline and `summary.md`
  showed it empty.
- **`summary.md` keeps the aspect after a defense.** After a concede or a
  rebut, `[security/a.go:1]` read `[/a.go:1]`.
- **A defense of an attack the fork did not raise is ignored.** A concede
  or rebut of an unknown id, or of another fork's attack, added a resolved
  attack with no claim and counted it in the critic-found-bug rate.
- **An attack id with a huge number is renamed.** One such id made the
  next id negative, and the proposer could not answer that attack.
- **`Summary.SessionDir` is always absolute.** It was relative when
  `StateDir` was.
- **`end.json` describes the run.** `ended_at`, `cost_cap` and
  `rounds_executed_per_fork` are filled in, and `exit_code` is 0 for a
  clean run, 130 for an interrupted run and 1 otherwise. It was always 0.
- **`input.Compute` diffs untracked files of any name.** A name with
  non-ASCII bytes, a quote or a newline made the whole diff fail.
- The comment on `Summary.Unresolved` says what it counts: attacks still
  open or rebutted at the end.

### Security

- **A session id cannot leave the projects folder or act as an option.**
  `input.LocateTranscript` and `input.FindSession` put the id into a path
  unchecked, so `../../x` read a file outside `~/.claude/projects`, and
  `claude.NewProposer` passed it to `claude`, where an id starting with
  `-` reads as an option. An id outside `[A-Za-z0-9_-]`, longer than 128
  characters or starting with `-` is now refused. This is incompatible
  for a caller that uses other ids.
- **A revision cannot act as a git option.** `input.Compute` passed `From`
  and `To` to git unchecked, so `--output=<file>` made git write a file.
  A revision that starts with `-` is now refused.
- **The aspect and location of an attack are bounded in `summary.md`.**
  They were printed in full, so a critic could fill the file or break its
  list with a newline. They are now one line of at most 200 characters,
  like the claim.

## v0.3.1 - 2026-11-05

### Fixed

- **Built with Go 1.27.3,** which fixes GO-2026-6809, a flaw in the
  standard library's certificate verification.

- **A summary's list item stays on one line.** Every run of whitespace in
  a field, a carriage return, a tab or a line separator included, becomes
  one space, so a field no longer breaks the list it sits in.

## v0.3.0 - 2026-10-21

### Added

- The adversarial review engine as its own module, `latere.ai/x/agon`,
  carried from `latere.ai/x/topos/adversarial` at topos v0.7.0: the engine
  (`agon`), the Claude Code backends (`claude`), the model critic (`critic`)
  and the diff and transcript helpers (`input`). It depends on no version of
  `latere.ai/x/topos`, so a consumer can take any topos release beside it.

### Changed

- `latere.ai/x/pkg` v0.90.2, past GO-2026-6615 and GO-2026-6505. The module
  graph holds no OpenTelemetry or gRPC: the critic's model client is the
  Latere Go SDK's `llm` package, which imports the standard library alone.

- Import paths move from `latere.ai/x/topos/adversarial/...` to
  `latere.ai/x/agon/...`. The root package is named `agon`: code that
  referred to `adversarial.Review` either imports the package under that
  name or refers to `agon.Review`. Every exported identifier of the root,
  `claude` and `input` is unchanged.
- `critic.Config` takes a `critic.Generator` (`Model`) and a model id
  (`Name`) in place of topos `ModelOptions`, `Sandbox` and `Tools`. A
  `Generator` is any type with the `Generate` method of the Latere Go SDK's
  models client, `*llm.Client` from `latere.ai/x/sdk-go/llm`; a
  provider-direct `*llmdirect.Client` from `latere.ai/x/pkg/llmdirect` is
  one too. A critic round is one tool-free model call rather than an agent
  run, so it behaves as before, and it now reports the call's token usage,
  duration and, when the gateway reports one, its cost. Its tokens count
  against `CostCap` like the subprocess critics'.

### Removed

- `critic.Config.Sandbox` and `critic.Config.Tools`: a critic runs no agent,
  so it has no sandbox and is granted no tools.

### Security

- Built with Go 1.27.2 and golang.org/x/net v0.60.0, which fix GO-2026-6611, GO-2026-6612, GO-2026-6613 and GO-2026-6617.
