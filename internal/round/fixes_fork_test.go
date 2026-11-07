// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package round

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon/internal/agent"

	"latere.ai/x/agon/internal/ledger"
	"latere.ai/x/agon/internal/state"
)

// TestDefenseForOtherForkIsIgnored pins that a fork cannot dispose of an
// attack another fork's critic raised.
func TestDefenseForOtherForkIsIgnored(t *testing.T) {
	sess, err := state.NewSession(t.TempDir(), 2, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Append(sess, ledger.Record{
		AttackID: "c1-1", CriticIndex: 1, Aspect: "security", Claim: "x",
		Status: ledger.StatusOpen, RoundLastTouched: 1,
	}); err != nil {
		t.Fatal(err)
	}
	c, r, ignored, err := updateLedgerFromDefense(sess, 2, "concede c1-1", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if c != 0 || r != 0 || len(ignored) != 1 {
		t.Errorf("conceded=%d rebutted=%d ignored=%v", c, r, ignored)
	}
}

// TestDefenseReportsUnreadableLedger covers the error return of the ledger
// read: a defense must not be silently dropped because the ledger is gone.
func TestDefenseReportsUnreadableLedger(t *testing.T) {
	sess, err := state.NewSession(t.TempDir(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sess.Path("attacks.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := updateLedgerFromDefense(sess, 1, "concede c1-1", nil, 2); err == nil {
		t.Fatal("expected an error")
	}
}

// TestDefenseReportsLedgerWriteFailure covers the error return of the append.
func TestDefenseReportsLedgerWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	sess, err := state.NewSession(t.TempDir(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Append(sess, ledger.Record{
		AttackID: "c1-1", CriticIndex: 1, Aspect: "security", Claim: "x",
		Status: ledger.StatusOpen, RoundLastTouched: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sess.Path("attacks.jsonl"), 0o400); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := updateLedgerFromDefense(sess, 1, "concede c1-1", nil, 2); err == nil {
		t.Fatal("expected an error")
	}
}

// TestRunReportsLedgerWriteFailureAtFinalize covers the unresolved rewrite at
// the end of a run: losing it would leave the ledger saying open.
func TestRunReportsLedgerWriteFailureAtFinalize(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not bind root")
	}
	r1 := critDoc(1, critAttack("c1-1", "a.go:1", "leaks token"))
	e, sess := newEngine(t, []string{r1}, "", 2)
	e.Proposer = &stubProposer{first: func(string) (*agent.ProposerResult, error) {
		if err := os.Chmod(sess.Path("attacks.jsonl"), 0o400); err != nil {
			return nil, err
		}
		return &agent.ProposerResult{ForkID: "f", Response: "no defense"}, nil
	}}
	if _, err := e.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "c1-1") {
		t.Fatalf("expected an error naming the attack, got %v", err)
	}
}
