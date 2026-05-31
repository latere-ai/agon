## Release

`agon-web` (the agon.latere.ai site) and the agon CLI binaries both ship from
local make. No tag push triggers anything in CI — releases are explicit.

```sh
make release-patch    # bump patch, build, push ghcr.io/latere-ai/agon-web:<new tag>
make release-minor    # bump minor
make release-major    # bump major
make release VERSION=v0.0.2   # override

make deploy           # rolls web image, runs smoke, creates GitHub release.
make deploy VERSION=v0.0.2    # deploy a specific tag

make release-cli      # opt-in: build CLI binaries via goreleaser and attach
                      # them to the existing GitHub release. Only run when
                      # you actually want to ship a new CLI version.
```

Needs on PATH: `gh`, `podman` (or `docker`), `op`, `doctl`, `kubectl`.

- gh token must have `write:packages,read:packages` — `gh auth refresh -s write:packages,read:packages` if pushes 403.
- DO PAT comes from 1Password at `op://LatereAI/Digital Ocean Credentials/PAT`. Token is passed via `DIGITALOCEAN_ACCESS_TOKEN` env var, never written to disk.
- Builds `linux/amd64` by default (cluster nodes are amd64). Override with `PLATFORM=...`.
- If a release fails partway, the bump tag is rolled back automatically.
- `make deploy` runs `tools/smoke/release.sh` against prod (checks `/`, `/healthz`, `/readyz`), then `tools/release/publish.sh` to push the git tag to origin and create the GitHub release with the smoke evidence body.
- `make release-cli` is a separate manual target: runs `goreleaser release --clean` locally to build CLI binaries for linux/darwin × amd64/arm64 and attach them to the existing GitHub release. Needs `goreleaser` on PATH (`brew install goreleaser`). Only run when you want to ship a new CLI version — web deploys don't require it.
