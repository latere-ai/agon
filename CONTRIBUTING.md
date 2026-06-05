# Contributing to agon

Thanks for your interest in `agon`. This guide covers how the project
is built, tested, and reviewed, and the conventions a change is
expected to follow.

`agon` is spec-driven. The specs under [specs/](specs/) are the
authoritative contracts for every slice of the system; the code is
meant to follow what the spec says, not the other way around. Read
[specs/README.md](specs/README.md) first, and start a non-trivial
change from the relevant spec.

## Layout

- `cmd/agon` and `cmd/agon-web` are the binaries.
- `internal/` holds the implementation packages.
- `e2e/` is the CLI integration and hook suite.
- `specs/` holds the authoritative design and per-component contracts.
- `frontend/` is the website (Vue + Vite), the source of the landing
  page copy in `frontend/src/content/`.

## Go toolchain

You need Go 1.26 or newer and `golangci-lint` v2 on your PATH. The
`Makefile` is the single entry point:

```sh
make           # lint, test (go test -race), then build bin/agon
make lint      # golangci-lint run ./...
make vet       # go vet ./...
make test      # go test -race -timeout 120s ./...
make build     # build bin/agon with version ldflags
make e2e       # CLI integration + hook suite
make coverage  # per-package coverage report -> coverage.html
make probe     # run the scripts/probes/*.sh probes
```

`make release-check` is the local pre-tag gate (lint, vet, test,
build, version smoke). CI runs the same set, so run it before opening
a release-bound PR.

## Frontend

The website lives in `frontend/` and uses [Bun](https://bun.sh):

```sh
cd frontend
bun install
bun run dev        # local dev server
bun run typecheck  # vue-tsc --noEmit
bun run test       # vitest run
bun run build      # type-check + static build
```

Landing-page copy is data, not markup: edit `frontend/src/content/en.ts`
and `frontend/src/content/zh.ts`. The two dictionaries are kept in
structural parity, and a vitest check fails the build if a key exists
in one language but not the other. Keep the prose in the same plain,
explanatory register as this README and the docs. Do not use em
dashes.

## Tests

Every bug fix must come with a reproducible test that fails without
the fix and passes with it. New behavior comes with the tests that
exercise it. Go code is tested with the race detector on (`make
test`); frontend logic is tested with vitest.

## Commits and pull requests

- Branch off `main`; open pull requests against `main`.
- Use [Conventional Commits](https://www.conventionalcommits.org), the
  style already in the history: `feat(...)`, `fix(...)`, `docs(...)`,
  `ci(...)`, `refactor(...)`, `revert: ...`.
- Keep each commit to one small, self-contained scope. Many small
  commits are preferred over one large one.
- Make sure `make` (or `make release-check` for release-bound work) is
  green before you push.

## Reviewing changes with agon

`agon` reviews its own changes. After a coding session, run a
verification pass over the diff and address what survives:

```sh
agon --side-count 4 --max-turn 6
```

It forks the producer session, spawns independent critics, runs the
bounded debate, and writes an auditable session under `.agon/`. See
the [README](README.md) for the full usage and exit codes.
