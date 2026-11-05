// SPDX-License-Identifier: Apache-2.0
package main

import (
	"reflect"
	"testing"
)

// AZU-V0-006 / TSP-V0-005: a verified shell caller declaration can name
// an unowned Makefile. Reading changed Go bytes does not close that caller.
func TestAZUV0006DeclaredReaderUnownedCaller(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	affectedGit(t, root, "init", "-q")
	cemWrite(t, root, "go.mod", "module example.com/reader\n\ngo 1.22\n")
	cemWrite(t, root, "data/data.go", "package data\nconst Value = 1\n")
	cemWrite(t, root, "check.sh", "#!/bin/sh\ncat data/data.go\n")
	cemWrite(t, root, "Makefile", "check:\n\tsh check.sh\n")
	cemWrite(t, root, ".corvint/shell-foreign-callers.json", `{"profile":"corvint-shell-foreign-callers/0","scripts":{"check.sh":{"callers":["Makefile"],"reason":"Makefile runs the reader"}}}`+"\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declared reader fixture")
	cemWrite(t, root, "data/data.go", "package data\nconst Value = 2\n")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	want := []any{map[string]any{"reason": "NO_SELECTABLE_TEST", "detail": "go:example.com/reader/data"}}
	if plan["scope"] != "UNKNOWN" || plan["uncovered"] != nil || !reflect.DeepEqual(plan["unknown"], want) {
		t.Fatalf("unowned reader caller must prevent a coverage proof: %v", plan)
	}
}
