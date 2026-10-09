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

- The adversarial review engine as its own module, `latere.ai/x/agon`,
  carried from `latere.ai/x/topos/adversarial` at topos v0.7.0: the engine
  (`agon`), the Claude Code backends (`claude`), the model critic (`critic`)
  and the diff and transcript helpers (`input`). It depends on no version of
  `latere.ai/x/topos`, so a consumer can take any topos release beside it.

### Changed

- `latere.ai/x/pkg` v0.90.2 and OpenTelemetry Go v1.46.0, past GO-2026-6615 and
  GO-2026-6505.

- Import paths move from `latere.ai/x/topos/adversarial/...` to
  `latere.ai/x/agon/...`. The root package is named `agon`: code that
  referred to `adversarial.Review` either imports the package under that
  name or refers to `agon.Review`. Every exported identifier of the root,
  `claude` and `input` is unchanged.
- `critic.Config` takes a `luxsdk.Caller` (`Model`) and a model id (`Name`)
  in place of topos `ModelOptions`, `Sandbox` and `Tools`. A critic round
  is one tool-free model call rather than an agent run, so it behaves as
  before, and it now reports the call's token usage, duration and, when the
  gateway reports one, its cost. Its tokens count against `CostCap` like
  the subprocess critics'.

### Removed

- `critic.Config.Sandbox` and `critic.Config.Tools`: a critic runs no agent,
  so it has no sandbox and is granted no tools.

### Security

- Built with Go 1.27.2 and golang.org/x/net v0.60.0, which fix GO-2026-6611, GO-2026-6612, GO-2026-6613 and GO-2026-6617.
