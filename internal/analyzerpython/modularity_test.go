package analyzerpython

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCoreDependencyClosureExcludesPythonCandidate(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-001 Core unreachable separately buildable candidate"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = strings.TrimSpace(string(root))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list Core dependencies: %v\n%s", err, output)
	}
	for _, forbidden := range []string{"github.com/Beamfall/corvint/internal/analyzerpython"} {
		if strings.Contains(string(output), forbidden+"\n") {
			t.Fatalf("Core depends on candidate-only package %s", forbidden)
		}
	}
}
