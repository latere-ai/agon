// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"latere.ai/x/agon/input"
)

// TestClaudeProposerRefusesOptionShapedSessionID: a session id that starts
// with "-" would be read by claude as an option, so it is refused before any
// process starts.
func TestClaudeProposerRefusesOptionShapedSessionID(t *testing.T) {
	for _, id := range []string{"--model=x", "-p", "", "a b", "a/b", "../x", "a\nb", "x;y"} {
		marker := filepath.Join(t.TempDir(), "ran")
		bin := filepath.Join(t.TempDir(), "claude")
		script := "#!/bin/sh\ntouch '" + marker + "'\n"
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		p := &ClaudeProposer{Bin: bin, Cwd: t.TempDir(), RootID: id}
		if _, err := p.FirstRound(context.Background(), "ptr"); !errors.Is(err, input.ErrInvalidSessionID) {
			t.Errorf("FirstRound(%q): err = %v, want ErrInvalidSessionID", id, err)
		}
		if _, err := p.NextRound(context.Background(), id, "ptr"); !errors.Is(err, input.ErrInvalidSessionID) {
			t.Errorf("NextRound(%q): err = %v, want ErrInvalidSessionID", id, err)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Errorf("id %q: the process ran", id)
		}
	}
}
