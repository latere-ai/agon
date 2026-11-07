// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package agon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"latere.ai/x/agon"
	"latere.ai/x/agon/input"
)

// TestRunRefusesADiffOverTheCap: a DiffPatch over input.MaxDiffBytes,
// however the caller made it, is refused before any session exists or any
// agent runs, since every critic round would send all of it.
func TestRunRefusesADiffOverTheCap(t *testing.T) {
	state := t.TempDir()
	called := false
	_, err := agon.Review(context.Background(), agon.ReviewOptions{
		StateDir: state, Cwd: state, Forks: 1,
		Proposer:  nopProposer{},
		NewCritic: func(int) agon.Critic { called = true; return nil },
		DiffPatch: strings.Repeat("+x\n", input.MaxDiffBytes/3+1),
	})
	if !errors.Is(err, input.ErrDiffTooLarge) {
		t.Fatalf("err = %v, want input.ErrDiffTooLarge", err)
	}
	if called {
		t.Error("a critic was made for a diff over the cap")
	}
	if entries, _ := os.ReadDir(filepath.Join(state, "sessions")); len(entries) != 0 {
		t.Errorf("a session was created: %v", entries)
	}
}

type nopProposer struct{}

func (nopProposer) FirstRound(context.Context, string) (*agon.ProposerResult, error) {
	return &agon.ProposerResult{ForkID: "f"}, nil
}

func (nopProposer) NextRound(_ context.Context, id, _ string) (*agon.ProposerResult, error) {
	return &agon.ProposerResult{ForkID: id}, nil
}
