// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon/internal/critic"
)

// TestClaudeCriticHasNoWebOrMCPTools: a diff can steer the critic, so the
// critic gets no tool that reaches the network. Its built-in tools are only
// Read, Grep and Glob (--tools, which no settings file widens), it loads no
// MCP server from any settings or plugin (--strict-mcp-config with no
// --mcp-config), and the web tools are denied as well. The values are
// spelled out here, not taken from the constants, so a change to them
// fails this test.
func TestClaudeCriticHasNoWebOrMCPTools(t *testing.T) {
	bin := buildHelper(t, "argv-claude-tools", `package main
import (
	"encoding/json/v2"
	"fmt"
	"os"
	"strings"
)
func main() {
	b, _ := json.Marshal(map[string]any{
		"type": "result", "subtype": "success", "session_id": "s",
		"result": strings.Join(os.Args[1:], "\n"), "is_error": false,
		"usage": map[string]int{"input_tokens": 1, "output_tokens": 2},
	})
	fmt.Println(string(b))
}
`)
	for _, verbose := range []bool{false, true} {
		c := &ClaudeCritic{Bin: bin, Verbose: verbose, EventOut: &strings.Builder{}}
		res, err := c.Round(context.Background(), CriticInput{
			Aspect: critic.Lookup("security"), CriticIndex: 1, Round: 1,
			SystemPrompt: "x", TaskContext: "t", DiffPatch: "d",
			Cwd: t.TempDir(), Deadline: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("verbose=%v: %v", verbose, err)
		}
		argv := strings.Split(res.Markdown, "\n")
		value := func(flag string) string {
			i := slices.Index(argv, flag)
			if i < 0 || i+1 >= len(argv) {
				return ""
			}
			return argv[i+1]
		}
		if got := value("--tools"); got != "Read,Grep,Glob" {
			t.Errorf("verbose=%v: --tools = %q, want Read,Grep,Glob", verbose, got)
		}
		if !slices.Contains(argv, "--strict-mcp-config") {
			t.Errorf("verbose=%v: no --strict-mcp-config, so MCP servers from settings load", verbose)
		}
		if slices.Contains(argv, "--mcp-config") {
			t.Errorf("verbose=%v: --mcp-config loads a server", verbose)
		}
		denied := strings.Split(value("--disallowedTools"), ",")
		for _, tool := range []string{"Bash", "Write", "Edit", "MultiEdit", "NotebookEdit", "WebFetch", "WebSearch"} {
			if !slices.Contains(denied, tool) {
				t.Errorf("verbose=%v: %s is not denied: %v", verbose, tool, denied)
			}
		}
	}
}
