SHELL := /bin/bash

# agon is a library (pkg/adversarial) plus the agon-web landing site. The
# standalone CLI was sunset in favour of `latere agon` (latere-cli), so there
# is no binary to build or install here.

.PHONY: all pre lint vet test build clean probe coverage

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
	go build ./...

clean:
	rm -rf bin coverage.txt

probe:
	@for s in scripts/probes/*.sh; do \
	  printf '== %s ==\n' "$$s"; \
	  "$$s" || exit $$?; \
	done

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
