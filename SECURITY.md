# Security

## Reporting a vulnerability

Report one privately, through
[GitHub's private vulnerability reporting](https://github.com/latere-ai/agon/security/advisories/new)
or by email to security@latere.ai. Do not open a public issue for it. The
[organization's security policy](https://github.com/latere-ai/.github/blob/main/SECURITY.md)
sets the response times, how disclosure works, and the safe harbor for
research done in good faith.

agon is before v1.0.0, so there is one release line: a fix ships in the
next release, and the [changelog](CHANGELOG.md) says which release has it.

## What is protected

agon runs inside your program and with your permissions. The text it
handles comes from people and models you may not trust: the diff and the
task text, the critic's reply, and the proposer's reply. The session ids
and revisions your program passes in may come from a user too. Against
each of them, agon commits to the following, and the named test checks
each property.

| Property | Tests |
|---|---|
| A session id cannot name a file outside `~/.claude/projects`, and cannot be read as a `claude` option. An id outside `[A-Za-z0-9_-]`, longer than 128 characters or starting with `-` is refused before any file is read or any process starts. | `input.TestSessionLookupRefusesTraversal`, `claude.TestNewProposerRefusesOptionShapedSessionID`, `agent.TestClaudeProposerRefusesOptionShapedSessionID` |
| A revision cannot be read as a `git` option. `input.Compute` refuses a `From` or `To` that starts with `-`. | `input.TestComputeRefusesOptionShapedRefs` |
| Text from a model never becomes a path. The only model text in a file name is an attack id, and an id is renamed unless it has the form `c<n>-<n>`, with numbers up to 1,000,000. | `critic.TestParseRenamesReusedPriorIDOnIntroduce`, `critic.TestParseBoundsHugeIDSequence` |
| A critic cannot hide an attack by making it large. A record over 64 KiB moves to its own file and is read back, so it stays in the summary. | `agon.TestHugeLocationKeepsAttackUnresolved`, `agon.TestSpilledClaimIsTheHeadline`, `ledger.TestAggregateReportsOversizedLine` |
| A proposer can only answer the attacks of its own fork. A `concede` or `rebut` of an id that its critic did not raise changes nothing. | `round.TestDefenseForUnknownIDIsIgnored`, `round.TestDefenseForOtherForkIsIgnored` |
| Each piece of model text in `summary.md`, such as a claim, an aspect or a location, is one line of at most 200 characters, so it cannot add list items or headings. | `summary.TestOneLine`, `summary.TestOneLineCollapsesEveryWhitespace`, `summary.TestLeafBoundsAspectAndLocation` |
| The `claude -p` critic can only read your working tree. Its only tools are Read, Grep and Glob, it loads no MCP server, and Bash, Write, Edit, MultiEdit, NotebookEdit, WebFetch and WebSearch are denied. Your Claude Code settings, a project or a plugin cannot add a tool back: the tool set and the MCP servers are fixed on the command line. | `agent.TestClaudeCriticHasNoWebOrMCPTools`, `agent.TestClaudeCriticDisallowsMutatingTools` |
| `claude.WithProposerReadOnly` denies Bash, Write, Edit, MultiEdit and NotebookEdit to the proposer. | `claude.TestWithProposerReadOnly`, `agent.TestClaudeProposer_ToolArgs` |
| A diff over 4 MiB (`input.MaxDiffBytes`) is refused, by `input.Compute` while it reads `git` and by `Engine.Run` before anything is written, so no critic gets it. | `input.TestComputeRefusesADiffOverTheCap`, `agon.TestRunRefusesADiffOverTheCap` |
| A `claude` process that writes more than 64 MiB to stdout (`MaxOutputBytes`) is stopped, and the call fails. Only the last 64 KiB of its stderr are kept. | `agent.TestExecRefusesOutputOverTheCap`, `agent.TestExecKeepsTheTailOfStderr` |
| The model-call critic sends one request with no tools. | `critic.TestRoundSendsOneToolFreeTurn` |
| Canceling the context stops a `claude` process. | `agent.TestExecCancellation` |
| A run whose `summary.md` or `end.json` cannot be written returns an error, never a result for a session that looks unfinished on disk. | `agon.TestPersistFailureIsReturned` |
| Only the `critic` package calls a model, and no package imports an agent runtime. | `critic.TestOnlyCriticPackageCallsModels`, `critic.TestNoPackageImportsTopos` |

Each test is named as `package.Test`. `agent`, `ledger`, `round` and
`summary` are under `internal/`. The two `critic.TestParse` tests are in
`internal/critic`, and the other `critic` tests are in the public
`critic` package.

## What is not defended

- **Prompt injection.** The diff and the task text go into the critic's
  prompt as they are. A diff can tell the critic to report nothing, and
  the run then reads clean. The proposer reads the critic's comments,
  which quote the diff, so a diff can steer the proposer too.
- **The proposer's tools.** `claude.NewProposer` runs a full Claude Code
  session in your working tree, with your tools and permissions. Without
  `claude.WithProposerReadOnly` it can edit files and run commands.
- **What the critic reads.** The `claude -p` critic can read any file in
  your working tree, and what it reads goes to the model. It cannot send
  a file anywhere else.
- **Concessions.** agon does not check that a conceded attack was fixed.
  A concession is the proposer's word, with the files it changed
  attached.
- **Secrets in the diff.** agon does not scan or redact. Everything in
  the diff reaches the model and the session files. `input.Compute` with
  `To: "."` includes every untracked file that is not ignored, such as a
  `.env` that is missing from `.gitignore`.
- **The session files.** Directories are created with mode 0755 and
  files with mode 0644, so other users of the machine can read the
  diff and the debate. Choose a `StateDir` that only you can read.
- **The environment.** The `claude` processes get your whole environment
  except `ANTHROPIC_API_KEY`, other credentials included.
- **An untrusted checkout.** `input.Compute` runs `git` with the
  repository's own configuration, so a configured external diff or
  textconv driver runs. Do not compute a diff in a checkout you do not
  trust.
- **Size and time.** The task text has no size limit, and the model-call
  critic reads its model's reply without one. The token budget is soft: it is
  checked before each round, a call in flight finishes, and it counts the
  tokens each backend reports. There is no limit on total time. An error
  or a timeout of one call ends the whole run.
- **Windows.** Canceling stops the `claude` process but not the
  processes it started.

## Audit

agon has had no external security audit.

## Supply chain

agon is a Go module and ships as source: there are no binaries and no
images. A release is a version tag on `main` with release notes. The Go
tools check each module download against the
[checksum database](https://sum.golang.org), so a tag cannot change after
it is published. To check what you have:

```sh
go mod verify
govulncheck ./...
```

`govulncheck` is in the quality bar that runs on every push, over agon
and its dependencies.
