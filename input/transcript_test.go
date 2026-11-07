// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package input

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	// Note: claude's encoding (literal '/' -> '-') is lossy when the
	// cwd contains '-'; we mirror that and only round-trip cases
	// without ambiguity.
	cases := []string{
		"/Users/ada/dev/foo",
		"/srv/something/x",
		"/",
	}
	for _, c := range cases {
		got := DecodeCwd(EncodeCwd(c))
		if got != c {
			t.Errorf("Encode/Decode round-trip: got %q, want %q", got, c)
		}
	}
}

func TestExtractFirstUserStringForm(t *testing.T) {
	rec := []byte(`{"type":"user","message":{"role":"user","content":"do the task"}}`)
	got, err := ExtractFirstUser([][]byte{rec})
	if err != nil {
		t.Fatal(err)
	}
	if got != "do the task" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFirstUserArrayForm(t *testing.T) {
	rec := []byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}}`)
	got, err := ExtractFirstUser([][]byte{rec})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\n\nb" {
		t.Errorf("got %q", got)
	}
}

func TestReadTranscriptMissing(t *testing.T) {
	_, err := ReadTranscript(filepath.Join(t.TempDir(), "missing.jsonl"))
	if err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestReadTranscriptNoUserTurn(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "abc.jsonl")
	if err := os.WriteFile(p, []byte(`{"type":"system","message":{"role":"system","content":"x"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadTranscript(p)
	if !errors.Is(err, ErrNoUserTurn) {
		t.Errorf("got %v, want ErrNoUserTurn", err)
	}
}

// TestReadTranscriptAllMalformed pins that a transcript whose every
// non-empty line is unparseable reports ErrTranscriptMalformed, not
// ErrNoUserTurn. Before the fix the >5% bad-line check was gated on
// LineCount>0, so an all-bad file (LineCount==0) skipped it entirely.
func TestReadTranscriptAllMalformed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "abc.jsonl")
	if err := os.WriteFile(p, []byte("not json\n{also not json\n<garbage>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadTranscript(p)
	if !errors.Is(err, ErrTranscriptMalformed) {
		t.Errorf("got %v, want ErrTranscriptMalformed", err)
	}
}

func TestReadTranscriptHappy(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "abc.jsonl")
	body := `{"type":"system","message":{"role":"system","content":"sys"}}` + "\n" +
		`{"type":"user","message":{"role":"user","content":"do the task"},"timestamp":"2026-05-06T14:12:33Z"}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":"ok"}}` + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := ReadTranscript(p)
	if err != nil {
		t.Fatal(err)
	}
	if tr.FirstUser != "do the task" {
		t.Errorf("FirstUser: got %q", tr.FirstUser)
	}
	if tr.SessionID != "abc" {
		t.Errorf("SessionID: got %q", tr.SessionID)
	}
}

func TestLocateTranscriptExplicit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jsonl")
	if err := os.WriteFile(p, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LocateTranscript("", "", "", p)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Errorf("got %q", got)
	}
}

func TestLocateTranscriptExplicitMissing(t *testing.T) {
	_, err := LocateTranscript("", "", "", "/nope/does-not-exist.jsonl")
	if err == nil {
		t.Fatal("want error for missing explicit path")
	}
	if !errors.Is(err, ErrTranscriptNotFound) {
		t.Errorf("want ErrTranscriptNotFound, got %v", err)
	}
}

func TestLocateTranscriptMissingArgs(t *testing.T) {
	_, err := LocateTranscript("", "", "", "")
	if err == nil {
		t.Fatal("want error when all args are empty")
	}
	if !errors.Is(err, ErrTranscriptNotFound) {
		t.Errorf("want ErrTranscriptNotFound, got %v", err)
	}
}

func TestLocateTranscriptByEncodedCwd(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/x/work"
	encoded := EncodeCwd(cwd)
	dir := filepath.Join(home, ".claude", "projects", encoded)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	id := "session-id-123"
	want := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(want, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LocateTranscript(home, cwd, id, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindSession(t *testing.T) {
	home := t.TempDir()
	dir1 := filepath.Join(home, ".claude", "projects", "-Users-a-x")
	dir2 := filepath.Join(home, ".claude", "projects", "-Users-b-y")
	if err := os.MkdirAll(dir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir2, "abc.jsonl")
	if err := os.WriteFile(want, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, seg, err := FindSession(home, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("path: got %q, want %q", got, want)
	}
	if seg != "-Users-b-y" {
		t.Errorf("segment: got %q", seg)
	}

	_, _, err = FindSession(home, "nope")
	if err == nil {
		t.Error("expected error for missing session")
	}
}

func TestFindSessionMissingArgs(t *testing.T) {
	_, _, err := FindSession("", "")
	if err == nil {
		t.Error("expected error for empty home/sessionID")
	}
}

// TestSessionIDRule pins the shape a session id may have.
func TestSessionIDRule(t *testing.T) {
	for _, ok := range []string{"abc", "session-id-123", "0b1c2d3e-4f50-6789-abcd-ef0123456789", "a_b", "_x"} {
		if err := ValidateSessionID(ok); err != nil {
			t.Errorf("ValidateSessionID(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-x", "--model=x", "../../x", "a/b", `a\b`, "a.b", "a b", "a\x00b", "a\nb", strings.Repeat("a", 129)} {
		if err := ValidateSessionID(bad); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("ValidateSessionID(%q) = %v, want ErrInvalidSessionID", bad, err)
		}
	}
}

// TestSessionLookupRefusesTraversal: an id with "../" must not reach a file
// outside ~/.claude/projects.
func TestSessionLookupRefusesTraversal(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/x/work"
	dir := filepath.Join(home, ".claude", "projects", EncodeCwd(cwd))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file the traversal would reach: <home>/x.jsonl.
	if err := os.WriteFile(filepath.Join(home, "x.jsonl"), []byte(`{"type":"user","message":{"content":"secret"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../../../x", "../../x", "-x", "a/b"} {
		p, err := LocateTranscript(home, cwd, id, "")
		if !errors.Is(err, ErrInvalidSessionID) || p != "" {
			t.Errorf("LocateTranscript(%q) = %q, %v", id, p, err)
		}
		p, _, err = FindSession(home, id)
		if !errors.Is(err, ErrInvalidSessionID) || p != "" {
			t.Errorf("FindSession(%q) = %q, %v", id, p, err)
		}
	}
}
