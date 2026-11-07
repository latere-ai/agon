// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// maxScanLine caps the per-line token size for every stdout scanner in
// this package. claude/codex stream-json tool results occasionally
// exceed bufio.Scanner's default 64 KB line limit; 8 MB covers the
// largest observed lines. All scanner sites share this so the streaming
// path, the buffered claude parser, and StreamJSON cannot drift apart
// (a smaller cap there would reject a line the other two accepted).
const maxScanLine = 8 * 1024 * 1024

// MaxOutputBytes is the most a claude or codex process may write to
// stdout in one call: 64 MiB. A call past it is stopped and refused with
// ErrOutputTooLarge. The default JSON output is one result object of a few
// kilobytes; the verbose event stream carries every tool result, and the
// largest of about 16,000 Claude Code turns measured was 30.5 MB.
const MaxOutputBytes = 64 << 20

// ErrOutputTooLarge is returned when a process writes more than
// MaxOutputBytes to stdout.
var ErrOutputTooLarge = fmt.Errorf("the agent wrote more than %d MiB to stdout", MaxOutputBytes>>20)

// maxStderrTail is how much of a process's stderr Exec keeps: the last
// 64 KiB. Stderr only goes into error messages, and its end says why the
// process failed.
const maxStderrTail = 64 << 10

// Run is one subprocess invocation.
//
// When OnStdoutLine is non-nil, Exec switches to a streaming pipe:
// stdout is read line-by-line and each line is forwarded to the
// callback as it arrives. The full stdout is still buffered into
// Result.Stdout for back-compat parsers. This is the verbose streaming
// path (OnStdoutLine): it surfaces tool-use / thinking events live
// while the agent CLI runs.
type Run struct {
	Bin          string
	Args         []string
	Cwd          string
	Env          []string
	Deadline     time.Duration
	OnStdoutLine func([]byte)

	// Stdin, when non-empty, is written to the child's standard input. It is
	// how a prompt too large for one argument reaches the CLI. Empty leaves
	// stdin on /dev/null.
	Stdin string
}

// Result is the outcome of one Exec.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
	Killed   bool
}

// Exec runs r. Cancellation propagates to the child process group;
// SIGINT first, then SIGKILL after a 2s grace.
func Exec(ctx context.Context, r Run) (Result, error) {
	if r.Bin == "" {
		return Result{}, errors.New("agent.Exec: empty Bin")
	}
	if r.Cwd == "" {
		return Result{}, errors.New("agent.Exec: empty Cwd")
	}
	bin := r.Bin
	if abs, err := exec.LookPath(bin); err == nil {
		bin = abs
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if r.Deadline > 0 {
		callCtx, cancel = context.WithTimeout(ctx, r.Deadline)
		defer cancel()
	}

	// runCtx also ends when stdout passes MaxOutputBytes. Killed reports
	// only callCtx, so a refused output is not mistaken for a timeout.
	runCtx, stopRun := context.WithCancel(callCtx)
	defer stopRun()
	cmd := exec.CommandContext(runCtx, bin, r.Args...)
	cmd.Dir = r.Cwd
	cmd.Env = r.Env
	// With no Stdin, os/exec connects /dev/null, so an agent CLI that probes
	// stdin sees immediate EOF and never a TTY.
	if r.Stdin != "" {
		cmd.Stdin = strings.NewReader(r.Stdin)
	}
	stderr := &tailBuffer{limit: maxStderrTail}
	cmd.Stderr = stderr

	setProcessGroup(cmd)
	cmd.Cancel = func() error {
		signalProcessGroup(cmd, true) // SIGINT
		go func() {
			time.Sleep(2 * time.Second)
			signalProcessGroup(cmd, false) // SIGKILL
		}()
		return nil
	}

	start := time.Now()
	var stdoutBytes []byte
	if r.OnStdoutLine == nil {
		stdout := &cappedOutput{limit: MaxOutputBytes, over: stopRun}
		cmd.Stdout = stdout
		err := cmd.Run()
		stdoutBytes = stdout.buf.Bytes()
		res := Result{
			Stdout:   stdoutBytes,
			Stderr:   stderr.Bytes(),
			Duration: time.Since(start),
		}
		if stdout.hit {
			return res, ErrOutputTooLarge
		}
		if err != nil {
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
				res.ExitCode = ee.ExitCode()
			}
			if callCtx.Err() != nil {
				res.Killed = true
			}
			return res, err
		}
		return res, nil
	}

	// Streaming path: pipe stdout, fan each line to the callback while
	// also buffering the full stdout for the buffered parser.
	pipe, perr := cmd.StdoutPipe()
	if perr != nil {
		return Result{Duration: time.Since(start)}, perr
	}
	if err := cmd.Start(); err != nil {
		return Result{Duration: time.Since(start), Stderr: stderr.Bytes()}, err
	}
	var stdoutBuf bytes.Buffer
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), maxScanLine)
	tooLarge := false
	for sc.Scan() {
		line := sc.Bytes()
		if stdoutBuf.Len()+len(line)+1 > MaxOutputBytes {
			tooLarge = true
			stopRun()
			break
		}
		stdoutBuf.Write(line)
		stdoutBuf.WriteByte('\n')
		// The scanner reuses its underlying buffer; copy before
		// handing the bytes to the callback.
		cp := make([]byte, len(line))
		copy(cp, line)
		r.OnStdoutLine(cp)
	}
	scErr := sc.Err()
	if tooLarge || scErr != nil {
		// The scanner stops at a line over maxScanLine while the child may
		// still be writing. Drain the rest so the child is not blocked on a
		// full pipe: Wait returns only after the child exits.
		if _, err := io.Copy(io.Discard, pipe); err != nil {
			scErr = errors.Join(scErr, err)
		}
	}
	waitErr := cmd.Wait()
	stdoutBytes = stdoutBuf.Bytes()
	res := Result{
		Stdout:   stdoutBytes,
		Stderr:   stderr.Bytes(),
		Duration: time.Since(start),
	}
	if tooLarge {
		return res, ErrOutputTooLarge
	}
	if waitErr != nil {
		if ee, ok := errors.AsType[*exec.ExitError](waitErr); ok {
			res.ExitCode = ee.ExitCode()
		}
		if callCtx.Err() != nil {
			res.Killed = true
		}
		return res, waitErr
	}
	if scErr != nil {
		return res, scErr
	}
	return res, nil
}

// CleanEnv returns a copy of os.Environ() with the named keys removed,
// plus the LC_ALL=C stability override.
func CleanEnv(remove ...string) []string {
	excl := map[string]struct{}{
		"ANTHROPIC_API_KEY": {},
	}
	for _, k := range remove {
		excl[k] = struct{}{}
	}
	out := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, drop := excl[k]; drop {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "LC_ALL=C")
	return out
}

// DecodeJSONLine decodes one JSON object that may carry unescaped
// control characters in string fields. Falls back to a sanitizing pass
// if the standard decoder rejects the input.
func DecodeJSONLine(line []byte, dst any) error {
	if err := json.Unmarshal(line, dst, foreignJSON); err == nil {
		return nil
	}
	sanitized := sanitizeControls(line)
	if err := json.Unmarshal(sanitized, dst, foreignJSON); err != nil {
		// Last-resort: log a hex preview of the offending region.
		return fmt.Errorf("decode JSON after sanitize: %w (preview: %x)", err, line[:min(len(line), 256)])
	}
	return nil
}

// StreamJSON reads stdout line-by-line and feeds each successfully
// decoded raw object to visit. Empty lines and undecodable lines are
// skipped silently.
func StreamJSON(stdout io.Reader, visit func(jsontext.Value) error) error {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), maxScanLine)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw jsontext.Value
		if err := DecodeJSONLine(line, &raw); err != nil {
			continue
		}
		if err := visit(raw); err != nil {
			return err
		}
	}
	return sc.Err()
}

func sanitizeControls(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escape := false
	for _, b := range in {
		switch {
		case escape:
			out = append(out, b)
			escape = false
		case b == '\\' && inString:
			out = append(out, b)
			escape = true
		case b == '"':
			out = append(out, b)
			inString = !inString
		case inString && b < 0x20 && b != '\t' && b != '\n' && b != '\r':
			out = append(out, []byte(fmt.Sprintf("\\u%04x", b))...)
		default:
			out = append(out, b)
		}
	}
	return out
}

// cappedOutput keeps at most limit bytes of stdout. The write that would
// pass the limit fails, which ends the copy and closes the pipe, and over
// stops the process. It has no ReadFrom, so io.Copy goes through Write.
type cappedOutput struct {
	buf   bytes.Buffer
	limit int
	over  func()
	hit   bool
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		if !w.hit {
			w.hit = true
			w.over()
		}
		return 0, ErrOutputTooLarge
	}
	return w.buf.Write(p)
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	buf   []byte
	limit int
}

func (w *tailBuffer) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if extra := len(w.buf) - w.limit; extra > 0 {
		w.buf = append(w.buf[:0], w.buf[extra:]...)
	}
	return len(p), nil
}

// Bytes returns what the buffer holds.
func (w *tailBuffer) Bytes() []byte { return w.buf }
