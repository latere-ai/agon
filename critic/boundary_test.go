// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package critic_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const (
	// runtimeModule is the agent runtime the engine left. Nothing here may
	// depend on it, so a consumer on any version of it can import this module.
	runtimeModule = "latere.ai/x/topos"
	// modelSeam is the model client. Only the critic package calls a model.
	modelSeam = "latere.ai/x/pkg/luxsdk"
)

// modulePath reads this module's path from `go list -m`, so the tests follow
// the module rather than restating go.mod.
func modulePath(t *testing.T) string {
	t.Helper()
	mod, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	return strings.TrimSpace(string(mod))
}

// goList runs `go list` over every package of this module, one line per
// package. Test imports are not listed (.Imports and -deps exclude them), so
// this file and the critic's tests may import anything.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	pattern := modulePath(t) + "/..."
	out, err := exec.Command("go", append(append([]string{"list"}, args...), pattern)...).CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

// TestNoPackageImportsTopos holds the module free of the agent runtime: no
// package of it, nor anything those packages build on, imports a path of the
// runtime module.
func TestNoPackageImportsTopos(t *testing.T) {
	var offenders []string
	for _, imp := range goList(t, "-deps", "-f", "{{.ImportPath}}") {
		if imp == runtimeModule || strings.HasPrefix(imp, runtimeModule+"/") {
			offenders = append(offenders, imp)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("the module depends on %s:\n%s", runtimeModule, strings.Join(offenders, "\n"))
	}
}

// TestOnlyCriticPackageCallsModels enforces the backend seam: within the
// module only the critic package imports the model client. The engine core
// and every other package reach a model only through a Critic or Proposer the
// caller supplies, so a model client stays an opt-in backend rather than a
// core dependency.
func TestOnlyCriticPackageCallsModels(t *testing.T) {
	allowed := modulePath(t) + "/critic"
	var offenders []string
	var allowedUsesSeam bool
	for _, line := range goList(t, "-f", "{{.ImportPath}} {{join .Imports \" \"}}") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !slices.Contains(fields[1:], modelSeam) {
			continue
		}
		if fields[0] == allowed {
			allowedUsesSeam = true
			continue
		}
		offenders = append(offenders, fields[0])
	}
	// Control: the critic package itself imports the seam, so the scan is
	// reading real import lists rather than passing over nothing.
	if !allowedUsesSeam {
		t.Errorf("control: %s does not import %s", allowed, modelSeam)
	}
	if len(offenders) > 0 {
		t.Errorf("only %s may import %s:\n%s", allowed, modelSeam, strings.Join(offenders, "\n"))
	}
}
