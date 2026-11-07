// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestExecRefusesOutputOverTheCap: a child that writes more than
// MaxOutputBytes to stdout is stopped and refused with ErrOutputTooLarge, on
// the buffered path and on the line-streaming path, and the result holds no
// more than the cap.
func TestExecRefusesOutputOverTheCap(t *testing.T) {
	script := fmt.Sprintf("yes aaaaaaaaaaaaaaa | head -c %d", MaxOutputBytes+1<<20)
	for _, streaming := range []bool{false, true} {
		r := Run{Bin: "sh", Args: []string{"-c", script}, Cwd: t.TempDir(), Env: CleanEnv(), Deadline: time.Minute}
		if streaming {
			r.OnStdoutLine = func([]byte) {}
		}
		res, err := Exec(context.Background(), r)
		if !errors.Is(err, ErrOutputTooLarge) {
			t.Errorf("streaming=%v: err = %v, want ErrOutputTooLarge", streaming, err)
		}
		if len(res.Stdout) > MaxOutputBytes {
			t.Errorf("streaming=%v: kept %d bytes of stdout", streaming, len(res.Stdout))
		}
		if res.Killed {
			t.Errorf("streaming=%v: reported as a timeout", streaming)
		}
	}
}

// TestExecKeepsTheTailOfStderr: stderr goes into error messages, so only
// its last maxStderrTail bytes are kept.
func TestExecKeepsTheTailOfStderr(t *testing.T) {
	script := fmt.Sprintf("yes bbbbbbbbbbbbbbb | head -c %d >&2; printf END >&2; exit 3", 1<<20)
	res, err := Exec(context.Background(), Run{Bin: "sh", Args: []string{"-c", script}, Cwd: t.TempDir(), Env: CleanEnv(), Deadline: time.Minute})
	if err == nil || res.ExitCode != 3 {
		t.Fatalf("err = %v, exit %d", err, res.ExitCode)
	}
	if len(res.Stderr) > maxStderrTail || !bytes.HasSuffix(res.Stderr, []byte("END")) {
		t.Errorf("stderr is %d bytes, ends %q", len(res.Stderr), res.Stderr[max(0, len(res.Stderr)-8):])
	}
}
