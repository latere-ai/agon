// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExecEmptyBin(t *testing.T) {
	_, err := Exec(context.Background(), Run{Bin: "", Cwd: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "empty Bin") {
		t.Errorf("got %v", err)
	}
}

func TestExecEmptyCwd(t *testing.T) {
	_, err := Exec(context.Background(), Run{Bin: "/bin/echo"})
	if err == nil || !strings.Contains(err.Error(), "empty Cwd") {
		t.Errorf("got %v", err)
	}
}

func TestExecNonZeroExit(t *testing.T) {
	res, err := Exec(context.Background(), Run{
		Bin: "sh", Args: []string{"-c", "exit 7"},
		Cwd: t.TempDir(), Env: CleanEnv(),
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if res.ExitCode != 7 {
		t.Errorf("ExitCode: got %d, want 7", res.ExitCode)
	}
}

// TestStreamJSONLargeLine guards the scanner buffer cap: a single JSON
// line between the old 1 MB limit and the 8 MB limit used by the
// streaming Exec path and the buffered claude parser must parse, not
// error. Before the caps were unified StreamJSON returned
// bufio.ErrTooLong here, failing an otherwise-valid codex critic round.
func TestStreamJSONLargeLine(t *testing.T) {
	// ~2 MB of payload, comfortably over the old 1 MB cap and under 8 MB.
	big := strings.Repeat("x", 2*1024*1024)
	line := `{"text":"` + big + `"}` + "\n"
	in := strings.NewReader(line)
	var got int
	if err := StreamJSON(in, func(json.RawMessage) error {
		got++
		return nil
	}); err != nil {
		t.Fatalf("StreamJSON on a 2 MB line: %v", err)
	}
	if got != 1 {
		t.Errorf("visited %d lines, want 1", got)
	}
}

func TestDecodeJSONLineFailureIncludesPreview(t *testing.T) {
	var dst map[string]string
	err := DecodeJSONLine([]byte(`not json at all`), &dst)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "preview") {
		t.Errorf("err missing preview: %v", err)
	}
}

// TestExecStreamingOversizeLineFailsFast guards the streaming path when one
// stdout line exceeds maxScanLine. The scanner stops there; if nothing drains
// the rest of the pipe, the child blocks on a full pipe, Wait never returns,
// and the call ends only at its deadline as a timeout (or never, with none).
func TestExecStreamingOversizeLineFailsFast(t *testing.T) {
	bin := buildHelper(t, "oversize-line", `package main
import (
	"os"
	"strings"
)
func main() {
	os.Stdout.WriteString(strings.Repeat("x", 9*1024*1024) + "\n")
	os.Stdout.WriteString("{\"type\":\"result\"}\n")
}
`)
	res, err := Exec(context.Background(), Run{
		Bin: bin, Cwd: t.TempDir(), Env: CleanEnv(), Deadline: 10 * time.Second,
		OnStdoutLine: func([]byte) {},
	})
	if res.Killed {
		t.Fatalf("Exec hung until its deadline (err=%v); want the scanner error at once", err)
	}
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Errorf("Exec err = %v, want bufio.ErrTooLong", err)
	}
}
