// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"slices"
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

func TestCleanEnvDropsNamedKeys(t *testing.T) {
	t.Setenv("AGON_TEST_DROP", "1")
	t.Setenv("AGON_TEST_KEEP", "1")
	env := CleanEnv("AGON_TEST_DROP")
	if slices.Contains(env, "AGON_TEST_DROP=1") {
		t.Error("named key not dropped")
	}
	if !slices.Contains(env, "AGON_TEST_KEEP=1") {
		t.Error("unnamed key dropped")
	}
}

// TestExecStreamingDeliversLinesAndExitCode: with OnStdoutLine set, each
// line reaches the callback as it is written, the full stdout is still
// buffered, and a non-zero exit is reported with its code.
func TestExecStreamingDeliversLinesAndExitCode(t *testing.T) {
	var lines []string
	res, err := Exec(context.Background(), Run{
		Bin: "sh", Args: []string{"-c", "echo one; echo two; exit 7"},
		Cwd: t.TempDir(), Env: CleanEnv(), Deadline: 5 * time.Second,
		OnStdoutLine: func(b []byte) { lines = append(lines, string(b)) },
	})
	if err == nil {
		t.Fatal("expected an error for a non-zero exit")
	}
	if res.ExitCode != 7 || res.Killed {
		t.Errorf("ExitCode = %d, Killed = %v; want 7, false", res.ExitCode, res.Killed)
	}
	if strings.Join(lines, ",") != "one,two" {
		t.Errorf("callback lines = %q, want [one two]", lines)
	}
	if string(res.Stdout) != "one\ntwo\n" {
		t.Errorf("Stdout = %q", res.Stdout)
	}
}

func TestExecStreamingDeadline(t *testing.T) {
	res, err := Exec(context.Background(), Run{
		Bin: "sh", Args: []string{"-c", "exec sleep 5"},
		Cwd: t.TempDir(), Env: CleanEnv(), Deadline: 200 * time.Millisecond,
		OnStdoutLine: func([]byte) {},
	})
	if err == nil {
		t.Fatal("expected an error past the deadline")
	}
	if !res.Killed {
		t.Error("expected Killed=true")
	}
}

func TestExecStreamingMissingBinary(t *testing.T) {
	_, err := Exec(context.Background(), Run{
		Bin: "/no/such/binary-z9z", Cwd: t.TempDir(), Env: CleanEnv(),
		OnStdoutLine: func([]byte) { t.Error("callback ran for a binary that never started") },
	})
	if err == nil {
		t.Fatal("expected a start error")
	}
}

func TestStreamJSONStopsOnVisitError(t *testing.T) {
	stop := errors.New("stop")
	var seen int
	err := StreamJSON(strings.NewReader("{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n"), func(json.RawMessage) error {
		seen++
		if seen == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Errorf("StreamJSON err = %v, want the visit error", err)
	}
	if seen != 2 {
		t.Errorf("visited %d objects, want 2", seen)
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
