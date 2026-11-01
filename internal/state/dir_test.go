// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSessionIDFormat(t *testing.T) {
	id := newSessionID(time.Date(2026, 5, 6, 14, 12, 33, 0, time.UTC))
	re := regexp.MustCompile(`^20260506T141233Z-[a-z0-9]{6}$`)
	if !re.MatchString(id) {
		t.Errorf("session id %q does not match format", id)
	}
}

func TestNewSessionLayout(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		p := filepath.Join(sess.Root, "forks", "critic-"+strconv.Itoa(i), "rounds")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.AtomicWrite("start.json", []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(sess.Path("start.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"x":1}` {
		t.Errorf("got %q", got)
	}
}

func TestAppendLineMultiple(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for range 100 {
		if err := sess.AppendLine("attacks.jsonl", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(sess.Path("attacks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, c := range b {
		if c == '\n' {
			count++
		}
	}
	if count != 100 {
		t.Errorf("lines: got %d, want 100", count)
	}
}

func TestAppendLineRace(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if err := sess.AppendLine("attacks.jsonl", []byte(`{}`)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestAtomicWriteRefusesExistingTemp(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Sanity: same target written twice - second wins (rename overwrites
	// is the OS guarantee). An in-flight temp file cannot collide because
	// atomicfile creates it with a random name.
	if err := sess.AtomicWrite("end.json", []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := sess.AtomicWrite("end.json", []byte("b")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(sess.Path("end.json"))
	if string(got) != "b" {
		t.Errorf("got %q", got)
	}
}

func TestAtomicWriteCreatesNestedDir(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.AtomicWrite("forks/critic-1/rounds/r1.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(sess.Path("forks/critic-1/rounds/r1.md"))
	if string(got) != "x" {
		t.Errorf("got %q", got)
	}
}

func TestAtomicWrite_OpenError_DirIsAFile(t *testing.T) {
	// Trigger MkdirAll's error path: pre-create a regular file at the
	// dir we'd need to make. AtomicWrite then can't create a directory
	// where a file already exists.
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Create a regular file at the path that AtomicWrite would try to
	// turn into a directory.
	conflict := sess.Path("conflict")
	if err := os.WriteFile(conflict, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sess.AtomicWrite("conflict/sub/file", nil); err == nil {
		t.Error("expected MkdirAll to fail when path is a file")
	}
}

func TestAppendLine_DirIsAFile(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	conflict := sess.Path("c")
	if err := os.WriteFile(conflict, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sess.AppendLine("c/sub/file", nil); err == nil {
		t.Error("expected error")
	}
}

func TestWriteForkStats(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(dir, 2, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	stats := map[string]any{"tokens": 1234, "rounds": 3}
	if err := WriteForkStats(sess, 1, stats); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(sess.Path(filepath.Join("forks", "critic-1", "stats.json")))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`"tokens":\s*1234`).Match(b) {
		t.Errorf("stats body missing token field: %s", b)
	}
}

func TestNewSession_StateDirIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if sess, err := NewSession(file, 1, time.Now()); err == nil {
		t.Errorf("NewSession under a regular file: got %+v, want an error", sess)
	}
}

func TestAppendLine_TargetIsADirectory(t *testing.T) {
	sess, err := NewSession(t.TempDir(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// forks/ exists as a directory, so it cannot be opened for append.
	if err := sess.AppendLine("forks", []byte("x")); err == nil {
		t.Error("AppendLine onto a directory: want an error")
	}
}

func TestFsync_MissingDir(t *testing.T) {
	if err := fsync(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("fsync of a missing directory: want an error")
	}
}

func TestWriteForkStats_UnencodableStats(t *testing.T) {
	sess, err := NewSession(t.TempDir(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteForkStats(sess, 1, map[string]any{"bad": make(chan int)}); err == nil {
		t.Error("WriteForkStats with a channel value: want an encoding error")
	}
	if _, err := os.Stat(sess.Path(filepath.Join("forks", "critic-1", "stats.json"))); !os.IsNotExist(err) {
		t.Errorf("stats.json written despite the encoding error: stat err = %v", err)
	}
}
