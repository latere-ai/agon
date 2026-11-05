// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// jsonBenchProse returns deterministic prose of about n bytes.
func jsonBenchProse(n int) string {
	words := []string{
		"the", "handler", "returns", "early", "when", "the", "context", "is",
		"cancelled", "but", "the", "deferred", "close", "still", "runs", "so",
		"the", "second", "caller", "observes", "a", "closed", "channel",
	}
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		b.WriteString(words[i%len(words)])
		b.WriteByte(' ')
	}
	return b.String()[:n]
}

// jsonBenchSizes are the payload sizes (approximate encoded bytes).
var jsonBenchSizes = []struct {
	name  string
	bytes int
}{
	{"small", 512},
	{"medium", 10 * 1024},
	{"large", 200 * 1024},
}

func jsonBenchMarshal(tb testing.TB, v any) []byte {
	tb.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

// jsonBenchClaudeAssistant builds one claude stream-json assistant line.
func jsonBenchClaudeAssistant(tb testing.TB, parts []map[string]any) []byte {
	return jsonBenchMarshal(tb, map[string]any{
		"type":       "assistant",
		"session_id": "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10",
		"message": map[string]any{
			"id":      "msg_01HxExample0000000000000",
			"role":    "assistant",
			"model":   "claude-opus-4-5",
			"content": parts,
			"usage":   map[string]int{"input_tokens": 1200, "output_tokens": 340},
		},
	})
}

// jsonBenchClaudeStream builds a stream-json stdout of about size bytes
// ending with a result event.
func jsonBenchClaudeStream(tb testing.TB, size int) []byte {
	var buf []byte
	buf = append(buf, jsonBenchMarshal(tb, map[string]any{
		"type": "system", "subtype": "init",
		"session_id": "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10",
		"cwd":        "/Users/dev/src/service", "model": "claude-opus-4-5",
		"tools": []string{"Read", "Grep", "Glob", "Bash"},
	})...)
	buf = append(buf, '\n')
	for i := 0; i < 2 || len(buf) < size-1024; i++ {
		var line []byte
		if i%2 == 0 {
			line = jsonBenchClaudeAssistant(tb, []map[string]any{{
				"type": "tool_use", "id": fmt.Sprintf("toolu_%04d", i), "name": "Read",
				"input": map[string]any{"file_path": fmt.Sprintf("/Users/dev/src/service/internal/pkg%d/handler.go", i)},
			}})
		} else {
			line = jsonBenchClaudeAssistant(tb, []map[string]any{{
				"type": "text", "text": jsonBenchProse(600),
			}})
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	buf = append(buf, jsonBenchMarshal(tb, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"session_id":     "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10",
		"result":         jsonBenchProse(800),
		"total_cost_usd": 0.1234,
		"usage": map[string]int{
			"input_tokens": 1200, "output_tokens": 340,
			"cache_creation_input_tokens": 5000, "cache_read_input_tokens": 42000,
		},
	})...)
	buf = append(buf, '\n')
	return buf
}

// jsonBenchCodexStream builds a codex --json stdout of about size bytes.
func jsonBenchCodexStream(tb testing.TB, size int) []byte {
	var buf []byte
	buf = append(buf, `{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53"}`+"\n"...)
	for i := 0; i < 2 || len(buf) < size-512; i++ {
		var v any
		if i%2 == 0 {
			v = map[string]any{"type": "item.started", "item": map[string]any{
				"id": fmt.Sprintf("item_%d", i), "type": "command_execution",
				"command": fmt.Sprintf("rg -n 'func Handle%d' internal/", i), "status": "in_progress",
			}}
		} else {
			v = map[string]any{"type": "item.completed", "item": map[string]any{
				"id": fmt.Sprintf("item_%d", i), "type": "agent_message", "text": jsonBenchProse(700),
			}}
		}
		buf = append(buf, jsonBenchMarshal(tb, v)...)
		buf = append(buf, '\n')
	}
	buf = append(buf, `{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122,"reasoning_output_tokens":64}}`+"\n"...)
	return buf
}

func BenchmarkJSONClaudeStreamResultDecode(b *testing.B) {
	for _, sz := range jsonBenchSizes {
		b.Run(sz.name, func(b *testing.B) {
			stdout := jsonBenchClaudeStream(b, sz.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(len(stdout)))
			for b.Loop() {
				res, err := decodeClaudeStreamResult(stdout)
				if err != nil {
					b.Fatal(err)
				}
				if res.Type != "result" {
					b.Fatalf("type = %q", res.Type)
				}
			}
		})
	}
}

func BenchmarkJSONClaudeResultUnmarshal(b *testing.B) {
	for _, n := range []int{512, 10 * 1024, 200 * 1024} {
		b.Run(fmt.Sprintf("result%dB", n), func(b *testing.B) {
			doc := jsonBenchMarshal(b, map[string]any{
				"type": "result", "subtype": "success", "is_error": false,
				"session_id":     "5b1f0c1e-7d3a-4c1e-9a55-0d6f3f1c2a10",
				"result":         jsonBenchProse(n),
				"total_cost_usd": 0.1234,
				"usage": map[string]int{
					"input_tokens": 1200, "output_tokens": 340,
					"cache_creation_input_tokens": 5000, "cache_read_input_tokens": 42000,
				},
			})
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			for b.Loop() {
				var parsed claudeJSON
				if err := DecodeJSONLine(doc, &parsed); err != nil {
					b.Fatal(err)
				}
				if parsed.Result == "" {
					b.Fatal("empty result")
				}
			}
		})
	}
}

func BenchmarkJSONDecodeJSONLineSanitizeUnmarshal(b *testing.B) {
	// A raw control character inside a string forces the sanitize and
	// retry path.
	line := []byte(`{"type":"result","result":"` + jsonBenchProse(2000) + "\x07bell\x1b[0m" + `","usage":{"input_tokens":10}}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for b.Loop() {
		var parsed claudeJSON
		if err := DecodeJSONLine(line, &parsed); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONClaudeStreamEventFormat(b *testing.B) {
	cases := []struct {
		name string
		line []byte
	}{
		{"toolUse", jsonBenchClaudeAssistant(b, []map[string]any{{
			"type": "tool_use", "id": "toolu_0001", "name": "Grep",
			"input": map[string]any{"pattern": "func (s \\*Session) AtomicWrite", "path": "/Users/dev/src/service/internal"},
		}})},
		{"text", jsonBenchClaudeAssistant(b, []map[string]any{{
			"type": "text", "text": jsonBenchProse(2000),
		}})},
		{"thinking", jsonBenchClaudeAssistant(b, []map[string]any{{
			"type": "thinking", "thinking": jsonBenchProse(4000), "signature": jsonBenchProse(300),
		}})},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if FormatClaudeStreamEvent(c.line) == "" {
				b.Fatal("event not surfaced")
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(c.line)))
			for b.Loop() {
				if FormatClaudeStreamEvent(c.line) == "" {
					b.Fatal("event not surfaced")
				}
			}
		})
	}
}

func BenchmarkJSONCodexStreamEventFormat(b *testing.B) {
	cases := []struct {
		name string
		line []byte
	}{
		{"command", jsonBenchMarshal(b, map[string]any{"type": "item.started", "item": map[string]any{
			"id": "item_1", "type": "command_execution", "command": "rg -n 'func Handle' internal/", "status": "in_progress",
		}})},
		{"toolCall", jsonBenchMarshal(b, map[string]any{"type": "item.started", "item": map[string]any{
			"id": "item_2", "type": "tool_call", "name": "read_file",
			"input": map[string]any{"path": "/Users/dev/src/service/main.go"},
		}})},
		{"agentMessage", jsonBenchMarshal(b, map[string]any{"type": "item.completed", "item": map[string]any{
			"id": "item_3", "type": "agent_message",
			"text": "# Critic 1 - round 1 attacks\n\n## Attack 1\nClaim: " + jsonBenchProse(1500) + "\n\nExpected: " + jsonBenchProse(300),
		}})},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			if FormatCodexStreamEvent(c.line) == "" {
				b.Fatal("event not surfaced")
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(c.line)))
			for b.Loop() {
				if FormatCodexStreamEvent(c.line) == "" {
					b.Fatal("event not surfaced")
				}
			}
		})
	}
}

func BenchmarkJSONStreamJSONDecode(b *testing.B) {
	for _, sz := range jsonBenchSizes {
		b.Run(sz.name, func(b *testing.B) {
			stdout := jsonBenchCodexStream(b, sz.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(len(stdout)))
			for b.Loop() {
				n := 0
				err := StreamJSON(strings.NewReader(string(stdout)), func(json.RawMessage) error {
					n++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
				if n < 4 {
					b.Fatalf("events = %d", n)
				}
			}
		})
	}
}
