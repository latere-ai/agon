// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"latere.ai/x/agon/input"
)

// TestNewProposerRefusesOptionShapedSessionID: the public constructor's
// proposer refuses an id like "--model=x" before it starts claude.
func TestNewProposerRefusesOptionShapedSessionID(t *testing.T) {
	binDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	script := "#!/bin/sh\ntouch '" + marker + "'\n"
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	p := NewProposer("--model=x", t.TempDir())
	if _, err := p.FirstRound(context.Background(), "ptr"); !errors.Is(err, input.ErrInvalidSessionID) {
		t.Fatalf("err = %v, want ErrInvalidSessionID", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("claude ran")
	}
}
