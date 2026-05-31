## Release

One command ships everything: web image build → push → k8s deploy → smoke → GitHub release → CLI binaries via goreleaser.

```sh
make release-patch    # v0.0.7 → v0.0.8
make release-minor    # v0.0.7 → v0.1.0
make release-major    # v0.0.7 → v1.0.0
```

Needs on PATH: `gh`, `podman` (or `docker`), `op`, `doctl`, `kubectl`, `goreleaser`.

- DO PAT in 1Password at `op://LatereAI/Digital Ocean Credentials/PAT`.
- Web image: `ghcr.io/latere-ai/agon-web`, deployment `agon-web`, namespace `latere`, prod URL `https://agon.latere.ai`.
- CLI binaries: goreleaser builds linux/darwin × amd64/arm64 tarballs and attaches them to the GitHub release.
- Builds `linux/amd64` (cluster). Failure at any step rolls back the bump tag.
- Internal targets exist if you need to step through (`make release`, `make deploy`, `make release-cli`, `make ghcr-login`, `make kubeconfig`) — `release-patch` is the standard path.
