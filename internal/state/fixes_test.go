// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"path/filepath"
	"testing"
	"time"
)

// TestNewSessionResolvesRelativeStateDir covers B12: the session root and
// every path built from it are absolute whatever the caller passed.
func TestNewSessionResolvesRelativeStateDir(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := NewSession("rel-state", 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(s.Root) || !filepath.IsAbs(s.StateDirAbs) || !filepath.IsAbs(s.Path("x")) {
		t.Errorf("not absolute: root %q state %q", s.Root, s.StateDirAbs)
	}
}
