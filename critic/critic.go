// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package critic provides an [agon.Critic] that makes one model call per
// round through a [luxsdk.Caller].
//
// The caller is either the Lux gateway client ([luxsdk.New]) or a
// provider-direct caller ([luxsdk.NewDirect]); the critic sees only the call
// surface both satisfy. Each round sends the assembled critic prompt, which
// already contains the diff, as one user turn with no tools, and returns the
// model's text verbatim as [agon.CriticResult.Markdown]; the engine parses it
// like any other backend's. The response's usage fills the result's token
// counts, so these critics count against the engine's cost cap like the
// subprocess critics do.
//
// Within the module only this package calls a model API, so the engine core
// ([agon]) and every other package stay free of that dependency (enforced by
// boundary_test.go).
package critic

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"latere.ai/x/agon"
	"latere.ai/x/pkg/luxsdk"
)

// Config wires a critic to a model.
type Config struct {
	// Model is the model connection: a [luxsdk.Client] for the gateway or a
	// [luxsdk.Direct] for a provider. Required.
	Model luxsdk.Caller
	// Name is the model id each request names. A round's
	// [agon.CriticInput.Model] overrides it when set.
	Name string
}

// maxOutputTokens caps one round's output. It is the cap the agent loop that
// ran this critic before applied to every turn, kept so a round's output
// budget did not change when the loop went away.
const maxOutputTokens int64 = 4096

// microsPerUSD converts the gateway's cost, reported in millionths of a USD.
const microsPerUSD = 1e6

var (
	errNoModel     = errors.New("critic: Config.Model is required")
	errNoModelName = errors.New("critic: no model name: set Config.Name or CriticInput.Model")
)

// NewCriticFactory returns an [agon.CriticFactory] whose critics make one
// model call per round. The critic holds no state across rounds, so every
// fork shares the same configuration.
func NewCriticFactory(cfg Config) agon.CriticFactory {
	return func(int) agon.Critic {
		return &critic{cfg: cfg}
	}
}

type critic struct {
	cfg Config
}

// Round sends the assembled critic prompt as a single tool-free turn and
// returns the model's text as CriticResult.Markdown, with the call's usage.
func (c *critic) Round(ctx context.Context, in agon.CriticInput) (*agon.CriticResult, error) {
	if c.cfg.Model == nil {
		return nil, errNoModel
	}
	model := cmp.Or(in.Model, c.cfg.Name)
	if model == "" {
		return nil, errNoModelName
	}
	// Match the subprocess critics, which bound each round by in.Deadline
	// (internal/agent.CodexCritic / ClaudeCritic pass it to the subprocess).
	if in.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, in.Deadline)
		defer cancel()
	}
	maxTokens := maxOutputTokens
	start := time.Now()
	res, err := c.cfg.Model.Generate(ctx, &luxsdk.Request{
		Model:     model,
		Messages:  []luxsdk.Message{luxsdk.UserText(agon.AssemblePrompt(in))},
		MaxTokens: &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("critic: generate: %w", err)
	}
	duration := time.Since(start)
	usage := agon.TokenUsage{
		Input:       int(res.Usage.InputTokens),
		Output:      int(res.Usage.OutputTokens),
		CacheCreate: int(deref(res.Usage.CacheWriteInputTokens)),
		CacheRead:   int(deref(res.Usage.CacheReadInputTokens)),
	}
	return &agon.CriticResult{
		Markdown: text(res.Blocks),
		// Input plus output, as the subprocess critics count it; the cache
		// buckets stay in Usage.
		Tokens:   usage.Input + usage.Output,
		Usage:    usage,
		USD:      usd(res.Usage.CostUSDMicro),
		Duration: duration,
	}, nil
}

// text joins the response's text blocks, skipping reasoning and any other
// block kind, which are not part of the attack document.
func text(blocks []luxsdk.Block) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == luxsdk.BlockText {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

// usd converts a reported cost to dollars. A provider-direct caller reports
// none, and CriticResult has no unknown state, so an unreported cost is zero.
func usd(micros *int64) float64 {
	return float64(deref(micros)) / microsPerUSD
}

// deref reads an optional usage figure, with an unreported one as zero.
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
