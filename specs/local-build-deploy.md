---
title: Local-First Build and Deploy (agon / agon-web)
status: planned
depends_on:
  - ../../auth/specs/local-build-deploy.md
  - ../../terraform/specs/local-build-deploy.md
affects:
  - Makefile
  - .github/workflows/site.yml
  - README.md
  - DEPLOY_LOG.md
effort: small
trigger: parent umbrella spec; agon-web ships from this repo, goreleaser CLI release unaffected
created: 2026-05-31
updated: 2026-05-31
author: changkun
dispatched_task_id: null
---

# Local-First Build and Deploy (agon / agon-web)

## Overview

Tier B child of [[local-build-deploy]]. The agon repo ships **two** things on a `v*` tag: the `agon-web` landing site (via `site.yml`) and the agon CLI binaries (via `release.yml`, goreleaser). Only `site.yml`'s build + deploy migrate to local. `release.yml` is unchanged — goreleaser still uploads CLI binaries to GitHub releases.

## Current state

- `.github/workflows/ci.yml` — lint, vet, test. Already test-only; keep.
- `.github/workflows/real-e2e.yml` — manual-dispatch real e2e tests. No secrets relevant here; keep.
- `.github/workflows/release.yml` — goreleaser for the agon CLI binaries on tag push. **Keep unchanged.**
- `.github/workflows/site.yml` — build `Dockerfile.web` → `ghcr.io/latere-ai/agon-web:<tag>`, then `doctl` + `kubectl apply -f deploy/prod/`, `set image deployment/agon-web agon-web=<image>`, `rollout status --timeout=180s`. Both jobs on `v*` tag.
- `Makefile` — exists but has no docker/deploy targets.
- `deploy/prod/` — `deployment.yaml`, `ingress.yaml`, `service.yaml`.

## Acceptance criteria

1. `Makefile` gains the standard two-entry surface:
   - `make release VERSION=v1.2.3` builds `Dockerfile.web` → `ghcr.io/latere-ai/agon-web:v1.2.3`, pushes. Idempotent.
   - `make deploy VERSION=v1.2.3` verifies the tag exists, applies `deploy/prod/`, `set image deployment/agon-web agon-web=...`, `rollout status`, appends to `DEPLOY_LOG.md`.
   - Standard `ghcr-login` and `kubeconfig` prereqs.
2. `.github/workflows/site.yml` is **deleted**.
3. `.github/workflows/release.yml` is unchanged. `ci.yml`, `real-e2e.yml` unchanged.
4. `README.md` gains a `## Release` section documenting:
   - `make release` + `make deploy` for the web site.
   - That cutting a `v*` tag still triggers `release.yml` (goreleaser) for the CLI — that flow is unchanged.
5. `DEPLOY_LOG.md` created with a one-line header.
6. `grep -r DO_TOKEN .github/` returns nothing.

## Non-goals

- Migrating the CLI goreleaser flow. It releases to GitHub, has no DO_TOKEN, and is the right shape already.
- Touching `ci.yml` or `real-e2e.yml`.

## Implementation notes

- After this spec lands, cutting a tag still triggers `release.yml` (CLI binaries published to GitHub releases). The web image push and deploy now require explicit local `make release` + `make deploy` runs. Document this clearly in the README so the tag-cutting muscle memory doesn't accidentally assume the site auto-deploys.
- Image is `ghcr.io/latere-ai/agon-web`, deployment is `agon-web`, Dockerfile is `Dockerfile.web`. Same shape as agents/topos-web.

## Doc updates checklist

- [ ] `README.md` — new `## Release` section covering both the web make-flow and the CLI tag-flow.
- [ ] `DEPLOY_LOG.md` — new file.

## Verification

1. `cd agon && make test` — passes.
2. `make release VERSION=v0.0.0-pilot` — `ghcr.io/latere-ai/agon-web:v0.0.0-pilot` pushed.
3. `make deploy VERSION=v0.0.0-pilot` — rollout succeeds; `kubectl -n latere get pods -l app=agon-web` shows new tag.
4. `curl -I https://agon.latere.ai/` returns 200.
5. A `v0.0.0-pilot+1` tag push: `ci.yml` and `release.yml` run; `site.yml` is absent.
6. `gh release view v0.0.0-pilot+1` shows the CLI binaries from goreleaser (unchanged behavior).
7. `grep -r DO_TOKEN .github/` empty.

## Rollback

`git revert` the spec commit; `site.yml` recovered from history.
