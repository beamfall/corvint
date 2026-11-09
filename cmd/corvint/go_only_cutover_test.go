package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/versionline"
)

func TestGoOnlySourceAndVersion(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	for _, pattern := range []string{"src/corvint_cli.py", "src/context_corvint*.py", "pyproject.toml"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil || len(matches) != 0 {
			t.Fatalf("GOC-V0-001 retired path %s: %v %v", pattern, matches, err)
		}
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || strings.TrimSpace(string(version)) == "" {
		t.Fatalf("GOC-V0-001 VERSION: %q %v", version, err)
	}
	output, err := candidateCommand("--version").CombinedOutput()
	// PUB-V0-053: an unstamped source build omits the build member and names a development version.
	line, parsed := versionline.Parse(string(output))
	if err != nil || !parsed || line.Name != "Corvint" || line.Version != versionline.DevelopmentVersion(strings.TrimSpace(string(version))) || line.Build != "" || line.Commit == "" {
		t.Fatalf("GOC-V0-001 native version: %q %v", output, err)
	}
	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.y*ml"))
	if err != nil || len(workflows) == 0 {
		t.Fatalf("GOC-V0-001 CI workflows: %v %v", workflows, err)
	}
	for _, workflow := range workflows {
		body, err := os.ReadFile(workflow)
		if err != nil {
			t.Fatalf("GOC-V0-001 CI workflow %s: %v", workflow, err)
		}
		lower := strings.ToLower(string(body))
		for _, needle := range []string{"wheel", "pypi", "twine", "bdist_wheel", "cibuildwheel", "setup.py"} {
			if strings.Contains(lower, needle) {
				t.Fatalf("GOC-V0-001 CI workflow %s retains a wheel release job (%q)", workflow, needle)
			}
		}
	}
}
