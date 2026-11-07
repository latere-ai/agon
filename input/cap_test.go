// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package input

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestComputeRefusesADiffOverTheCap: a diff over MaxDiffBytes is refused
// with ErrDiffTooLarge, whether the bytes come from one untracked file or
// from several that pass the cap only together.
func TestComputeRefusesADiffOverTheCap(t *testing.T) {
	line := bytes.Repeat([]byte("x"), 99)
	line = append(line, '\n')
	for name, sizes := range map[string][]int{
		"one file":          {MaxDiffBytes + 1},
		"files that add up": {MaxDiffBytes / 2, MaxDiffBytes / 2, 4096},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			for i, n := range sizes {
				data := bytes.Repeat(line, n/len(line)+1)
				if err := os.WriteFile(filepath.Join(dir, "big"+string(rune('a'+i))+".txt"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			d, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir})
			if !errors.Is(err, ErrDiffTooLarge) {
				t.Fatalf("err = %v, want ErrDiffTooLarge (patch %d bytes)", err, patchLen(d))
			}
		})
	}
	// A diff under the cap still works.
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), bytes.Repeat(line, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compute(context.Background(), DiffSpec{From: "HEAD", To: ".", Cwd: dir}); err != nil {
		t.Fatalf("a 100 KB diff: %v", err)
	}
}

func patchLen(d *Diff) int {
	if d == nil {
		return 0
	}
	return len(d.Patch)
}
