// SPDX-License-Identifier: Apache-2.0
package golang_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-9930: comparisons do not consume repository contents; a real read
// in the same binding, a returned path and a dependent retain rule (d).
func TestLocatorComparisonSelection_AZU_V0_9930(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":             "module example.test/m\n",
		"guard/g.go":         "package guard\nimport system \"os\"\nfunc F() { cwd, err := system.Getwd(); if err != nil || cwd != \"/product\" { return } }\n",
		"reader/r.go":        "package reader\nimport \"os\"\nfunc F() { cwd, _ := os.Getwd(); if cwd != \"/product\" { return }; os.ReadDir(cwd) }\n",
		"escape/e.go":        "package escape\nimport \"os\"\nfunc F() string { cwd, _ := os.Getwd(); return cwd }\n",
		"dependent/d.go":     "package dependent\nimport _ \"example.test/m/reader\"\n",
		"elsewhere/data.txt": "x\n",
	})
	for _, name := range []string{"guard", "reader", "escape", "dependent"} {
		writeFiles(t, root, map[string]string{name + "/x_test.go": "package " + name + "\n"})
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"elsewhere/data.txt"})
	found := map[string]bool{}
	for _, selected := range plan.Selected {
		found[selected.UnitID] = true
	}
	if found["go:example.test/m/guard"] {
		t.Fatal("comparison-only guard selected for unrelated data")
	}
	for _, name := range []string{"reader", "escape", "dependent"} {
		if !found["go:example.test/m/"+name] {
			t.Errorf("real reader %s dropped", name)
		}
	}
}
