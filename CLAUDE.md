## Release

`agon-web` (the agon.latere.ai site) ships via local make. The CLI binaries
still ship via `release.yml` (goreleaser) on tag push.

```sh
make release-patch    # bump patch, build, push ghcr.io/latere-ai/agon-web:<new tag>
make release-minor    # bump minor
make release-major    # bump major
make release VERSION=v0.0.2   # override

make deploy           # defaults to tag at HEAD; runs smoke + publishes GitHub release (web)
make deploy VERSION=v0.0.2    # deploy a specific tag
# Pushing the tag during deploy ALSO triggers goreleaser for the CLI binaries.
```

Needs on PATH: `gh`, `podman` (or `docker`), `op`, `doctl`, `kubectl`.

- gh token must have `write:packages,read:packages` — `gh auth refresh -s write:packages,read:packages` if pushes 403.
- DO PAT comes from 1Password at `op://LatereAI/Digital Ocean Credentials/PAT`. Token is passed via `DIGITALOCEAN_ACCESS_TOKEN` env var, never written to disk.
- Builds `linux/amd64` by default (cluster nodes are amd64). Override with `PLATFORM=...`.
- If a release fails partway, the bump tag is rolled back automatically.
- `make deploy` runs `tools/smoke/release.sh` against prod (checks `/`, `/healthz`, `/readyz`), then `tools/release/publish.sh` to push the git tag to origin and create a GitHub release for the web side. Pushing the tag also triggers `release.yml` (goreleaser) to publish the CLI binaries — this is intentional, one tag = one ship of both.
