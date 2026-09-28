# agon

Adversarial review of an agent's change: independent critics cross-examine a
diff and a contention score decides the outcome.

```sh
go get latere.ai/x/agon
```

Versions through v0.2.6 of this module path belong to an earlier, unrelated
tree; this engine's releases start above them.

## What it does

After a coding agent produces a change, agon runs one or more critic forks
against the diff. In each fork a critic declares an aspect (security,
correctness, and so on) and attacks the change with concrete claims, each
naming a location, the expected violation and a reproduction. The proposer,
the agent that wrote the change, concedes or rebuts each attack. Rounds
continue until the fork reaches steady state, spends its round budget or
trips the shared token budget.

An attack ledger follows every claim across rounds. The [`Summary`] reports
how many attacks stayed unresolved and headlines the one with the highest
contention, so a reader sees the dispute that survived rather than every
comment a critic made. Every round is written under
`<StateDir>/sessions/<id>/` for audit; the engine invents no default
location.

## Packages

| Package | What it holds |
|---|---|
| `latere.ai/x/agon` | The engine: `Review`, `Engine`, the `Proposer` and `Critic` interfaces, `Summary` |
| `latere.ai/x/agon/claude` | A proposer that forks a Claude Code session (`claude --resume --fork-session`) and a critic that runs `claude -p` |
| `latere.ai/x/agon/critic` | A critic that makes one model call per round through a `luxsdk.Caller` |
| `latere.ai/x/agon/input` | The working-tree diff (`input.Compute`) and Claude Code transcript lookup |

The engine core depends on no model client and no agent runtime. A caller
supplies a `Proposer` and a `CriticFactory`; the subpackages are ready-made
backends, and any type satisfying the interfaces works.

## Wiring a critic

`critic.Config` takes a model connection and a model name. The connection is
any `luxsdk.Caller`: the Lux gateway client, or a provider-direct caller.

```go
// Through the Lux gateway, with LUX_BASE_URL and LUX_API_KEY from the
// environment.
critics := critic.NewCriticFactory(critic.Config{
	Model: luxsdk.New(""),
	Name:  "claude-sonnet-5",
})

// Straight to a provider.
direct, err := luxsdk.NewDirect(luxsdk.ProviderAnthropic, apiKey, "")
if err != nil {
	return err
}
critics = critic.NewCriticFactory(critic.Config{Model: direct, Name: "claude-sonnet-5"})
```

Each round sends the assembled critic prompt, diff included, as one turn with
no tools, and reports the call's token usage, duration and, when the gateway
reports one, its cost. A round's `CriticInput.Model` overrides `Name`.

## Example

Review the uncommitted changes of a working tree, with a Claude Code session
defending them:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"latere.ai/x/agon"
	"latere.ai/x/agon/claude"
	"latere.ai/x/agon/critic"
	"latere.ai/x/agon/input"
	"latere.ai/x/pkg/luxsdk"
)

func main() {
	ctx := context.Background()
	const cwd, sessionID = "/path/to/checkout", "claude-session-id"

	diff, err := input.Compute(ctx, input.DiffSpec{From: "HEAD", To: ".", Cwd: cwd})
	if err != nil {
		log.Fatal(err)
	}
	sum, err := agon.Review(ctx, agon.ReviewOptions{
		StateDir:    "/path/to/state",
		Cwd:         cwd,
		Forks:       2,
		Proposer:    claude.NewProposer(sessionID, cwd),
		NewCritic:   critic.NewCriticFactory(critic.Config{Model: luxsdk.New(""), Name: "claude-sonnet-5"}),
		TaskContext: "add rate limiting to the search handler",
		DiffPatch:   diff.Patch,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d unresolved\n%s\nsee %s\n", sum.Termination, sum.Unresolved, sum.Headline, sum.SessionDir)
}
```

`agon.Engine` is the same machinery with every field exposed, for a caller
that needs per-fork control.

## Development

The whole quality bar runs locally with `go tool lateregate`, the same
command CI runs. Install the hooks once per checkout:

```sh
git config core.hooksPath .githooks
```

## License

Apache-2.0. See [LICENSE](LICENSE).

[`Summary`]: https://pkg.go.dev/latere.ai/x/agon#Summary
