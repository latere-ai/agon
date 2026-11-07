// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agon_test

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"latere.ai/x/agon"
)

// funcCritic adapts a function to agon.Critic.
type funcCritic struct {
	n int
	f func(n int, in agon.CriticInput) (*agon.CriticResult, error)
}

func (c *funcCritic) Round(_ context.Context, in agon.CriticInput) (*agon.CriticResult, error) {
	c.n++
	return c.f(c.n, in)
}

// funcProposer adapts a function to agon.Proposer.
type funcProposer struct {
	f func(pointer string) (*agon.ProposerResult, error)
}

func (p *funcProposer) FirstRound(_ context.Context, pointer string) (*agon.ProposerResult, error) {
	return p.f(pointer)
}

func (p *funcProposer) NextRound(_ context.Context, _, pointer string) (*agon.ProposerResult, error) {
	return p.f(pointer)
}

func atk(id, loc, claim string) string {
	return fmt.Sprintf("## %s [%s]\n\nclaim: %s\n\nexpected violation: it panics\n\nreproduction:\n```\ngo test\n```\n", id, loc, claim)
}

func criticDoc(round int, body string) string {
	return fmt.Sprintf("# Critic 1 - round %d attacks\n\naspect: security\n\n%s", round, body)
}

// oneAttackEngine reports one attack in round 1 and nothing after, with a
// proposer that answers every round with reply.
func oneAttackEngine(t *testing.T, loc, claim, reply string) *agon.Engine {
	t.Helper()
	c := &funcCritic{f: func(n int, _ agon.CriticInput) (*agon.CriticResult, error) {
		if n == 1 {
			return &agon.CriticResult{Markdown: criticDoc(1, atk("c1-1", loc, claim))}, nil
		}
		return &agon.CriticResult{Markdown: criticDoc(2*n-1, "")}, nil
	}}
	return &agon.Engine{
		StateDir: t.TempDir(), Cwd: t.TempDir(), ForkCount: 1, MaxRounds: 6,
		Proposer:    &stubProposer{forkID: "f", reply: reply},
		NewCritic:   func(int) agon.Critic { return c },
		TaskContext: "add login", DiffPatch: "+x := 1",
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSessionDirIsAbsoluteForRelativeStateDir(t *testing.T) {
	t.Chdir(t.TempDir())
	e := oneAttackEngine(t, "a.go:1", "panic", "rebut c1-1: no")
	e.StateDir = "rel-state"
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(sum.SessionDir) {
		t.Errorf("SessionDir %q is not absolute", sum.SessionDir)
	}
}

func TestHugeLocationKeepsAttackUnresolved(t *testing.T) {
	e := oneAttackEngine(t, strings.Repeat("x", 5*1024*1024), "SQL injection", "rebut c1-1: no")
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.Unresolved != 1 || sum.Headline != "SQL injection" {
		t.Errorf("unresolved=%d headline=%q", sum.Unresolved, sum.Headline)
	}
}

func TestSpilledClaimIsTheHeadline(t *testing.T) {
	claim := strings.Repeat("c", 70*1024)
	e := oneAttackEngine(t, "a.go:1", claim, "rebut c1-1: no")
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sum.Headline != claim {
		t.Errorf("headline length %d, want %d", len(sum.Headline), len(claim))
	}
	md, err := os.ReadFile(filepath.Join(sum.SessionDir, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "- [security/a.go:1] ccc") {
		t.Errorf("summary.md headline empty:\n%.400s", md)
	}
}

func TestSummaryKeepsAspectAfterRebuttal(t *testing.T) {
	e := oneAttackEngine(t, "a.go:1", "leaks token", "rebut c1-1: no")
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(sum.SessionDir, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "[security/a.go:1] leaks token") {
		t.Errorf("summary.md:\n%s", md)
	}
}

func TestEndJSONRecordsTheRun(t *testing.T) {
	e := oneAttackEngine(t, "a.go:1", "leaks token", "rebut c1-1: no")
	e.CostCap = 1_000_000
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	end := readJSON(t, filepath.Join(sum.SessionDir, "end.json"))
	if got := end["ended_at"].(string); strings.HasPrefix(got, "0001") {
		t.Errorf("ended_at is the zero time: %s", got)
	}
	if got := end["exit_code"].(float64); got != 1 {
		t.Errorf("exit_code: got %v, want 1 (one unresolved attack)", got)
	}
	stats := end["stats"].(map[string]any)
	if got := stats["cost_cap"].(float64); got != 1_000_000 {
		t.Errorf("cost_cap: got %v", got)
	}
	rounds := stats["rounds_executed_per_fork"].([]any)
	if len(rounds) != 1 || rounds[0].(float64) != float64(sum.Forks[0].Rounds) {
		t.Errorf("rounds_executed_per_fork: got %v, want [%d]", rounds, sum.Forks[0].Rounds)
	}
}

func TestEndJSONExitCodeZeroWhenClean(t *testing.T) {
	c := &funcCritic{f: func(n int, _ agon.CriticInput) (*agon.CriticResult, error) {
		return &agon.CriticResult{Markdown: criticDoc(2*n-1, "")}, nil
	}}
	e := &agon.Engine{
		StateDir: t.TempDir(), Cwd: t.TempDir(), MaxRounds: 6,
		Proposer:  &stubProposer{forkID: "f", reply: "ok"},
		NewCritic: func(int) agon.Critic { return c },
	}
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, filepath.Join(sum.SessionDir, "end.json"))["exit_code"].(float64); got != 0 {
		t.Errorf("exit_code: got %v, want 0", got)
	}
}

var pointerRE = regexp.MustCompile(`@(\S+\.md)`)

func TestPersistFailureIsReturned(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	e := oneAttackEngine(t, "a.go:1", "leaks token", "")
	e.Proposer = &funcProposer{f: func(pointer string) (*agon.ProposerResult, error) {
		m := pointerRE.FindStringSubmatch(pointer)
		if m == nil {
			return nil, fmt.Errorf("no path in pointer %q", pointer)
		}
		// forks/critic-1/rounds/r1-critic.md sits four levels below the root.
		root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(m[1]))))
		if err := os.Chmod(root, 0o500); err != nil {
			return nil, err
		}
		t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
		return &agon.ProposerResult{ForkID: "f", Response: "rebut c1-1: no"}, nil
	}}
	sum, err := e.Run(context.Background())
	if err == nil {
		t.Fatalf("a run whose end.json could not be written must fail, got %+v", sum)
	}
	if !strings.Contains(err.Error(), "summary") {
		t.Errorf("error does not say what failed: %v", err)
	}
}

func TestSessionRecordsStartAndDiff(t *testing.T) {
	e := oneAttackEngine(t, "a.go:1", "leaks token", "rebut c1-1: no")
	e.CostCap = 5000
	e.ForkCount = 1
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	start := readJSON(t, filepath.Join(sum.SessionDir, "start.json"))
	if start["task_context"] != "add login" {
		t.Errorf("task_context: %v", start["task_context"])
	}
	cfg := start["config"].(map[string]any)
	if cfg["side_count"].(float64) != 1 || cfg["max_turn"].(float64) != 6 || cfg["cost_cap"].(float64) != 5000 {
		t.Errorf("config: %v", cfg)
	}
	if start["started_at"].(string) == "" || strings.HasPrefix(start["started_at"].(string), "0001") {
		t.Errorf("started_at: %v", start["started_at"])
	}
	diff, err := os.ReadFile(filepath.Join(sum.SessionDir, "diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if string(diff) != "+x := 1" {
		t.Errorf("diff.patch: %q", diff)
	}
}

func TestNilInputsAreErrors(t *testing.T) {
	okCritic := func(int) agon.Critic {
		return &funcCritic{f: func(n int, _ agon.CriticInput) (*agon.CriticResult, error) {
			return &agon.CriticResult{Markdown: criticDoc(2*n-1, atk("c1-1", "a:1", "panic"))}, nil
		}}
	}
	okProposer := &stubProposer{forkID: "f", reply: "x"}
	cases := map[string]func(sd string) *agon.Engine{
		"nil proposer":  func(sd string) *agon.Engine { return &agon.Engine{StateDir: sd, NewCritic: okCritic} },
		"nil NewCritic": func(sd string) *agon.Engine { return &agon.Engine{StateDir: sd, Proposer: okProposer} },
		"nil critic": func(sd string) *agon.Engine {
			return &agon.Engine{StateDir: sd, Proposer: okProposer, NewCritic: func(int) agon.Critic { return nil }}
		},
		"nil crit result": func(sd string) *agon.Engine {
			return &agon.Engine{StateDir: sd, Proposer: okProposer, NewCritic: func(int) agon.Critic {
				return &funcCritic{f: func(int, agon.CriticInput) (*agon.CriticResult, error) { return nil, nil }}
			}}
		},
		"nil prop result": func(sd string) *agon.Engine {
			return &agon.Engine{StateDir: sd, NewCritic: okCritic, Proposer: &funcProposer{f: func(string) (*agon.ProposerResult, error) { return nil, nil }}}
		},
	}
	cases["nil next-round result"] = func(sd string) *agon.Engine {
		n := 0
		return &agon.Engine{StateDir: sd, NewCritic: okCritic, MaxRounds: 6, Proposer: &funcProposer{f: func(string) (*agon.ProposerResult, error) {
			if n++; n == 1 {
				return &agon.ProposerResult{ForkID: "f", Response: "x"}, nil
			}
			return nil, nil
		}}}
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			sd := t.TempDir()
			sum, err := mk(sd).Run(context.Background())
			if err == nil || sum != nil {
				t.Fatalf("want an error and no summary, got %v %v", sum, err)
			}
			if strings.HasPrefix(name, "nil proposer") || name == "nil NewCritic" {
				if _, statErr := os.Stat(filepath.Join(sd, "sessions")); statErr == nil {
					t.Error("session directory created before the nil check")
				}
			}
		})
	}
}
