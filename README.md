# agon

Adversarial review for Claude Code coding sessions, as a Go engine and
protocol.

After Claude finishes a task, agon forks the session for one or more
critic agents, runs a multi-round cross-examination per critic, applies
any concessions the proposer makes, and surfaces only the unresolved
disputes for human attention. Each critic picks its own attack topic in
round 1 (security, perf, internal-consistency, evidence-gap, ...); later
critics are told which topics are taken and pick something else. No agon
content ever lands in the root Claude session - agon happens in branched
forks off the root.

Design and per-component contracts live under [specs/](specs/) -
start at [specs/README.md](specs/README.md) for the index.
Release-cut evidence (probe outcomes, smoke recordings) is committed
to [release-notes-v0.0.1.md](release-notes-v0.0.1.md).

## CLI

The agon CLI ships as `latere agon` in
[latere-cli](https://github.com/latere-ai/latere-cli): it embeds this engine,
forks your real Claude Code session as the proposer, and routes critics through
Lux on your Latere identity.

```sh
latere auth login   # once
latere agon         # review the latest session under the cwd
```

See the [latere agon guide](https://github.com/latere-ai/latere-cli/blob/main/docs/agon.md)
for flags, exit codes, and how it works.

## Embedding the engine

This repository is the importable engine. Implement [`Proposer`] and
[`Critic`], wire them into an `Engine`, and call `Run`:

```go
import adversarial "latere.ai/x/agon/pkg/adversarial"

sum, err := (&adversarial.Engine{
    StateDir:  stateDir,
    Cwd:       cwd,
    ForkCount: forks,
    Proposer:  proposer,   // e.g. pkg/adversarial/claude.NewProposer(...)
    NewCritic: critics,    // e.g. pkg/adversarial/topos.NewCriticFactory(...)
    MaxRounds: maxRounds,
    DiffPatch: diff,
}).Run(ctx)
```

- `pkg/adversarial` - the public engine, interfaces, and result types ([spec 37](specs/37-pkg-public-api.md)).
- `pkg/adversarial/input` - locate the Claude transcript and compute the working-tree diff ([spec 40](specs/40-pkg-input-public.md)).
- `pkg/adversarial/claude` - claude-CLI proposer (`--resume --fork-session`) and critic.
- `pkg/adversarial/topos` - critics over [topos](https://github.com/latere-ai/topos) with model routing via Lux ([spec 39](specs/39-topos-backed-critic.md)).

A completed run writes per-fork artifacts and a contention-scored
`summary.md` under `.agon/sessions/<id>/`.

## Design architecture

```
                  user's claude session (the "root")
                            │
                            │  --fork-session (per critic)
                ┌───────────┼───────────┐
                ▼           ▼           ▼
          fork-1            fork-2 …    fork-N
          ┌──────┐          ┌──────┐    ┌──────┐
          │ pro- │ <══════> │ ...  │    │ ...  │
          │poser │  rounds  │      │    │      │
          │clone │          │      │    │      │
          └──┬───┘          └──────┘    └──────┘
             │
             ▼ writes round files to disk
   .agon/sessions/<id>/forks/critic-i/rounds/r{1,2,3,…}-{critic,proposer}.md
                                            │
                                            ▼
                                    summary.md  (contention-scored headline + leaves)
```

Five load-bearing pieces (full design in
[spec 01](specs/01-overview.md)):

- **Forked agon, no agon content in root.** Each critic gets its
  own claude fork via `--fork-session`. agon runs as a separate
  process and only ever touches the live session through that fork,
  writing results to disk. The user's root transcript never sees an
  agon turn.
- **Verbatim channel.** Critic output reaches the proposer-clone as a
  plain user turn pointing at a file: `Some comments at @<path>.
  Please resolve or respond.` No skill, slash-command, or
  plugin-template wrapping that would distort the proposer's normal
  defense behavior.
- **Self-declared topics.** Each critic chooses its own attack topic
  in R1 (security, perf, internal-consistency, evidence-gap, ...) and
  later critics are told which topics are already claimed so they
  pick something else. No fixed catalog. The debate-theoretic
  property - one competent honest player suffices for soundness -
  means a lazy critic on one topic doesn't break the others.
- **Persisted ledger.** Every attack carries a stable id (`c<critic>-<seq>`),
  every transition is appended to `attacks.jsonl`. Headlines are
  picked by a pure contention score (`rounds_survived + (1 if
  re-attacked)`) - no LLM judging at this layer.
- **Best-effort critic isolation.** "Artifact + task only" is enforced
  by the critic system prompt plus each backend's read-only posture
  (claude proposer via `--disallowedTools`, topos critic via a no-tools
  grant, `codex --sandbox read-only`), not OS isolation.

## Related work

- [agents-byzantine-tolerance](https://github.com/changkun/agents-byzantine-tolerance)
  - research repo studying multi-agent Byzantine fault tolerance,
  including
  [spec 07 / Adversarial Debate](https://github.com/changkun/agents-byzantine-tolerance/blob/main/specs/07-adversarial-debate.md),
  the architecture this tool productizes. Specs 08–13 explore
  protocol variants (compute asymmetry, recursive sub-debate,
  stochastic systems, PCP-style leaves, Prover-Estimator,
  DQC scaling). None drive v0; each one's empirical result *could*
  license a specific change here if it goes a particular way - see
  [specs/README.md §Related research](specs/README.md#related-research)
  for the conditional mapping.
- Irving, Christiano & Amodei,
  [*AI Safety via Debate*](https://arxiv.org/abs/1805.00899) (2018) -
  one agent proposes, another finds flaws, a judge inspects only the
  single disputed claim that decides the debate. The
  complexity-theoretic intuition (debate ≈ PSPACE under optimal
  play) motivates the architecture.
- Brown-Cohen, Irving & Piliouras,
  [*Scalable AI Safety via Doubly-Efficient Debate*](https://arxiv.org/abs/2311.14125)
  (2023) - extends 2018 to stochastic systems and proves soundness
  under unbounded compute asymmetry between the players. The formal
  license for applying debate to LLMs at all.
