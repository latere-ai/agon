// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package critic_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"latere.ai/x/agon"
	nativecritic "latere.ai/x/agon/critic"
	"latere.ai/x/agon/internal/critic"
	"latere.ai/x/pkg/luxsdk"
)

// cannedR1 is a well-formed round-1 attack block (critic 1, aspect security,
// one attack) shaped like the contract in specs/026-protocol.md, so critic.Parse accepts
// it exactly as it would a claude or codex critic's output.
const cannedR1 = "# Critic 1 - round 1 attacks\n\n" +
	"aspect: security\n\n" +
	"## c1-1 [src/api.py:88]\n\n" +
	"claim: The search handler concatenates user input into a SQL LIKE pattern without escaping.\n\n" +
	"expected violation: An attacker can inject boolean logic via q=%' OR 1=1--.\n\n" +
	"reproduction:\n```\ncurl 'http://localhost:8000/search?q=1'\n```\n"

// modelName is the model id the tests configure.
const modelName = "critic-model"

// scriptedCaller is a luxsdk.Caller that records the request it was sent and
// answers from a fixed response, so a critic round is deterministic and
// network-free.
type scriptedCaller struct {
	resp  luxsdk.Response
	err   error
	delay time.Duration // how long the call takes before it answers
	got   *luxsdk.Request
}

func (s *scriptedCaller) Generate(_ context.Context, req *luxsdk.Request) (*luxsdk.Result, error) {
	s.got = req
	time.Sleep(s.delay)
	if s.err != nil {
		return nil, s.err
	}
	return &luxsdk.Result{Response: s.resp}, nil
}

// Stream is never called: a critic round is one non-streaming call.
func (s *scriptedCaller) Stream(context.Context, *luxsdk.Request) (*luxsdk.Stream, error) {
	return nil, errors.New("scriptedCaller: Stream is not part of a critic round")
}

func textResponse(text string) luxsdk.Response {
	return luxsdk.Response{Blocks: []luxsdk.Block{{Type: luxsdk.BlockText, Text: text}}}
}

// blockingCaller never answers; it blocks until the request context is
// cancelled, then surfaces ctx.Err(). It lets a test observe whether Round's
// per-round deadline actually bounds the model call.
type blockingCaller struct{}

func (blockingCaller) Generate(ctx context.Context, _ *luxsdk.Request) (*luxsdk.Result, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingCaller) Stream(ctx context.Context, _ *luxsdk.Request) (*luxsdk.Stream, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func runOnce(t *testing.T, cfg nativecritic.Config, in agon.CriticInput) *agon.CriticResult {
	t.Helper()
	res, err := nativecritic.NewCriticFactory(cfg)(1).Round(context.Background(), in)
	if err != nil {
		t.Fatalf("Round: %v", err)
	}
	return res
}

func securityInput() agon.CriticInput {
	return agon.CriticInput{
		AspectName:   "security",
		SystemPrompt: "you are a security critic",
		CriticIndex:  1,
		Round:        1,
		TaskContext:  "review the diff",
		DiffPatch:    "--- a/x\n+++ b/x\n@@\n-old\n+new\n",
	}
}

// TestRoundReturnsModelTextVerbatim is the backend-swap contract: the critic
// returns the model's text unchanged as CriticResult.Markdown, and that
// markdown parses into the same Record set (critic.Parse) as the claude/codex
// critics on identical input. securityInput() leaves Deadline at its zero value,
// so this is also the no-cap side of the `if in.Deadline > 0` guard that
// TestRoundHonorsDeadline covers from the other side.
func TestRoundReturnsModelTextVerbatim(t *testing.T) {
	model := &scriptedCaller{resp: textResponse(cannedR1)}
	res := runOnce(t, nativecritic.Config{Model: model, Name: modelName}, securityInput())

	if res.Markdown != cannedR1 {
		t.Fatalf("markdown not verbatim:\n got %q\nwant %q", res.Markdown, cannedR1)
	}
	attacks, _, err := critic.Parse(res.Markdown, "security", 1, 1, nil, critic.ParseOption{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(attacks) != 1 {
		t.Fatalf("attacks: got %d, want 1", len(attacks))
	}
	if attacks[0].AttackID != "c1-1" {
		t.Errorf("attack id: got %q, want c1-1", attacks[0].AttackID)
	}
}

// TestRoundJoinsTextBlocksOnly pins that the attack document is the text the
// model wrote: text blocks are joined in order, and a reasoning block, which
// is the model's working rather than its answer, is left out.
func TestRoundJoinsTextBlocksOnly(t *testing.T) {
	head, tail := cannedR1[:20], cannedR1[20:]
	model := &scriptedCaller{resp: luxsdk.Response{Blocks: []luxsdk.Block{
		{Type: luxsdk.BlockThinking, Text: "first I read the handler"},
		{Type: luxsdk.BlockText, Text: head},
		{Type: luxsdk.BlockText, Text: tail},
	}}}
	res := runOnce(t, nativecritic.Config{Model: model, Name: modelName}, securityInput())
	if res.Markdown != cannedR1 {
		t.Fatalf("markdown:\n got %q\nwant %q", res.Markdown, cannedR1)
	}
}

// TestRoundSendsOneToolFreeTurn pins the request a round makes: the configured
// model, the assembled prompt as the single user turn, no tools, and a bounded
// output.
func TestRoundSendsOneToolFreeTurn(t *testing.T) {
	in := securityInput()
	model := &scriptedCaller{resp: textResponse(cannedR1)}
	runOnce(t, nativecritic.Config{Model: model, Name: modelName}, in)

	req := model.got
	if req == nil {
		t.Fatal("no request sent")
	}
	if req.Model != modelName {
		t.Errorf("model: got %q, want %q", req.Model, modelName)
	}
	if len(req.Tools) != 0 || len(req.ServerTools) != 0 {
		t.Errorf("critic was offered tools: %+v %+v", req.Tools, req.ServerTools)
	}
	if req.MaxTokens == nil || *req.MaxTokens <= 0 {
		t.Errorf("output is unbounded: %v", req.MaxTokens)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != luxsdk.RoleUser {
		t.Fatalf("messages: got %+v, want one user turn", req.Messages)
	}
	blocks := req.Messages[0].Blocks
	if len(blocks) != 1 || blocks[0].Text != agon.AssemblePrompt(in) {
		t.Errorf("user turn is not the assembled prompt: %+v", blocks)
	}
}

// TestRoundModelOverride pins that a round naming its own model wins over the
// configured default.
func TestRoundModelOverride(t *testing.T) {
	const override = "stronger-model"
	in := securityInput()
	in.Model = override
	model := &scriptedCaller{resp: textResponse(cannedR1)}
	runOnce(t, nativecritic.Config{Model: model, Name: modelName}, in)
	if model.got.Model != override {
		t.Errorf("model: got %q, want %q", model.got.Model, override)
	}
}

// TestRoundHonorsDeadline pins the per-round budget: CriticInput.Deadline is a
// time.Duration (a budget from the moment Round is called), so Round applies
// it via context.WithTimeout and a model that never returns is cancelled
// rather than running unbounded. A tiny positive deadline must surface
// context.DeadlineExceeded promptly.
func TestRoundHonorsDeadline(t *testing.T) {
	in := securityInput()
	in.Deadline = 20 * time.Millisecond

	start := time.Now()
	_, err := nativecritic.NewCriticFactory(nativecritic.Config{Model: blockingCaller{}, Name: modelName})(1).
		Round(context.Background(), in)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a deadline error, got nil (round ran unbounded)")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded in chain, got %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("deadline not honored: round blocked %v on a 20ms budget", elapsed)
	}
}

// TestRoundReportsUsage pins that a round carries the model's token counts
// and reported cost, so a critic backed by a model call counts against the
// engine's cost cap. Tokens is input plus output, as the subprocess critics
// count it, and Duration covers the call.
func TestRoundReportsUsage(t *testing.T) {
	const callTime = 5 * time.Millisecond
	cacheRead, cacheWrite, cost := int64(30), int64(10), int64(2500)
	resp := textResponse(cannedR1)
	resp.Usage = luxsdk.Usage{
		InputTokens:           120,
		OutputTokens:          45,
		CacheReadInputTokens:  &cacheRead,
		CacheWriteInputTokens: &cacheWrite,
		CostUSDMicro:          &cost,
	}
	model := &scriptedCaller{resp: resp, delay: callTime}
	res := runOnce(t, nativecritic.Config{Model: model, Name: modelName}, securityInput())

	want := agon.TokenUsage{Input: 120, Output: 45, CacheRead: 30, CacheCreate: 10}
	if res.Usage != want {
		t.Errorf("usage: got %+v, want %+v", res.Usage, want)
	}
	if res.Tokens != want.Input+want.Output {
		t.Errorf("tokens: got %d, want %d", res.Tokens, want.Input+want.Output)
	}
	if res.USD != 0.0025 {
		t.Errorf("usd: got %v, want 0.0025", res.USD)
	}
	if res.Duration < callTime {
		t.Errorf("duration: got %v, want at least the call's %v", res.Duration, callTime)
	}
}

// TestRoundUnreportedUsageIsZero pins the provider-direct case: no cache
// figures and no cost reported read as zero rather than failing the round.
func TestRoundUnreportedUsageIsZero(t *testing.T) {
	resp := textResponse(cannedR1)
	resp.Usage = luxsdk.Usage{InputTokens: 7, OutputTokens: 3}
	res := runOnce(t, nativecritic.Config{Model: &scriptedCaller{resp: resp}, Name: modelName}, securityInput())
	if res.Usage.CacheRead != 0 || res.Usage.CacheCreate != 0 || res.USD != 0 {
		t.Errorf("unreported figures not zero: usage=%+v usd=%v", res.Usage, res.USD)
	}
	if res.Tokens != 10 {
		t.Errorf("tokens: got %d, want 10", res.Tokens)
	}
}

// TestRoundErrors pins the failures a round reports instead of calling a
// model: no connection, no model name, and the call's own error, which stays
// in the chain.
func TestRoundErrors(t *testing.T) {
	if _, err := nativecritic.NewCriticFactory(nativecritic.Config{Name: modelName})(1).
		Round(context.Background(), securityInput()); err == nil {
		t.Error("nil Model: expected an error")
	}

	unnamed := &scriptedCaller{resp: textResponse(cannedR1)}
	if _, err := nativecritic.NewCriticFactory(nativecritic.Config{Model: unnamed})(1).
		Round(context.Background(), securityInput()); err == nil {
		t.Error("no model name: expected an error")
	}
	if unnamed.got != nil {
		t.Error("no model name: a request was sent anyway")
	}

	cause := errors.New("gateway unavailable")
	_, err := nativecritic.NewCriticFactory(nativecritic.Config{Model: &scriptedCaller{err: cause}, Name: modelName})(1).
		Round(context.Background(), securityInput())
	if !errors.Is(err, cause) {
		t.Errorf("call error: got %v, want %v in the chain", err, cause)
	}
}
