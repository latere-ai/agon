// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"os"
	"strings"
	"testing"
)

// TestAggregateKeepsAttackWithHugeLocation covers B3: a 5 MiB location used
// to push the line past the scanner limit and hide the attack.
func TestAggregateKeepsAttackWithHugeLocation(t *testing.T) {
	s := freshSession(t)
	loc := strings.Repeat("x", 5*1024*1024)
	if err := Append(s, Record{AttackID: "c1-1", CriticIndex: 1, Claim: "sql injection", Location: loc, Status: StatusOpen}); err != nil {
		t.Fatal(err)
	}
	agg, err := Aggregate(s)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := agg["c1-1"]
	if !ok {
		t.Fatal("attack vanished from the ledger")
	}
	if r.Location != loc || r.Claim != "sql injection" {
		t.Errorf("record not restored: loc len %d claim %q", len(r.Location), r.Claim)
	}
	if r.Status != StatusOpen {
		t.Errorf("status %s", r.Status)
	}
}

// TestAggregateReportsOversizedLine pins that a line the scanner cannot read
// is an error and never a silently shorter ledger.
func TestAggregateReportsOversizedLine(t *testing.T) {
	s := freshSession(t)
	line := `{"attack_id":"c1-1","claim":"` + strings.Repeat("y", 5*1024*1024) + `"}` + "\n"
	if err := os.WriteFile(s.Path("attacks.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Aggregate(s); err == nil {
		t.Fatal("expected an error for a line over the scanner limit")
	}
}

// TestAggregateLoadsSpilledClaim covers B4.
func TestAggregateLoadsSpilledClaim(t *testing.T) {
	s := freshSession(t)
	claim := strings.Repeat("c", 70*1024)
	if err := Append(s, Record{AttackID: "c1-1", CriticIndex: 1, Aspect: "security", Claim: claim, Status: StatusOpen}); err != nil {
		t.Fatal(err)
	}
	if err := Append(s, Record{AttackID: "c1-1", RoundLastTouched: 2, Status: StatusRebutted}); err != nil {
		t.Fatal(err)
	}
	agg, err := Aggregate(s)
	if err != nil {
		t.Fatal(err)
	}
	if agg["c1-1"].Claim != claim {
		t.Errorf("claim len %d, want %d", len(agg["c1-1"].Claim), len(claim))
	}
}

// TestAggregateMissingSidecarIsAnError pins that a spilled body that cannot be
// read back is reported.
func TestAggregateMissingSidecarIsAnError(t *testing.T) {
	s := freshSession(t)
	if err := Append(s, Record{AttackID: "c1-1", CriticIndex: 1, Claim: strings.Repeat("c", 70*1024), Status: StatusOpen}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.Path("forks/critic-1/attacks/c1-1.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Aggregate(s); err == nil {
		t.Fatal("expected an error for a missing side-car")
	}
}

// TestFoldKeepsAspectAndCriticIndex covers B5.
func TestFoldKeepsAspectAndCriticIndex(t *testing.T) {
	out := map[string]Record{}
	fold(out, Record{AttackID: "c2-1", CriticIndex: 2, Aspect: "security", Claim: "x", Status: StatusOpen})
	fold(out, Record{AttackID: "c2-1", Status: StatusRebutted, RoundLastTouched: 2})
	r := out["c2-1"]
	if r.Aspect != "security" || r.CriticIndex != 2 {
		t.Errorf("aspect %q critic %d", r.Aspect, r.CriticIndex)
	}
	if r.Status != StatusRebutted {
		t.Errorf("status %s", r.Status)
	}
}
