// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon"
)

// stubProposer satisfies agon.Proposer.
type stubProposer struct {
	forkID string
	reply  string
}

func (s *stubProposer) FirstRound(_ context.Context, _ string) (*agon.ProposerResult, error) {
	return &agon.ProposerResult{ForkID: s.forkID, Response: s.reply, Duration: time.Millisecond}, nil
}

func (s *stubProposer) NextRound(_ context.Context, forkID, _ string) (*agon.ProposerResult, error) {
	return &agon.ProposerResult{ForkID: forkID, Response: s.reply, Duration: time.Millisecond}, nil
}

// stubCritic satisfies agon.Critic.
type stubCritic struct {
	rounds []string
	idx    int
}

func (s *stubCritic) Round(_ context.Context, _ agon.CriticInput) (*agon.CriticResult, error) {
	if s.idx >= len(s.rounds) {
		return &agon.CriticResult{
			Markdown: "# Critic 1 - round 99 attacks\n\naspect: security\n",
			Duration: time.Millisecond,
		}, nil
	}
	md := s.rounds[s.idx]
	s.idx++
	return &agon.CriticResult{Markdown: md, Duration: time.Millisecond}, nil
}

// TestAssemblePrompt verifies the public helper produces a non-empty
// string containing the expected sections.
func TestAssemblePrompt(t *testing.T) {
	in := agon.CriticInput{
		AspectName:   "security",
		SystemPrompt: "you are a security critic",
		TaskContext:  "add auth",
		DiffPatch:    "+x := 1",
	}
	got := agon.AssemblePrompt(in)
	for _, want := range []string{"security critic", "# Task", "add auth", "# Diff", "+x := 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("AssemblePrompt output missing %q", want)
		}
	}
}

// TestEngineSteadyState runs the public Engine with stub proposer and critic,
// verifying it terminates cleanly and returns a Summary with correct fields.
func TestEngineSteadyState(t *testing.T) {
	// R1 critic: introduces attack c1-1
	r1 := "# Critic 1 - round 1 attacks\n\naspect: security\n\n" +
		"## c1-1 [main.go:5]\n\n" +
		"claim: password logged in plaintext\n\n" +
		"expected violation: log line exposes secret\n\n" +
		"reproduction:\n```\ngo test ./...\n```\n"
	// R3 critic: steady-state (no new attacks)
	r3 := "# Critic 1 - round 3 attacks\n\naspect: security\n"

	critic := &stubCritic{rounds: []string{r1, r3}}
	proposer := &stubProposer{forkID: "fork-abc", reply: "concede c1-1: fixed by hashing"}

	eng := &agon.Engine{
		StateDir:    t.TempDir(),
		Cwd:         t.TempDir(),
		ForkCount:   1,
		Proposer:    proposer,
		NewCritic:   func(_ int) agon.Critic { return critic },
		MaxRounds:   6,
		CostCap:     1_000_000,
		TaskContext: "add user login",
		DiffPatch:   "+password := req.Password",
	}

	ctx := context.Background()
	sum, err := eng.Run(ctx)
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if sum == nil {
		t.Fatal("nil Summary")
	}
	if len(sum.Forks) != 1 {
		t.Errorf("Forks = %d, want 1", len(sum.Forks))
	}
	if sum.SessionDir == "" {
		t.Error("SessionDir is empty")
	}
	// Attack was conceded → unresolved should be 0
	if sum.Unresolved != 0 {
		t.Errorf("Unresolved = %d, want 0", sum.Unresolved)
	}
}

// TestEngineRun_WritesEndJSON verifies Engine.Run persists the session's
// terminal end.json (summary.md too), so embedders can tell a finished run from
// a running one and read its token usage.
func TestEngineRun_WritesEndJSON(t *testing.T) {
	critic := &stubCritic{rounds: []string{"# Critic 1 - round 1 attacks\n\naspect: security\n"}}
	proposer := &stubProposer{forkID: "fork-xyz", reply: "looks fine"}
	eng := &agon.Engine{
		StateDir:    t.TempDir(),
		Cwd:         t.TempDir(),
		ForkCount:   1,
		Proposer:    proposer,
		NewCritic:   func(_ int) agon.Critic { return critic },
		MaxRounds:   2,
		CostCap:     1_000_000,
		TaskContext: "add login",
		DiffPatch:   "+x := 1",
	}
	sum, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	for _, name := range []string{"end.json", "summary.md"} {
		if _, err := os.Stat(filepath.Join(sum.SessionDir, name)); err != nil {
			t.Errorf("expected %s written by the library path: %v", name, err)
		}
	}
}

// TestEngineCostCapZeroMeansUnbounded asserts a zero CostCap is treated as no
// token budget rather than an instantly-exhausted one. Before the guard, the
// meter reported used(0) >= cap(0) and the loop terminated on cost-cap before
// running a single round.
func TestEngineCostCapZeroMeansUnbounded(t *testing.T) {
	critic := &stubCritic{rounds: []string{"# Critic 1 - round 1 attacks\n\naspect: security\n"}}
	eng := &agon.Engine{
		StateDir:    t.TempDir(),
		Cwd:         t.TempDir(),
		ForkCount:   1,
		Proposer:    &stubProposer{forkID: "fork-abc", reply: "ack"},
		NewCritic:   func(_ int) agon.Critic { return critic },
		MaxRounds:   6,
		CostCap:     0,
		TaskContext: "add user login",
		DiffPatch:   "+x := 1",
	}
	sum, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if sum.Termination == "cost-cap" || len(sum.Forks) == 0 || sum.Forks[0].Rounds < 1 {
		rounds := -1
		if len(sum.Forks) > 0 {
			rounds = sum.Forks[0].Rounds
		}
		t.Fatalf("CostCap: 0 fired cost-cap before any work; termination=%s, fork rounds=%d",
			sum.Termination, rounds)
	}
}

// TestEngineMaxRoundsZeroDefaults asserts an unset MaxRounds runs the default
// round budget instead of zero rounds.
func TestEngineMaxRoundsZeroDefaults(t *testing.T) {
	critic := &stubCritic{rounds: []string{"# Critic 1 - round 1 attacks\n\naspect: security\n"}}
	eng := &agon.Engine{
		StateDir:    t.TempDir(),
		Cwd:         t.TempDir(),
		ForkCount:   1,
		Proposer:    &stubProposer{forkID: "fork-abc", reply: "ack"},
		NewCritic:   func(_ int) agon.Critic { return critic },
		CostCap:     1_000_000,
		TaskContext: "add user login",
		DiffPatch:   "+x := 1",
	}
	sum, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if len(sum.Forks) == 0 || sum.Forks[0].Rounds < 1 {
		rounds := -1
		if len(sum.Forks) > 0 {
			rounds = sum.Forks[0].Rounds
		}
		t.Fatalf("MaxRounds: 0 ran no rounds; termination=%s, fork rounds=%d", sum.Termination, rounds)
	}
}

// TestAssemblePromptListsPriorRounds asserts that from round 3 onward the
// prompt points the critic at each prior round file by path, round and role.
func TestAssemblePromptListsPriorRounds(t *testing.T) {
	got := agon.AssemblePrompt(agon.CriticInput{
		SystemPrompt: "you are a security critic",
		TaskContext:  "add auth",
		DiffPatch:    "+x := 1",
		PriorRoundFiles: []agon.RoundFileRef{
			{Path: "forks/critic-1/rounds/r1-critic.md", Round: 1, Role: "critic"},
			{Path: "forks/critic-1/rounds/r2-proposer.md", Round: 2, Role: "proposer"},
		},
	})
	for _, want := range []string{
		"# Prior rounds",
		"- @forks/critic-1/rounds/r1-critic.md - round 1 critic\n",
		"- @forks/critic-1/rounds/r2-proposer.md - round 2 proposer\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AssemblePrompt output missing %q:\n%s", want, got)
		}
	}
}

// failingProposer returns firstErr from FirstRound and nextErr from
// NextRound; a nil error yields a normal reply.
type failingProposer struct {
	firstErr, nextErr error
}

func (p *failingProposer) FirstRound(_ context.Context, _ string) (*agon.ProposerResult, error) {
	if p.firstErr != nil {
		return nil, p.firstErr
	}
	return &agon.ProposerResult{ForkID: "fork-abc", Response: "looks fine"}, nil
}

func (p *failingProposer) NextRound(_ context.Context, forkID, _ string) (*agon.ProposerResult, error) {
	if p.nextErr != nil {
		return nil, p.nextErr
	}
	return &agon.ProposerResult{ForkID: forkID, Response: "looks fine"}, nil
}

// failingCritic returns err from every Round.
type failingCritic struct{ err error }

func (c *failingCritic) Round(_ context.Context, _ agon.CriticInput) (*agon.CriticResult, error) {
	return nil, c.err
}

// openAttack is a round-1 critic document with one attack the stub proposers
// never concede or rebut.
const openAttack = "# Critic 1 - round 1 attacks\n\naspect: security\n\n" +
	"## c1-1 [main.go:5]\n\n" +
	"claim: password logged in plaintext\n\n" +
	"expected violation: log line exposes secret\n\n" +
	"reproduction:\n```\ngo test ./...\n```\n"

// TestEngineRun_StateDirNotADirectory asserts Run reports the session
// folder it could not create rather than running a debate it cannot record.
func TestEngineRun_StateDirNotADirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := &agon.Engine{
		StateDir:  file,
		Cwd:       t.TempDir(),
		Proposer:  &stubProposer{forkID: "fork-abc", reply: "ack"},
		NewCritic: func(_ int) agon.Critic { return &stubCritic{} },
	}
	sum, err := eng.Run(context.Background())
	if err == nil {
		t.Fatalf("Engine.Run with a file as StateDir: got summary %+v, want an error", sum)
	}
}

// TestEngineRun_AgentErrors asserts an error from either side of the debate
// ends Run with that error, for the critic, the proposer's first round, and
// the proposer's later rounds.
func TestEngineRun_AgentErrors(t *testing.T) {
	boom := errors.New("agent exited 1")
	for _, tc := range []struct {
		name     string
		proposer agon.Proposer
		critic   agon.Critic
	}{
		{"critic", &stubProposer{forkID: "fork-abc", reply: "ack"}, &failingCritic{err: boom}},
		{"proposer first round", &failingProposer{firstErr: boom}, &stubCritic{rounds: []string{openAttack}}},
		{"proposer next round", &failingProposer{nextErr: boom}, &stubCritic{rounds: []string{openAttack}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := &agon.Engine{
				StateDir:    t.TempDir(),
				Cwd:         t.TempDir(),
				ForkCount:   1,
				Proposer:    tc.proposer,
				NewCritic:   func(_ int) agon.Critic { return tc.critic },
				MaxRounds:   6,
				TaskContext: "add user login",
				DiffPatch:   "+password := req.Password",
			}
			sum, err := eng.Run(context.Background())
			if !errors.Is(err, boom) {
				t.Fatalf("Engine.Run = (%+v, %v), want an error wrapping %v", sum, err, boom)
			}
			if sum != nil {
				t.Errorf("Summary = %+v, want nil on error", sum)
			}
		})
	}
}

// TestEngineRun_HeadlineNamesOpenAttack asserts an attack the proposer never
// answered is counted as unresolved and its claim becomes the headline.
func TestEngineRun_HeadlineNamesOpenAttack(t *testing.T) {
	eng := &agon.Engine{
		StateDir:    t.TempDir(),
		Cwd:         t.TempDir(),
		ForkCount:   1,
		Proposer:    &stubProposer{forkID: "fork-abc", reply: "looks fine"},
		NewCritic:   func(_ int) agon.Critic { return &stubCritic{rounds: []string{openAttack}} },
		MaxRounds:   2,
		TaskContext: "add user login",
		DiffPatch:   "+password := req.Password",
	}
	sum, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if sum.Unresolved != 1 {
		t.Errorf("Unresolved = %d, want 1", sum.Unresolved)
	}
	if sum.Headline != "password logged in plaintext" {
		t.Errorf("Headline = %q, want the open attack's claim", sum.Headline)
	}
}
