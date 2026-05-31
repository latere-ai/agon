## Release

`agon-web` (the agon.latere.ai site) ships via local make. The CLI binaries
still ship via `release.yml` (goreleaser) on tag push.

```sh
make release-patch    # bump patch, build, push ghcr.io/latere-ai/agon-web:<new tag>
make release-minor    # bump minor
make release-major    # bump major
make release VERSION=v0.0.2   # override

make deploy           # defaults to tag at HEAD (set by release-patch)
make deploy VERSION=v0.0.2    # deploy a specific tag
git push origin <tag> # publish the tag — also triggers goreleaser for the CLI
```

Needs on PATH: `gh`, `podman` (or `docker`), `op`, `doctl`, `kubectl`.

- gh token must have `write:packages,read:packages` — `gh auth refresh -s write:packages,read:packages` if pushes 403.
- DO PAT comes from 1Password at `op://LatereAI/Digital Ocean Credentials/PAT`. Token is passed via `DIGITALOCEAN_ACCESS_TOKEN` env var, never written to disk.
- Builds `linux/amd64` by default (cluster nodes are amd64). Override with `PLATFORM=...`.
- If a release fails partway, the bump tag is rolled back automatically.
