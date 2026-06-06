package input

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		if !contains(got, want) {
			t.Errorf("Error() = %q; missing %q", got, want)
		}
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

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
