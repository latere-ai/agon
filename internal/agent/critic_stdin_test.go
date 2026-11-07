// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"latere.ai/x/agon/internal/critic"
)

// stdinRecorder writes a fake claude that saves its stdin and argv under a
// fresh directory and answers with a valid result. It returns the script path
// and the directory.
func stdinRecorder(t *testing.T) (bin, out string) {
	t.Helper()
	out = t.TempDir()
	bin = filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
cat > "$FAKE_OUT/stdin"
for a in "$@"; do printf '%s\n' "$a"; done > "$FAKE_OUT/args"
echo '{"type":"result","subtype":"success","session_id":"s","result":"# Critic 1 - round 1 attacks","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_OUT", out)
	return bin, out
}

// TestClaudeCriticPromptOnStdin: the prompt, diff included, goes to the
// child's stdin byte for byte and no argument carries it, so a diff past the
// OS argument limit (2 MiB on macOS, 128 KiB per argument on Linux) still
// reaches the CLI.
func TestClaudeCriticPromptOnStdin(t *testing.T) {
	for _, tc := range []struct {
		name    string
		diff    string
		verbose bool
	}{
		{"small", "d", false},
		{"small verbose", "d", true},
		{"2MiB", strings.Repeat("+line of a very large diff\n", 2<<20/27+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, out := stdinRecorder(t)
			in := CriticInput{
				Aspect: critic.Lookup("security"), CriticIndex: 1, Round: 1,
				SystemPrompt: "sys", TaskContext: "task", DiffPatch: tc.diff,
				Cwd: t.TempDir(), Deadline: 30 * time.Second, Model: "m",
			}
			c := &ClaudeCritic{Bin: bin, Verbose: tc.verbose, EventOut: &strings.Builder{}}
			if _, err := c.Round(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(out, "stdin"))
			if err != nil {
				t.Fatal(err)
			}
			if want := AssemblePrompt(in); !bytes.Equal(got, []byte(want)) {
				t.Errorf("stdin is %d bytes, want the %d byte prompt", len(got), len(want))
			}
			args, err := os.ReadFile(filepath.Join(out, "args"))
			if err != nil {
				t.Fatal(err)
			}
			if len(args) > 512 || strings.Contains(string(args), "Output rules") || strings.Contains(string(args), "task") {
				t.Errorf("argv carries the prompt (%d bytes): %.200s", len(args), args)
			}
			if !strings.Contains(string(args), "--print\n") {
				t.Errorf("argv lacks --print: %q", args)
			}
		})
	}
}
