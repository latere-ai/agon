// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon/internal/state"
)

func jsonBenchProse(n int) string {
	words := []string{
		"the", "retry", "loop", "never", "checks", "ctx", "done", "so", "a",
		"cancelled", "run", "keeps", "writing", "records", "after", "the",
		"session", "folder", "was", "removed",
	}
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		b.WriteString(words[i%len(words)])
		b.WriteByte(' ')
	}
	return b.String()[:n]
}

func jsonBenchSession(b *testing.B) *state.Session {
	b.Helper()
	s, err := state.NewSession(b.TempDir(), 2, time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC))
	if err != nil {
		b.Fatal(err)
	}
	return s
}

func jsonBenchRecord(i, bodyBytes int) Record {
	round := 1 + i%3
	return Record{
		TS:                time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC).Add(time.Duration(i) * time.Second),
		AttackID:          fmt.Sprintf("c1-a%03d", i),
		CriticIndex:       1 + i%2,
		Aspect:            "error handling",
		RoundIntroduced:   &round,
		Location:          fmt.Sprintf("internal/server/handler_%d.go:%d", i%12, 40+i),
		Claim:             jsonBenchProse(bodyBytes / 2),
		ExpectedViolation: jsonBenchProse(bodyBytes / 4),
		Reproduction:      jsonBenchProse(bodyBytes / 4),
		RoundLastTouched:  round,
		Status:            StatusOpen,
		RoundsSurvived:    i % 3,
		ConcessionFiles:   []string{"internal/server/handler.go"},
		IntroducedIn:      "forks/critic-1/rounds/r1-critic.md",
		LastTouchedIn:     "forks/critic-1/rounds/r2-proposer.md",
	}
}

func BenchmarkJSONLedgerAppend(b *testing.B) {
	for _, sz := range []struct {
		name  string
		bytes int
	}{{"small", 400}, {"medium", 10 * 1024}} {
		b.Run(sz.name, func(b *testing.B) {
			s := jsonBenchSession(b)
			rec := jsonBenchRecord(1, sz.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(sz.bytes))
			for b.Loop() {
				if err := Append(s, rec); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkJSONLedgerSpillDocMarshal covers the side-car body written when a
// record exceeds SpillThreshold.
func BenchmarkJSONLedgerSpillDocMarshal(b *testing.B) {
	body := SpillThreshold + 2*1024
	doc := spillDoc{
		Claim:             jsonBenchProse(body / 2),
		ExpectedViolation: jsonBenchProse(body / 4),
		Reproduction:      jsonBenchProse(body / 4),
	}
	enc, err := json.Marshal(doc, state.StoredJSON)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(enc)))
	for b.Loop() {
		if _, err := json.Marshal(doc, state.StoredJSON); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONLedgerAggregateUnmarshal(b *testing.B) {
	for _, sz := range []struct {
		name    string
		records int
	}{{"records1", 1}, {"records25", 25}, {"records500", 500}} {
		b.Run(sz.name, func(b *testing.B) {
			s := jsonBenchSession(b)
			for i := 0; i < sz.records; i++ {
				// Two transitions per attack, as a run produces.
				r := jsonBenchRecord(i/2, 800)
				if i%2 == 1 {
					r.Status = StatusConceded
				}
				if err := Append(s, r); err != nil {
					b.Fatal(err)
				}
			}
			size := int64(0)
			if st, err := os.Stat(s.Path("attacks.jsonl")); err == nil {
				size = st.Size()
			}
			b.ReportAllocs()
			b.SetBytes(size)
			for b.Loop() {
				m, err := Aggregate(s)
				if err != nil {
					b.Fatal(err)
				}
				if len(m) == 0 {
					b.Fatal("no attacks aggregated")
				}
			}
		})
	}
}

func BenchmarkJSONLedgerLoadBodyUnmarshal(b *testing.B) {
	s := jsonBenchSession(b)
	rec := jsonBenchRecord(3, SpillThreshold+2*1024)
	if err := Append(s, rec); err != nil {
		b.Fatal(err)
	}
	agg, err := Aggregate(s)
	if err != nil {
		b.Fatal(err)
	}
	spilled := agg[rec.AttackID]
	if spilled.BodyPath == "" {
		b.Fatal("record did not spill")
	}
	b.ReportAllocs()
	b.SetBytes(int64(SpillThreshold + 8*1024))
	for b.Loop() {
		r, err := LoadBody(s, spilled)
		if err != nil {
			b.Fatal(err)
		}
		if r.Claim == "" {
			b.Fatal("body not loaded")
		}
	}
}
