// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package input

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestComputeEmptyDiff(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	d, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	if d.ChangedLines != 0 {
		t.Errorf("ChangedLines: got %d, want 0", d.ChangedLines)
	}
}

// TestAnalyzePatchDashPlusContentLines guards against undercounting
// ChangedLines for content lines whose text starts with "-- " or "++ ".
// git emits a removed "-- foo" line as "--- foo" and an added "++ foo"
// line as "+++ foo", both inside the @@ hunk; the old header-shape check
// swallowed them. Also covers a multi-file diff so the per-file hunk
// reset does not miscount the second file's headers as changes.
func TestAnalyzePatchDashPlusContentLines(t *testing.T) {
	patch := "" +
		"diff --git a/dash.go b/dash.go\n" +
		"index 365a263..37ddcab 100644\n" +
		"--- a/dash.go\n" +
		"+++ b/dash.go\n" +
		"@@ -1,2 +1,3 @@\n" +
		" package x\n" +
		"--- a removed dash line\n" +
		"+++ an added plus line\n" +
		"diff --git a/b.go b/b.go\n" +
		"index 111..222 100644\n" +
		"--- a/b.go\n" +
		"+++ b/b.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	changed, files := analyzePatch(patch)
	// dash.go: "--- a removed dash line", "+++ an added plus line";
	// b.go: "-old", "+new" -> 4 changed lines total.
	if changed != 4 {
		t.Errorf("ChangedLines: got %d, want 4", changed)
	}
	if len(files) != 2 {
		t.Errorf("Files: got %v, want 2 entries", files)
	}
}

func TestComputeWithUntracked(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	if d.ChangedLines == 0 {
		t.Errorf("expected non-zero ChangedLines for untracked file; patch:\n%s", d.Patch)
	}
}

// TestComputeUntrackedGitFailurePropagates pins that a genuine git
// failure on an untracked file (here: git cannot read a 0000 file that
// ls-files still reports) propagates instead of being swallowed, which
// would drop the file and undercount ChangedLines, wrongly gating a run
// as a trivial diff.
func TestComputeUntrackedGitFailurePropagates(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file-permission checks")
	}
	dir := t.TempDir()
	gitInit(t, dir)
	bad := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(bad, []byte("secret\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	if _, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir}); err == nil {
		t.Fatal("expected error when git cannot read an untracked file, got nil")
	}
}

func TestTrivialGate(t *testing.T) {
	d := &Diff{ChangedLines: 3}
	if !Trivial(d, 10) {
		t.Error("expected trivial")
	}
	if Trivial(d, 2) {
		t.Error("expected non-trivial")
	}
}

func TestComputeNotARepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir})
	if err == nil {
		t.Fatal("expected ErrNotGitRepo")
	}
}

func TestErrGitErrorMessage(t *testing.T) {
	e := &ErrGit{
		Args:   []string{"diff", "--no-color", "HEAD"},
		Stderr: "  fatal: bad revision\n",
		Err:    errSentinel("inner failure"),
	}
	got := e.Error()
	for _, want := range []string{
		"git diff --no-color HEAD",
		"inner failure",
		"fatal: bad revision",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q; missing %q", got, want)
		}
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestComputeBadRange(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	// Reference a non-existent rev so git exits non-zero. We expect
	// Compute to surface the error wrapped in *ErrGit.
	_, err := Compute(context.Background(), DiffSpec{
		From: "definitely-not-a-real-ref", To: ".", Cwd: dir,
	})
	if err == nil {
		t.Fatal("expected error from bogus ref")
	}
}

// TestComputeRefusesOptionShapedRefs: a ref that starts with "-" would be read
// by git as an option (--output=FILE writes a file), so Compute refuses it and
// git never runs.
func TestComputeRefusesOptionShapedRefs(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	out := filepath.Join(t.TempDir(), "pwned")
	for _, s := range []DiffSpec{
		{From: "--output=" + out, To: ".", Cwd: dir},
		{From: "--output=" + out, To: "HEAD", Cwd: dir},
		{From: "HEAD", To: "--output=" + out, Cwd: dir},
		{From: "-U0", To: ".", Cwd: dir},
	} {
		if _, err := Compute(context.Background(), s); err == nil {
			t.Errorf("Compute(%+v): want an error", s)
		}
		if _, err := os.Stat(out); err == nil {
			t.Fatalf("Compute(%+v): git wrote %s", s, out)
		}
	}
}

// TestComputeUntrackedQuotedNames: git quotes names with non-ASCII bytes or a
// double quote in its plain ls-files output; the diff must still cover them.
func TestComputeUntrackedQuotedNames(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	names := []string{"café.txt", `say "hi".txt`, "new\nline.txt"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("one\ntwo\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	if d.ChangedLines != 2*len(names) {
		t.Errorf("ChangedLines = %d, want %d; patch:\n%s", d.ChangedLines, 2*len(names), d.Patch)
	}
	if !strings.Contains(d.Patch, "caf") || !strings.Contains(d.Patch, "two") {
		t.Errorf("patch lacks the files:\n%s", d.Patch)
	}
}

// TestComputeRevisionRange: a legitimate range still works
// after the refs are fenced off from options.
func TestComputeRevisionRange(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "f.txt"},
		{"-c", "user.email=t@e.com", "-c", "user.name=t", "commit", "-q", "-m", "f"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, b)
		}
	}
	d, err := Compute(context.Background(), DiffSpec{From: "HEAD~1", To: "HEAD", Cwd: dir})
	if err != nil {
		t.Fatal(err)
	}
	if d.ChangedLines != 1 || len(d.Files) != 1 || d.Files[0] != "f.txt" {
		t.Errorf("got %+v", d)
	}
}
