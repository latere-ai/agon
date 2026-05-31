## Release

`agon-web` (the agon.latere.ai site) and the agon CLI binaries both ship from
local make. No tag push triggers anything in CI — releases are explicit.

```sh
make release-patch    # bump patch, build, push ghcr.io/latere-ai/agon-web:<new tag>
make release-minor    # bump minor
make release-major    # bump major
make release VERSION=v0.0.2   # override

make deploy           # rolls web image, runs smoke, creates GitHub release,
                      # then runs goreleaser to attach CLI binaries to that release.
make deploy VERSION=v0.0.2    # deploy a specific tag
```

Needs on PATH: `gh`, `podman` (or `docker`), `op`, `doctl`, `kubectl`.

- gh token must have `write:packages,read:packages` — `gh auth refresh -s write:packages,read:packages` if pushes 403.
- DO PAT comes from 1Password at `op://LatereAI/Digital Ocean Credentials/PAT`. Token is passed via `DIGITALOCEAN_ACCESS_TOKEN` env var, never written to disk.
- Builds `linux/amd64` by default (cluster nodes are amd64). Override with `PLATFORM=...`.
- If a release fails partway, the bump tag is rolled back automatically.
- `make deploy` runs `tools/smoke/release.sh` against prod (checks `/`, `/healthz`, `/readyz`), then `tools/release/publish.sh` to push the git tag to origin and create the GitHub release with the smoke evidence body, then `make release-cli` to run goreleaser locally and attach the CLI binary archives to that same release.
- `goreleaser` must be on PATH (`brew install goreleaser`). Auth uses `gh auth token`.
