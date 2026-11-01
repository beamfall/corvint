package affected_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryShellForeignCallersComplete_TSP_V0_005 verifies this
// repository's .corvint/shell-foreign-callers.json against the tree, so the
// declaration is checked rather than trusted. Every caller-candidate file
// that names a declared script's file name (a path literal, the bare file
// name, a Make recipe, a comment) or builds a name it may be must be listed as
// a caller or a mention, and every listed file must still name it, by the
// plugin's own rule (shell.ForeignCallerFindings). The shell plugin must then accept the declaration and drop the
// foreign-invocation frontier from the script.
func TestRepositoryShellForeignCallersComplete_TSP_V0_005(t *testing.T) {
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ShellForeignCallersPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Scripts map[string]struct {
			Callers  []string `json:"callers"`
			Mentions []string `json:"mentions"`
		} `json:"scripts"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	if len(declared.Scripts) == 0 {
		t.Fatalf("no script is declared: %s", body)
	}

	// The plugin's own rule decides completeness (TSP-V0-920, V1-3899): only
	// caller-candidate files count, by the script's file name or by a name a
	// shell file builds, and each disagreement is named by file and line, so a
	// comment naming a script is reported where it is (V1-3913).
	findings, err := shell.ForeignCallerFindings(affected.DiskSourceContext(context.Background(), root))
	if err != nil {
		t.Fatalf("%s: %v", affected.ShellForeignCallersPath, err)
	}
	for _, finding := range findings {
		t.Error(finding)
	}

	result, err := shell.New().UnitsSource(affected.DiskSourceContext(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(result.Frontier, shell.FrontierForeignCallersInvalid) {
		t.Fatalf("the shell plugin refused %s", affected.ShellForeignCallersPath)
	}
	for _, unit := range result.Units {
		if _, ok := declared.Scripts[strings.TrimPrefix(unit.ID, "shell:")]; ok {
			if !slices.Equal(unit.Callers, declared.Scripts[strings.TrimPrefix(unit.ID, "shell:")].Callers) || slices.Contains(unit.Frontier, shell.FrontierForeignInvocation) {
				t.Errorf("%s: callers %v, frontier %v; want declared callers and no foreign frontier", unit.ID, unit.Callers, unit.Frontier)
			}
		}
	}
}
