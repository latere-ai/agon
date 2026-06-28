package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"latere.ai/x/agon/pkg/adversarial/input"
)

func TestPreflightCostCapMin(t *testing.T) {
	f := DefaultFlags()
	f.SessionID = "x"
	f.CostCap = 0
	_, err := Preflight(context.Background(), f)
	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Code != 123 {
		t.Errorf("expected exit 123, got %v", err)
	}
}

func TestPreflightChangedLinesMinNegative(t *testing.T) {
	f := DefaultFlags()
	f.SessionID = "x"
	f.ChangedLinesMin = -1
	_, err := Preflight(context.Background(), f)
	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Code != 124 {
		t.Errorf("expected exit 124, got %v", err)
	}
}

func TestPreflightSideCountMin(t *testing.T) {
	f := DefaultFlags()
	f.SessionID = "x"
	f.SideCount = 0
	_, err := Preflight(context.Background(), f)
	var pe *PreflightError
	if !errors.As(err, &pe) || pe.Code != 121 {
		t.Errorf("expected exit 121, got %v", err)
	}
}

func TestPreflightCrossFamilyOK(t *testing.T) {
	f := DefaultFlags()
	f.SessionID = "x"
	// Default already cross-family (claude vs codex).
	if _, err := Preflight(context.Background(), f); err != nil {
		t.Errorf("cross-family should pass: %v", err)
	}
}

func TestPreflightSameFamilyDifferentModelsOK(t *testing.T) {
	f := DefaultFlags()
	f.SessionID = "x"
	f.Side = "claude"
	f.MainModel = "claude-sonnet"
	f.SideModel = "claude-opus"
	if _, err := Preflight(context.Background(), f); err != nil {
		t.Errorf("same-family different-model should pass: %v", err)
	}
}

func TestPreflightErrorWraps(t *testing.T) {
	pe := &PreflightError{Code: 42, Msg: "boom", Wrap: errors.New("inner")}
	if got := pe.Error(); got != "boom: inner" {
		t.Errorf("got %q", got)
	}
	if pe.Unwrap() == nil {
		t.Error("Unwrap should return inner")
	}
}

func TestPreflightErrorNoWrap(t *testing.T) {
	pe := &PreflightError{Code: 1, Msg: "boom"}
	if got := pe.Error(); got != "boom" {
		t.Errorf("Error() without Wrap: got %q, want \"boom\"", got)
	}
	if pe.Unwrap() != nil {
		t.Error("Unwrap should be nil when Wrap unset")
	}
}

// TestPreflightTranscriptDottedCwdDoesNotFalseFlag pins the bug
// reported on rc2: a repo path containing a dot (e.g.
// /Users/x/dev/changkun.de/agon) was rejected by preflight because
// the lossy decoder mapped both `/` and `.` to `-`, then decoded `-`
// blindly to `/`. Now we compare in encoded space and the dotted
// path matches itself.
func TestPreflightTranscriptDottedCwdDoesNotFalseFlag(t *testing.T) {
	tmp := t.TempDir()
	dotted := filepath.Join(tmp, "host.tld", "repo")
	if err := os.MkdirAll(dotted, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dotted)

	// Synthesize a transcript path that lives under the encoded form
	// of the dotted cwd.
	home := t.TempDir()
	encoded := input.EncodeCwd(dotted)
	transcriptDir := filepath.Join(home, ".claude", "projects", encoded)
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(transcriptDir, "abc.jsonl")
	if err := os.WriteFile(transcript, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	f := DefaultFlags()
	f.Transcript = transcript
	if _, err := Preflight(context.Background(), f); err != nil {
		var pe *PreflightError
		if errors.As(err, &pe) && pe.Code == 101 {
			t.Errorf("preflight false-flagged a dotted-cwd transcript: %v", err)
		}
		// Other errors (e.g. validation paths unrelated to transcript)
		// are not the regression we're guarding against.
	}
}

func TestEncodedSegmentFromTranscript(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"no-projects-segment", "/some/random/file.jsonl", ""},
		{"projects-segment-passthrough", "/Users/x/.claude/projects/-tmp-abc/sess.jsonl", "-tmp-abc"},
		{"with-dotted-segment", "/Users/x/.claude/projects/-Users-y-foo-bar-baz/sess.jsonl", "-Users-y-foo-bar-baz"},
		{"empty", "", ""},

		// Regression for agon c1-1 (2026-05-07): an unrelated
		// "projects" directory in a workspace path must NOT be
		// treated as claude's projects/ marker. Only `.claude/projects/<x>`
		// consecutively counts.
		{"non-claude-projects-dir-ignored", "/tmp/work/projects/notes/session.jsonl", ""},
		{"projects-without-dot-claude-parent", "/var/projects/sub/session.jsonl", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := encodedSegmentFromTranscript(c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Verbatim repro from critic c1-1 (specs/14 attack format):
// /tmp/work/projects/notes/session.jsonl must return "" because
// the helper is documented as matching ~/.claude/projects/<encoded>/
// only.
func TestEncodedSegmentFromTranscriptIgnoresNonClaudeProjectsDir(t *testing.T) {
	got := encodedSegmentFromTranscript("/tmp/work/projects/notes/session.jsonl")
	if got != "" {
		t.Fatalf("got %q, want empty for non-.claude/projects transcript path", got)
	}
}

func TestAgentFamily(t *testing.T) {
	cases := map[string]string{
		"claude":  "claude",
		"codex":   "codex",
		"unknown": "unknown", // unknown family passes through
		"foo":     "foo",
	}
	for in, want := range cases {
		if got := agentFamily(in); got != want {
			t.Errorf("agentFamily(%q) = %q, want %q", in, got, want)
		}
	}
}
