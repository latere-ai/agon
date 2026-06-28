package topos_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestOnlyToposPackageImportsTopos enforces spec 39's seam: only
// pkg/adversarial/topos may import latere.ai/x/topos (root or subpackage). The
// engine core (pkg/adversarial) and every other package must not, so topos
// stays an opt-in backend rather than a core dependency. Test imports are not
// counted (go list .Imports excludes them), so this file importing topos is
// fine. Adapts wallfacer's internal/agentgraph/boundary_test.go.
func TestOnlyToposPackageImportsTopos(t *testing.T) {
	out, err := exec.Command("go", "list", "-f",
		"{{.ImportPath}} {{range .Imports}}{{.}} {{end}}",
		"latere.ai/x/agon/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	const allowed = "latere.ai/x/agon/pkg/adversarial/topos"
	var offenders []string
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if pkg == allowed {
			continue
		}
		for _, imp := range fields[1:] {
			if imp == "latere.ai/x/topos" || strings.HasPrefix(imp, "latere.ai/x/topos/") {
				offenders = append(offenders, pkg+" -> "+imp)
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("only %s may import topos:\n%s", allowed, strings.Join(offenders, "\n"))
	}
}
