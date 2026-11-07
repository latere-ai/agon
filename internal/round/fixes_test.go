// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package round

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon/internal/agent"
	"latere.ai/x/agon/internal/ledger"
	"latere.ai/x/agon/internal/state"
)

func critDoc(round int, body string) string {
	return fmt.Sprintf("# Critic 1 - round %d attacks\n\naspect: security\n\n%s", round, body)
}

func critAttack(id, loc, claim string) string {
	return fmt.Sprintf("## %s [%s]\n\nclaim: %s\n\nexpected violation: panic at runtime\n\nreproduction:\n```\ngo test\n```\n", id, loc, claim)
}

func fixedProposer(response string) *stubProposer {
	r := func(string) (*agent.ProposerResult, error) {
		return &agent.ProposerResult{ForkID: "f", Response: response, Tokens: 1}, nil
	}
	return &stubProposer{first: r, next: r}
}

func newEngine(t *testing.T, rounds []string, response string, maxRounds int) (*Engine, *state.Session) {
	t.Helper()
	sess, err := state.NewSession(t.TempDir(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{
		Sess: sess, Cwd: t.TempDir(), ForkCount: 1,
		Proposer:  fixedProposer(response),
		NewCritic: func(int) agent.Critic { return &stubCritic{rounds: rounds} },
		MaxRounds: maxRounds, CostCap: 1_000_000, TaskContext: "task", DiffPatch: "diff",
	}, sess
}

var pointerPathRE = regexp.MustCompile(`@(\S+\.md)`)

// TestProposerPointerNamesAbsoluteExistingFile pins that the pointer the
// proposer receives names a file that exists from the proposer's own working
// directory, which is the repository and not the session directory.
func TestProposerPointerNamesAbsoluteExistingFile(t *testing.T) {
	r1 := critDoc(1, critAttack("c1-1", "x.go:1", "leaks token"))
	e, _ := newEngine(t, []string{r1}, "rebut c1-1: fine", 2)
	var pointer string
	e.Proposer = &stubProposer{first: func(p string) (*agent.ProposerResult, error) {
		pointer = p
		return &agent.ProposerResult{ForkID: "f", Response: "x"}, nil
	}}
	if _, err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	m := pointerPathRE.FindStringSubmatch(pointer)
	if m == nil {
		t.Fatalf("pointer names no file: %q", pointer)
	}
	if !filepath.IsAbs(m[1]) {
		t.Errorf("pointer path %q is not absolute", m[1])
	}
	b, err := os.ReadFile(m[1])
	if err != nil {
		t.Fatalf("pointer file does not exist: %v", err)
	}
	if !strings.Contains(string(b), "leaks token") {
		t.Errorf("pointer file is not the critic round: %q", b)
	}
}

// TestSingleMalformedCriticReplyDoesNotAbort pins that one reply without the
// header counts as one malformed round and the debate continues.
func TestSingleMalformedCriticReplyDoesNotAbort(t *testing.T) {
	rounds := []string{
		"The diff looks fine to me.",
		critDoc(3, critAttack("c1-1", "x.go:1", "leaks token")),
		critDoc(5, ""),
		critDoc(7, ""),
	}
	e, sess := newEngine(t, rounds, "no change", 8)
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("a single malformed reply must not abort: %v", err)
	}
	if sum.Termination == TermMalformedOutput {
		t.Errorf("termination: got %s", sum.Termination)
	}
	raw, err := os.ReadFile(sess.Path("forks/critic-1/rounds/r1-critic.md"))
	if err != nil {
		t.Fatalf("raw reply not kept: %v", err)
	}
	if !strings.Contains(string(raw), "looks fine to me") {
		t.Errorf("round file does not hold the raw reply: %q", raw)
	}
	if sum.Unresolved != 1 {
		t.Errorf("unresolved: got %d, want 1", sum.Unresolved)
	}
}

// TestTwoMalformedCriticRepliesEndTheFork pins the spec 026 rule.
func TestTwoMalformedCriticRepliesEndTheFork(t *testing.T) {
	e, _ := newEngine(t, []string{"prose one", "prose two", critDoc(5, "")}, "x", 8)
	sum, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("two malformed replies end the run, not fail it: %v", err)
	}
	if sum.Termination != TermMalformedOutput {
		t.Errorf("termination: got %s, want malformed-output", sum.Termination)
	}
	if sum.Forks[0].Rounds != 3 {
		t.Errorf("rounds: got %d, want 3", sum.Forks[0].Rounds)
	}
}

// TestDefenseKeepsAspectAndCriticIndex covers B5 through the full loop.
func TestDefenseKeepsAspectAndCriticIndex(t *testing.T) {
	r1 := critDoc(1, critAttack("c1-1", "a.go:1", "leaks token"))
	e, sess := newEngine(t, []string{r1}, "rebut c1-1: fine", 2)
	if _, err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	agg, err := ledger.Aggregate(sess)
	if err != nil {
		t.Fatal(err)
	}
	if r := agg["c1-1"]; r.Aspect != "security" || r.CriticIndex != 1 {
		t.Errorf("aspect %q critic %d, want security 1", r.Aspect, r.CriticIndex)
	}
}

// TestDefenseForUnknownIDIsIgnored covers B6.
func TestDefenseForUnknownIDIsIgnored(t *testing.T) {
	r1 := critDoc(1, critAttack("c1-1", "a.go:1", "leaks token"))
	e, sess := newEngine(t, []string{r1}, "concede c9-9: fixed\nrebut c2-1: no", 2)
	var log strings.Builder
	e.Progress = &log
	if _, err := e.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	agg, err := ledger.Aggregate(sess)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"c9-9", "c2-1"} {
		if _, ok := agg[id]; ok {
			t.Errorf("phantom record for %s", id)
		}
	}
	if !strings.Contains(log.String(), "c9-9") {
		t.Errorf("ignored defense not reported on progress: %q", log.String())
	}
}
