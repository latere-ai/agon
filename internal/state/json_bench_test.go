// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"latere.ai/x/agon/internal/agent"
)

func jsonBenchSession(b *testing.B) *Session {
	b.Helper()
	s, err := NewSession(b.TempDir(), 2, time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC))
	if err != nil {
		b.Fatal(err)
	}
	return s
}

func jsonBenchStart(files int) *StartFile {
	changed := make([]string, files)
	for i := range changed {
		changed[i] = fmt.Sprintf("internal/server/handlers/resource_%03d.go", i)
	}
	return &StartFile{
		SessionID: "20261105T093000Z-abcd12",
		StartedAt: time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC),
		Proposer:  AgentRef{Agent: "claude", Model: "claude-opus-4-5"},
		Critic:    AgentRef{Agent: "codex", Model: "gpt-5"},
		TaskContext: "Add pagination to the session list endpoint and keep the " +
			"existing cursor format so older clients continue to work.",
		TaskSource: "transcript",
		Diff: DiffSnap{
			From: "origin/main", To: "HEAD", ChangedLines: 40 * files,
			Files: changed, PatchPath: "run.diff",
		},
		Config: ConfigSnap{
			MaxTurn: 6, SideCount: 2, CostCap: 400000, ChangedLinesMin: 10,
			Format: "markdown", MainModel: "claude-opus-4-5", SideModel: "gpt-5",
		},
		RootSession:        RootSession{ID: "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10", Cwd: "/Users/dev/src/service"},
		AdversarialVersion: "v0.9.0",
		GoVersion:          "go1.27.3",
	}
}

func BenchmarkJSONSessionStartFileMarshal(b *testing.B) {
	for _, n := range []int{3, 150} {
		b.Run(fmt.Sprintf("files%d", n), func(b *testing.B) {
			x := jsonBenchStart(n)
			x.Schema = SchemaStart
			doc, err := json.Marshal(x)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			for b.Loop() {
				if _, err := json.Marshal(x); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONSessionEndFileMarshal(b *testing.B) {
	fork, round := 1, 4
	x := &EndFile{
		SessionID:   "20261105T093000Z-abcd12",
		EndedAt:     time.Date(2026, 11, 5, 9, 41, 12, 0, time.UTC),
		Termination: Termination{Reason: "converged", ForkIndex: &fork, Round: &round},
		Stats: Stats{
			TotalAttacks:          17,
			ByStatus:              map[string]int{"conceded": 9, "rebutted": 5, "withdrawn": 2, "unresolved": 1},
			RoundsExecutedPerFork: []int{4, 3},
			TokensUsed:            412345,
			TokenUsage:            &agent.TokenUsage{Input: 12000, Output: 8000, CacheCreate: 40000, CacheRead: 352345},
			CostCap:               500000,
			WallSeconds:           672,
		},
		Headline:    &HeadlineRef{AttackID: "c1-a003", Contention: 7},
		SummaryPath: "summary.md",
		Extras:      map[string]any{"verifier": "ci", "attempt": 1},
	}
	x.Schema = SchemaEnd
	doc, err := json.Marshal(x)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(doc)))
	for b.Loop() {
		if _, err := json.Marshal(x); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONSessionAppendTranscriptMarshal(b *testing.B) {
	s := jsonBenchSession(b)
	r := &TranscriptRecord{
		TS: time.Date(2026, 11, 5, 9, 31, 2, 0, time.UTC), Fork: 1, Round: 2,
		Role: "critic", Path: "forks/critic-1/rounds/r2-critic.md", MS: 48211,
	}
	b.ReportAllocs()
	b.SetBytes(160)
	for b.Loop() {
		if err := AppendTranscript(s, r); err != nil {
			b.Fatal(err)
		}
	}
}
