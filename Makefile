SHELL := /bin/bash
BIN   := bin/agon
PKG   := ./cmd/agon

VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo sha-$$(git rev-parse --short HEAD))
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

IMAGE      := ghcr.io/latere-ai/agon-web
DEPLOYMENT := agon-web
NAMESPACE  := latere
CLUSTER    := latere-k8s
OP_DO_PAT  := op://LatereAI/Digital Ocean Credentials/PAT

# docker is the default; podman is detected as a drop-in. Override with
# DOCKER=... if both are installed and you want a specific one.
DOCKER ?= $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null || echo docker)

# Cluster nodes are linux/amd64. Build for that explicitly so Apple Silicon
# laptops don't push arm64 images that pods refuse with "exec format error".
PLATFORM ?= linux/amd64

.PHONY: all pre lint vet test build install clean probe release-check coverage e2e \
        release release-cli release-patch release-minor release-major deploy ghcr-login kubeconfig \
        preflight-release preflight-deploy

all: pre test build

# pre — golangci-lint v2 vet runs before build and test.
pre: lint

lint:
	golangci-lint run ./...

vet:
	go vet ./...

test: pre
	go test -race -timeout 120s ./...

build: pre
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

install:
	go install $(PKG)

clean:
	rm -rf bin coverage.txt

probe:
	@for s in scripts/probes/*.sh; do \
	  printf '== %s ==\n' "$$s"; \
	  "$$s" || exit $$?; \
	done

# Run the full local end-to-end test suite (CLI integration + hook).
e2e: pre build
	go test -timeout 180s ./e2e/...

# Coverage report. Per-package mode: each package's tests cover its
# own code. The previous -coverpkg=./... flavour produced misleading
# numbers because go test concatenates per-process profiles and the
# function-entry block ends up reported with stale 0-counts from
# packages that did not exercise the function. Per-package is the
# standard Go practice that `go tool cover -func` expects.
coverage: pre
	go test -coverprofile=coverage.out -covermode=atomic ./...
	@printf 'Total coverage: '
	@go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo 'HTML report: coverage.html'

# release-check is the local pre-tag gate: lint, vet, test, build,
# version smoke. CI runs the same.
release-check: pre vet test build
	@./$(BIN) --version
	@echo "release-check: OK"

# Bump to the next semver from the latest v* tag, create an annotated tag at
# HEAD, then run `make release` with it. Push the tag to origin yourself
# (`git push origin <tag>`) when ready. Pre-release suffixes (e.g. -rc1) are
# stripped before incrementing. Split into two recipe lines so `make -n` does
# not actually create the tag (GNU make recurses into $(MAKE) under -n).
release-patch: BUMP := patch
release-minor: BUMP := minor
release-major: BUMP := major
release-patch release-minor release-major: preflight-release
release-patch release-minor release-major:
	@latest=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$latest" ]; then \
		next="v0.1.0"; \
	else \
		ver=$${latest#v}; \
		major=$${ver%%.*}; rest=$${ver#*.}; \
		minor=$${rest%%.*}; patch=$${rest#*.}; patch=$${patch%%-*}; \
		case "$(BUMP)" in \
			major) next="v$$((major+1)).0.0" ;; \
			minor) next="v$$major.$$((minor+1)).0" ;; \
			patch) next="v$$major.$$minor.$$((patch+1))" ;; \
		esac; \
	fi; \
	echo "bump: $${latest:-<none>} → $$next"; \
	git tag -a "$$next" -m "release $$next"
	@tag=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	$(MAKE) release VERSION="$$tag" || { \
		echo "release: failed; rolling back tag $$tag" >&2; \
		git tag -d "$$tag"; \
		exit 1; \
	}

# Build the agon-web image with the explicit version tag and push to ghcr.io.
# Dockerfile.web builds the frontend inside a bun stage, so no local
# frontend-build prerequisite is needed.
release: preflight-release ghcr-login
	$(DOCKER) build --platform $(PLATFORM) -f Dockerfile.web --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .
	$(DOCKER) push $(IMAGE):$(VERSION)

# Apply manifests, roll the new image, wait for readiness, append to DEPLOY_LOG.md.
deploy: preflight-deploy kubeconfig
	@$(DOCKER) pull $(IMAGE):$(VERSION) >/dev/null 2>&1 || { \
		echo "deploy: $(IMAGE):$(VERSION) not in ghcr.io" >&2; \
		case "$(VERSION)" in \
			sha-*) \
				latest=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
				echo "hint: HEAD is not on a v* tag (VERSION fell back to $(VERSION))." >&2; \
				echo "      run 'make release-patch' to bump + push + tag HEAD, then 'make deploy'." >&2; \
				echo "      or 'make deploy VERSION=$${latest:-<no v* tags yet>}' to deploy the last tag." >&2; \
				;; \
			*) \
				echo "hint: this version was never pushed; run 'make release VERSION=$(VERSION)' first." >&2; \
				;; \
		esac; \
		exit 1; \
	}
	kubectl apply -f deploy/prod/
	kubectl -n $(NAMESPACE) set image deployment/$(DEPLOYMENT) $(DEPLOYMENT)=$(IMAGE):$(VERSION)
	@set -e; \
		out=$$(kubectl -n $(NAMESPACE) rollout status deployment/$(DEPLOYMENT) --timeout=180s); \
		echo "$$out"; \
		sha=$$(printf '%s' "$$out" | shasum -a 256 | cut -d' ' -f1 | cut -c1-12); \
		printf '| %s | %s | %s |\n' "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(VERSION)" "$$sha" >> DEPLOY_LOG.md
	VERSION=$(VERSION) bash tools/release/publish.sh

# Build and upload CLI binary archives to the existing GitHub release.
# Independent of `make deploy` — run only when you actually want to ship a new
# CLI version. Requires goreleaser (`brew install goreleaser`); the
# .goreleaser.yaml has `use_existing_release: true` so this attaches assets to
# the release that publish.sh already created.
release-cli:
	@command -v goreleaser >/dev/null 2>&1 \
		|| { echo "missing: goreleaser (brew install goreleaser)" >&2; exit 1; }
	GITHUB_TOKEN=$$(gh auth token) goreleaser release --clean

preflight-release:
	@command -v $(DOCKER) >/dev/null 2>&1 \
		|| { echo "missing: docker or podman (install OrbStack, Docker Desktop, colima, or podman)" >&2; exit 1; }
	@command -v gh >/dev/null 2>&1 \
		|| { echo "missing: gh (brew install gh)" >&2; exit 1; }

preflight-deploy: preflight-release
	@for cmd in kubectl op doctl; do \
		command -v $$cmd >/dev/null 2>&1 \
			|| { echo "missing: $$cmd (deploy needs kubectl + op + doctl)" >&2; exit 1; }; \
	done

ghcr-login:
	@gh auth token | $(DOCKER) login ghcr.io -u $$USER --password-stdin

kubeconfig:
	@if [ "$$(kubectl config current-context 2>/dev/null)" != "$(CLUSTER)" ]; then \
		DIGITALOCEAN_ACCESS_TOKEN=$$(op read "$(OP_DO_PAT)") \
			doctl kubernetes cluster kubeconfig save $(CLUSTER) --expiry-seconds 3600; \
	fi
