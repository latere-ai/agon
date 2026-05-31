SHELL := /bin/bash
BIN   := bin/agon
PKG   := ./cmd/agon

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all pre lint vet test build install clean probe release-check coverage e2e

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
