# Contributing

Thank you for your interest. This file is for the people and agents who
change agon. If you use it, the [README](README.md) and the
[API reference](https://pkg.go.dev/latere.ai/x/agon) are for you. The
contracts the code keeps, with the reasons behind them, are in
[`specs/`](specs/README.md).

## The tree

| Path | What it holds |
|---|---|
| module root | The public engine: `Review`, `Engine`, the `Proposer` and `Critic` interfaces, and the result types |
| `claude/` | A proposer that forks a Claude Code session and a critic that runs `claude -p` |
| `critic/` | A critic that makes one model call per round through a `critic.Generator` |
| `input/` | The working-tree diff and the lookup of a Claude Code transcript |
| `internal/round/` | The debate loop: forks, rounds and termination |
| `internal/critic/` | The critic's side of the protocol: aspects, the attack format and its parser |
| `internal/ledger/` | The attack ledger, `attacks.jsonl`, and its aggregation |
| `internal/state/` | The session directory on disk |
| `internal/summary/` | Contention scores, the headline and `summary.md` |
| `internal/agent/` | Running the `claude` and `codex` processes and reading their output streams |
| `specs/` | The contracts: architecture, backends, the engine API, inputs, the debate protocol and the session format |

## Getting set up

You need Go 1.27 or later and `git`. Nothing else: no model key and no
`claude` binary, because the tests use fakes.

```sh
git clone https://github.com/latere-ai/agon.git && cd agon
git config core.hooksPath .githooks   # run the checks before each commit and push
go tool lateregate                    # the whole quality bar, the same one CI runs
```

`go tool lateregate list` names the gates, and `go tool lateregate <name>`
runs one of them. The bar covers formatting, lint, the tests with and
without the race detector, a coverage floor of 90% in each package (the
`claude` package is exempt in `.lateregate.yaml`, with the reason), known
vulnerabilities, the license notice on every file, and the spec tree. The
tests also run with only `/usr/bin` and `/bin` on `PATH`, where `git` and
`sh` live.

Two rules hold the design together, and tests check both:

- The engine core imports no model client and no agent runtime. Only the
  `critic` package calls a model, and nothing imports
  `latere.ai/x/topos`. `critic/boundary_test.go` checks it.
- The engine writes only under the `StateDir` its caller gives it and
  invents no default location.

## Sending a change

1. Fork the repository and make a branch.
2. Make one logical change per commit, and stage files by name.
3. Write the subject as `scope: what changed`, in lowercase, for example
   `ledger: keep a conceded attack conceded after a rebuttal`.
4. Open a pull request.

A bug fix comes with a test that fails without the fix. A change to a
contract (the exported API, the debate protocol or the session format)
changes its spec under `specs/` in the same commit. A change a user would
notice gets a line under `## Unreleased` in `CHANGELOG.md`; an
incompatible one says so there.

Maintainers push to `main` directly, and CI runs the same bar on every
push and pull request. A release is cut with
`go tool lateregate release vX.Y.Z`, which turns `Unreleased` into the
tag's section. If you plan a large change, open an issue first.

## Writing

Every sentence has one reader, and its register follows that reader: the
README and the templates are for users, package documentation and specs
for builders, and comments, log lines and error details for whoever
debugs the code. The rule and its checklist are in
[`docs/writing/registers.md`](https://github.com/latere-ai/pkg/blob/main/docs/writing/registers.md)
in `latere-ai/pkg`. Use American English, and no em dashes.

## Reporting a vulnerability

Do not open an issue. [`SECURITY.md`](SECURITY.md) says where to send the
report.
