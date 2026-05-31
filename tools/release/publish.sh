#!/usr/bin/env bash
#
# Post-deploy release-evidence publisher for agon-web. See lectio's
# tools/release/publish.sh for the canonical shape.
#
# Required env (or first arg):
#   $1 / VERSION       release tag (e.g. v0.2.0)
#   BASE_URL           prod URL (default https://agon.latere.ai)
#
# Optional env:
#   COMMIT             release commit sha (default: HEAD)
#   BUILD_URL          link for evidence body
#   DEPLOY_URL         link for evidence body
#
# Required tools: gh, go, curl, grep.

set -euo pipefail

VERSION="${1:-${VERSION:-}}"
[ -n "$VERSION" ] || { echo "publish.sh: VERSION (arg 1) is required" >&2; exit 2; }

BASE_URL="${BASE_URL:-https://agon.latere.ai}"
COMMIT="${COMMIT:-$(git rev-parse HEAD)}"
BUILD_URL="${BUILD_URL:-local://make release $VERSION}"
DEPLOY_URL="${DEPLOY_URL:-local://make deploy $VERSION}"

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repo_root"

mkdir -p release-evidence

if ! git ls-remote --tags origin "refs/tags/$VERSION" | grep -q "$VERSION"; then
  echo "publish.sh: pushing tag $VERSION to origin"
  git push origin "$VERSION"
fi

BASE_URL="$BASE_URL" \
  TAG="$VERSION" \
  COMMIT="$COMMIT" \
  BUILD_URL="$BUILD_URL" \
  DEPLOY_URL="$DEPLOY_URL" \
  OUTPUT_MD=release-evidence/smoke.md \
  tools/smoke/release.sh

go build -o release-evidence/evidence-body ./tools/release

prefix=release-evidence/prefix.md
body=release-evidence/release-body.md
if gh release view "$VERSION" --json body --jq .body > "$prefix" 2>/dev/null; then
  release-evidence/evidence-body \
    --prefix "$prefix" --evidence release-evidence/smoke.md --out "$body"
  gh release edit "$VERSION" --notes-file "$body"
else
  repo_slug=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
  gh api "repos/$repo_slug/releases/generate-notes" \
    -f tag_name="$VERSION" \
    -f target_commitish="$COMMIT" \
    --jq .body > "$prefix" || true
  release-evidence/evidence-body \
    --prefix "$prefix" --evidence release-evidence/smoke.md --out "$body"
  gh release create "$VERSION" \
    --title "Agon $VERSION" \
    --notes-file "$body"
fi

echo "publish.sh: release $VERSION published"
