// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package input

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func jsonBenchProse(n int) string {
	words := []string{
		"please", "add", "pagination", "to", "the", "session", "list", "endpoint",
		"and", "keep", "the", "cursor", "format", "stable", "for", "older", "clients",
	}
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		b.WriteString(words[i%len(words)])
		b.WriteByte(' ')
	}
	return b.String()[:n]
}

func jsonBenchLine(tb testing.TB, v any) []byte {
	tb.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

// jsonBenchTranscript builds a claude code session transcript of about
// size bytes: a first user turn followed by assistant and tool records.
func jsonBenchTranscript(tb testing.TB, size int) []byte {
	base := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	var out []byte
	add := func(v any) {
		out = append(out, jsonBenchLine(tb, v)...)
		out = append(out, '\n')
	}
	add(map[string]any{
		"type": "user", "uuid": "u-0001", "sessionId": "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10",
		"cwd": "/Users/dev/src/service", "timestamp": base.Format(time.RFC3339Nano),
		"message": map[string]any{"role": "user", "content": []map[string]any{
			{"type": "text", "text": jsonBenchProse(300)},
		}},
	})
	for i := 1; i < 3 || len(out) < size; i++ {
		ts := base.Add(time.Duration(i) * 3 * time.Second).Format(time.RFC3339Nano)
		switch i % 3 {
		case 0:
			add(map[string]any{
				"type": "assistant", "uuid": fmt.Sprintf("a-%04d", i), "timestamp": ts,
				"message": map[string]any{"role": "assistant", "model": "claude-opus-4-5",
					"content": []map[string]any{{"type": "text", "text": jsonBenchProse(500)}},
					"usage":   map[string]int{"input_tokens": 1200, "output_tokens": 300}},
			})
		case 1:
			add(map[string]any{
				"type": "assistant", "uuid": fmt.Sprintf("a-%04d", i), "timestamp": ts,
				"message": map[string]any{"role": "assistant", "model": "claude-opus-4-5",
					"content": []map[string]any{{"type": "tool_use", "id": fmt.Sprintf("toolu_%04d", i), "name": "Read",
						"input": map[string]any{"file_path": fmt.Sprintf("/Users/dev/src/service/internal/pkg%d/handler.go", i)}}}},
			})
		default:
			add(map[string]any{
				"type": "user", "uuid": fmt.Sprintf("u-%04d", i), "timestamp": ts,
				"message": map[string]any{"role": "user", "content": []map[string]any{
					{"type": "tool_result", "tool_use_id": fmt.Sprintf("toolu_%04d", i-1), "content": jsonBenchProse(900)},
				}},
			})
		}
	}
	return out
}

func BenchmarkJSONReadTranscriptDecode(b *testing.B) {
	for _, sz := range []struct {
		name  string
		bytes int
	}{{"small", 512}, {"medium", 10 * 1024}, {"large", 200 * 1024}} {
		b.Run(sz.name, func(b *testing.B) {
			data := jsonBenchTranscript(b, sz.bytes)
			path := filepath.Join(b.TempDir(), "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10.jsonl")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				t, err := ReadTranscript(path)
				if err != nil {
					b.Fatal(err)
				}
				if t.FirstUser == "" {
					b.Fatal("no first user turn")
				}
			}
		})
	}
}

func BenchmarkJSONExtractFirstUserUnmarshal(b *testing.B) {
	cases := []struct {
		name string
		rec  map[string]any
	}{
		{"stringContent", map[string]any{
			"type": "user", "message": map[string]any{"role": "user", "content": jsonBenchProse(2000)},
		}},
		{"partsContent", map[string]any{
			"type": "user", "message": map[string]any{"role": "user", "content": []map[string]any{
				{"type": "text", "text": jsonBenchProse(2000)},
				{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": map[string]any{"file_path": "/a/b.go"}},
				{"type": "text", "text": jsonBenchProse(2000)},
			}},
		}},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			line := jsonBenchLine(b, c.rec)
			records := [][]byte{line}
			b.ReportAllocs()
			b.SetBytes(int64(len(line)))
			for b.Loop() {
				s, err := ExtractFirstUser(records)
				if err != nil {
					b.Fatal(err)
				}
				if s == "" {
					b.Fatal("empty first user")
				}
			}
		})
	}
}
