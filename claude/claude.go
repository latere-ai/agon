// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package claude provides [agon.Proposer] and [agon.Critic]
// implementations backed by the claude CLI.
//
// [NewProposer] uses `claude --resume <sessionID> --fork-session` and is
// therefore only usable when the implementation agent ran under Claude and
// produced a session ID. [NewCritic] invokes `claude -p` (stateless, one
// call per critic round) and can serve as a critic for any task harness.
package claude

import (
	"context"
	"time"

	"latere.ai/x/agon"
	"latere.ai/x/agon/internal/agent"
	"latere.ai/x/agon/internal/critic"
)

// ProposerOption configures a proposer created by [NewProposer].
type ProposerOption func(*agent.ClaudeProposer)

// WithProposerModel overrides the claude model used by the proposer.
func WithProposerModel(model string) ProposerOption {
	return func(p *agent.ClaudeProposer) { p.Model = model }
}

// WithProposerDeadline overrides the per-round call deadline.
func WithProposerDeadline(d time.Duration) ProposerOption {
	return func(p *agent.ClaudeProposer) { p.Deadline = d }
}

// proposerReadOnlyTools are the mutating tools disabled by
// [WithProposerReadOnly]. Bash is included because it can write to the tree.
var proposerReadOnlyTools = []string{"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash"}

// WithProposerReadOnly restricts the proposer to read-only tools (no Write,
// Edit, MultiEdit, NotebookEdit, or Bash). Use it when the proposer runs in a
// working tree that must not be modified — e.g. an embedded verifier whose
// proposer shares the real task worktree. The proposer can still read the code
// to argue and concede; it simply cannot edit it.
func WithProposerReadOnly() ProposerOption {
	return func(p *agent.ClaudeProposer) {
		p.DisallowedTools = append([]string(nil), proposerReadOnlyTools...)
	}
}

// NewProposer returns an [agon.Proposer] that drives the
// implementation-agent clone via `claude --resume <sessionID> --fork-session`.
// sessionID is the claude session ID produced by the implementation run
// (Task.SessionID in wallfacer). cwd is the working directory.
func NewProposer(sessionID, cwd string, opts ...ProposerOption) agon.Proposer {
	p := &agent.ClaudeProposer{
		RootID:   sessionID,
		Cwd:      cwd,
		Deadline: 5 * time.Minute,
	}
	for _, o := range opts {
		o(p)
	}
	return &proposerWrap{p: p}
}

type proposerWrap struct{ p *agent.ClaudeProposer }

func (w *proposerWrap) FirstRound(ctx context.Context, pointer string) (*agon.ProposerResult, error) {
	res, err := w.p.FirstRound(ctx, pointer)
	if err != nil {
		return nil, err
	}
	return fromInternal(res), nil
}

func (w *proposerWrap) NextRound(ctx context.Context, forkID, pointer string) (*agon.ProposerResult, error) {
	res, err := w.p.NextRound(ctx, forkID, pointer)
	if err != nil {
		return nil, err
	}
	return fromInternal(res), nil
}

func fromInternal(r *agent.ProposerResult) *agon.ProposerResult {
	return &agon.ProposerResult{
		ForkID:   r.ForkID,
		Response: r.Response,
		Usage: agon.TokenUsage{
			Input:       r.Usage.Input,
			Output:      r.Usage.Output,
			CacheCreate: r.Usage.CacheCreate,
			CacheRead:   r.Usage.CacheRead,
		},
		USD:      r.USD,
		Duration: r.Duration,
	}
}

// CriticOption configures a critic created by [NewCritic].
type CriticOption func(*agent.ClaudeCritic)

// NewCritic returns an [agon.Critic] that invokes `claude -p`
// (stateless, one-shot per round). It can serve as critic for any task
// harness since it is independent of the implementation session.
func NewCritic(opts ...CriticOption) agon.Critic {
	c := &agent.ClaudeCritic{}
	for _, o := range opts {
		o(c)
	}
	return &criticWrap{c: c}
}

type criticWrap struct{ c *agent.ClaudeCritic }

func (w *criticWrap) Round(ctx context.Context, in agon.CriticInput) (*agon.CriticResult, error) {
	internalIn := toInternalCriticInput(in)
	res, err := w.c.Round(ctx, internalIn)
	if err != nil {
		return nil, err
	}
	return &agon.CriticResult{
		Markdown: res.Markdown,
		Tokens:   res.Tokens,
		Usage: agon.TokenUsage{
			Input:       res.Usage.Input,
			Output:      res.Usage.Output,
			CacheCreate: res.Usage.CacheCreate,
			CacheRead:   res.Usage.CacheRead,
		},
		USD:      res.USD,
		Duration: res.Duration,
	}, nil
}

// toInternalCriticInput converts a public CriticInput to the internal
// agent.CriticInput. The Aspect field is reconstructed from AspectName
// using critic.Lookup so the internal prompt machinery gets the right
// skeleton; the SystemPrompt field already contains the fully assembled
// prompt so the Aspect.SystemPrompt is overridden immediately after.
func toInternalCriticInput(in agon.CriticInput) agent.CriticInput {
	out := agent.CriticInput{
		// Aspect.Name drives ledger bookkeeping; SystemPrompt is the
		// already-assembled prompt from the engine, so we set it directly.
		Aspect:       critic.Aspect{Name: in.AspectName},
		SystemPrompt: in.SystemPrompt,
		CriticIndex:  in.CriticIndex,
		Round:        in.Round,
		TaskContext:  in.TaskContext,
		DiffPatch:    in.DiffPatch,
		Cwd:          in.Cwd,
		Deadline:     in.Deadline,
		Model:        in.Model,
	}
	for _, r := range in.PriorRoundFiles {
		out.PriorRoundFiles = append(out.PriorRoundFiles, agent.RoundFileRef{
			Path: r.Path, Round: r.Round, Role: r.Role,
		})
	}
	return out
}
