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
| The `claude -p` critic cannot run commands or edit files: Bash, Write, Edit, MultiEdit and NotebookEdit are denied. `claude.WithProposerReadOnly` denies the same tools to the proposer. | `agent.TestClaudeCriticDisallowsMutatingTools`, `claude.TestWithProposerReadOnly`, `agent.TestClaudeProposer_ToolArgs` |
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
- **The `claude -p` critic's other tools.** The critic runs in your
  working tree. Tools other than the five denied ones, such as Read,
  WebFetch, WebSearch and MCP tools, follow your Claude Code settings. If
  those settings allow them, a critic steered by a diff can read files
  and send them out.
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
- **Size and time.** The diff, the task text and the output of a
  `claude` process have no size limit. The token budget is soft: it is
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
