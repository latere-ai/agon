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

- `pkg/adversarial/` is the public engine: the `Engine`, `Proposer`,
  and `Critic` interfaces, and the `claude`, `topos`, and `input`
  subpackages embedders build on.
- `internal/` holds the implementation packages the engine composes.
- `cmd/agon-web/` and `frontend/` are the `agon.latere.ai` landing
  site (Go server + Vue/Vite source); landing copy lives in
  `frontend/src/content/`.
- `specs/` holds the authoritative design and per-component contracts.

The CLI is not built here: it ships as `latere agon` in
[latere-cli](https://github.com/latere-ai/latere-cli), which imports
this engine.

## Go toolchain

You need Go 1.26 or newer and `golangci-lint` v2 on your PATH. The
`Makefile` is the single entry point:

```sh
make           # lint, test (go test -race), then build
make lint      # golangci-lint run ./...
make vet       # go vet ./...
make test      # go test -race -timeout 120s ./...
make build     # go build ./...
make coverage  # per-package coverage report -> coverage.html
```

Run `make` before opening a PR; CI runs the same lint + test + build.

## Frontend

The landing site lives in `frontend/` and uses [Bun](https://bun.sh):

```sh
cd frontend
bun install
bun run dev        # local dev server
bun run typecheck  # vue-tsc --noEmit
bun run test       # vitest run
bun run build      # type-check + static build
```

Landing-page copy is data, not markup: edit
`frontend/src/content/en.ts` and `frontend/src/content/zh.ts`, then run
`bun run build` and commit the regenerated `.js` alongside (the build
emits compiled `.js` next to each source, and Vite loads those). The
two dictionaries are kept in structural parity, and a vitest check
fails the build if a key exists in one language but not the other.
Keep the prose in the same plain, explanatory register as the README
and docs. Do not use em dashes.

The site deploys to `agon.latere.ai` only on a `v*` tag (via
`site.yml`); pushing to `main` does not deploy.

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
- Make sure `make` is green before you push.

## Reviewing changes with agon

`agon` reviews its own changes through `latere agon`. After a coding
session, run a verification pass over the diff and address what
survives:

```sh
latere agon --forks 4 --max-rounds 6
```

It forks the producer session, spawns independent critics, runs the
bounded debate, and writes an auditable session under `.agon/`. See
the [latere agon guide](https://github.com/latere-ai/latere-cli/blob/main/docs/agon.md)
for the full usage and exit codes.
