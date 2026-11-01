// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeProposer_OutputArgs(t *testing.T) {
	p := &ClaudeProposer{}
	if got := p.outputArgs(); strings.Join(got, " ") != "--output-format json" {
		t.Errorf("non-verbose: got %v", got)
	}
	p.Verbose = true
	if got := p.outputArgs(); strings.Join(got, " ") != "--output-format stream-json --verbose" {
		t.Errorf("verbose: got %v", got)
	}
}

func TestClaudeProposer_ToolArgs(t *testing.T) {
	// Default: no tool restriction, no args.
	if got := (&ClaudeProposer{}).toolArgs(); got != nil {
		t.Errorf("default toolArgs = %v, want nil", got)
	}
	// With a denylist: a single comma-joined --disallowedTools arg.
	p := &ClaudeProposer{DisallowedTools: []string{"Write", "Edit", "Bash"}}
	if got := strings.Join(p.toolArgs(), " "); got != "--disallowedTools Write,Edit,Bash" {
		t.Errorf("toolArgs = %q", got)
	}
}

func TestClaudeProposer_NextRound_HappyPath(t *testing.T) {
	if _, err := os.Stat("/usr/bin/env"); err != nil {
		t.Skip("posix env required")
	}

	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
cat <<'EOF'
{"type":"result","subtype":"success","session_id":"fork-1","result":"hi","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}
EOF
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	res, err := p.NextRound(context.Background(), "fork-1", "@some-pointer")
	if err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if res.Response != "hi" {
		t.Errorf("Response: got %q", res.Response)
	}
	if res.ForkID != "fork-1" {
		t.Errorf("ForkID: got %q", res.ForkID)
	}
}

func TestClaudeProposer_NextRound_UnexpectedFork(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
cat <<'EOF'
{"type":"result","subtype":"success","session_id":"OTHER","result":"hi","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}
EOF
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrUnexpectedFork) {
		t.Errorf("got %v, want ErrUnexpectedFork", err)
	}
}

func TestClaudeProposer_NextRound_AgentError(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
cat <<'EOF'
{"type":"result","subtype":"error","session_id":"fork-1","result":"boom","is_error":true,"usage":{"input_tokens":1,"output_tokens":1}}
EOF
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrAgentError) {
		t.Errorf("got %v, want ErrAgentError", err)
	}
}

func TestClaudeProposer_NextRound_EmptyResult(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
cat <<'EOF'
{"type":"result","subtype":"success","session_id":"fork-1","result":"","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}
EOF
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrEmptyResult) {
		t.Errorf("got %v, want ErrEmptyResult", err)
	}
}

func TestClaudeProposer_NextRound_AuthError(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
echo "Authentication error" >&2
exit 1
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrAuth) {
		t.Errorf("got %v, want ErrAuth", err)
	}
}

func TestClaudeProposer_NextRound_CwdMismatch(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
echo "No conversation found with session ID 'fork-1'" >&2
exit 1
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrCwdMismatch) {
		t.Errorf("got %v, want ErrCwdMismatch", err)
	}
}

func TestClaudeProposer_NextRound_BadJSON(t *testing.T) {
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "claude")
	body := `#!/usr/bin/env bash
echo "not-json"
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrJSON) {
		t.Errorf("got %v, want ErrJSON", err)
	}
}

// writeStub writes an executable shell script named name into a fresh
// directory and returns its path.
func writeStub(t *testing.T, name, body string) string {
	t.Helper()
	stub := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

// argvEchoClaude is a fake claude CLI that answers with a successful result
// whose text is its own argv, one argument per line, so a test can assert the
// flags a driver passed.
const argvEchoClaude = `package main
import (
	"encoding/json"
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
`

func TestDecodeClaudeStreamResult_SkipsNoise(t *testing.T) {
	stdout := strings.Join([]string{
		``,
		`not json`,
		`{"type":"assistant","message":{"content":[]}}`,
		`{"type":"result","usage":"not an object"}`,
		`{"type":"result","session_id":"s1","result":"first","usage":{"input_tokens":1}}`,
		`{"type":"result","session_id":"s2","result":"last","usage":{"input_tokens":3,"output_tokens":4}}`,
		``,
	}, "\n")
	got, err := decodeClaudeStreamResult([]byte(stdout))
	if err != nil {
		t.Fatalf("decodeClaudeStreamResult: %v", err)
	}
	if got.SessionID != "s2" || got.Result != "last" {
		t.Errorf("got session %q result %q, want the last result event (s2, last)", got.SessionID, got.Result)
	}
	if u := got.usage(); u.Input != 3 || u.Output != 4 {
		t.Errorf("usage = %+v, want input 3 output 4", u)
	}
}

func TestDecodeClaudeStreamResult_NoResultEvent(t *testing.T) {
	_, err := decodeClaudeStreamResult([]byte("{\"type\":\"system\"}\n{\"type\":\"result\",\"usage\":7}\n"))
	if err == nil || !strings.Contains(err.Error(), "no result event") {
		t.Errorf("got %v, want a no-result-event error", err)
	}
}

func TestClaudeProposer_ModelFlag(t *testing.T) {
	bin := buildHelper(t, "argv-claude", argvEchoClaude)
	p := &ClaudeProposer{Bin: bin, Cwd: t.TempDir(), RootID: "root-1", Model: "model-x"}

	first, err := p.FirstRound(context.Background(), "@pointer")
	if err != nil {
		t.Fatalf("FirstRound: %v", err)
	}
	for _, want := range []string{"--resume\nroot-1\n--fork-session", "--model\nmodel-x"} {
		if !strings.Contains(first.Response, want) {
			t.Errorf("FirstRound argv lacks %q:\n%s", want, first.Response)
		}
	}

	next, err := p.NextRound(context.Background(), "s", "@pointer")
	if err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if !strings.Contains(next.Response, "--model\nmodel-x") {
		t.Errorf("NextRound argv lacks the model flag:\n%s", next.Response)
	}
	if strings.Contains(next.Response, "--fork-session") {
		t.Errorf("NextRound forked again:\n%s", next.Response)
	}
}

// TestClaudeProposer_DefaultBin asserts an empty Bin runs the claude found on
// PATH. PATH holds only the fake, so no real claude can be reached.
func TestClaudeProposer_DefaultBin(t *testing.T) {
	bin := buildHelper(t, "claude", argvEchoClaude)
	t.Setenv("PATH", filepath.Dir(bin))
	p := &ClaudeProposer{Cwd: t.TempDir(), RootID: "root-1"}
	res, err := p.FirstRound(context.Background(), "@pointer")
	if err != nil {
		t.Fatalf("FirstRound: %v", err)
	}
	if res.ForkID != "s" || !strings.Contains(res.Response, "--resume\nroot-1") {
		t.Errorf("got fork %q response %q, want the fake claude's reply", res.ForkID, res.Response)
	}
}

// TestClaudeProposer_VerboseStreamsEvents drives the stream-json path: tool
// events reach EventOut while the call runs, and the final result event
// becomes the ProposerResult.
func TestClaudeProposer_VerboseStreamsEvents(t *testing.T) {
	stub := writeStub(t, "claude", `#!/usr/bin/env bash
cat <<'EOF'
{"type":"system","subtype":"init","session_id":"fork-1"}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"u1","name":"Read","input":{"file_path":"/repo/x.go"}}]}}
{"type":"result","subtype":"success","session_id":"fork-1","result":"concede c1-1","is_error":false,"total_cost_usd":0.5,"usage":{"input_tokens":10,"output_tokens":5}}
EOF
`)
	var events strings.Builder
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir(), Verbose: true, EventOut: &events}
	res, err := p.NextRound(context.Background(), "fork-1", "@p")
	if err != nil {
		t.Fatalf("NextRound: %v", err)
	}
	if !strings.Contains(events.String(), "  → Read: /repo/x.go") {
		t.Errorf("EventOut missing the tool event:\n%s", events.String())
	}
	if res.Response != "concede c1-1" || res.Tokens != 15 || res.USD != 0.5 {
		t.Errorf("result = %+v, want the final result event", res)
	}
}

func TestClaudeProposer_Timeout(t *testing.T) {
	stub := writeStub(t, "claude", "#!/usr/bin/env bash\nexec sleep 5\n")
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir(), Deadline: 200 * time.Millisecond}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("got %v, want ErrTimeout", err)
	}
}

// TestClaudeProposer_ExecErrorKeepsStderr asserts a failure that is neither
// auth nor a missing session is reported with the CLI's stderr.
func TestClaudeProposer_ExecErrorKeepsStderr(t *testing.T) {
	stub := writeStub(t, "claude", "#!/usr/bin/env bash\necho 'disk quota exceeded' >&2\nexit 3\n")
	p := &ClaudeProposer{Bin: stub, Cwd: t.TempDir()}
	_, err := p.NextRound(context.Background(), "fork-1", "@p")
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrAuth) || errors.Is(err, ErrCwdMismatch) || errors.Is(err, ErrTimeout) {
		t.Errorf("got %v, want a plain exec error", err)
	}
	if !strings.Contains(err.Error(), "disk quota exceeded") {
		t.Errorf("error lacks the CLI's stderr: %v", err)
	}
}
