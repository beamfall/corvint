package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-443 (GH #1751, V1-2795): a generic `--base` plan whose change range
// adds `.corvint/documentation-paths.json` together with a declared document
// applies the head declaration and states both classifications; it never
// narrows silently. Without the declaration the same document stays an
// UNOWNED_DIRTY_PATH with scope UNKNOWN.
func TestAffectedDocumentationDeclarationAddedInRange_GH1751(t *testing.T) {
	t.Parallel()
	for _, declared := range []bool{true, false} {
		t.Run(fmt.Sprintf("declared=%v", declared), func(t *testing.T) {
			t.Parallel()
			root := affectedFixtureRepository(t)
			base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
			files := map[string]string{"guides/notes/plan.md": "# plan\n"}
			if declared {
				files[".corvint/documentation-paths.json"] = `{"profile":"corvint-documentation-paths/0","paths":["guides/notes/"]}` + "\n"
			}
			for relative, body := range files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, relative), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			affectedGit(t, root, "add", ".")
			affectedGit(t, root, "commit", "-qm", "range")
			receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--full")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			plan := receipt["plan"].(map[string]any)
			nonSource := fmt.Sprint(plan["nonSource"])
			if !declared {
				if plan["scope"] != "UNKNOWN" || !strings.Contains(fmt.Sprint(plan["unknown"]), "UNOWNED_DIRTY_PATH") || strings.Contains(nonSource, "DECLARED_DOCUMENTATION") {
					t.Fatalf("undeclared document must stay unknown: scope=%v unknown=%v nonSource=%v", plan["scope"], plan["unknown"], nonSource)
				}
				return
			}
			if plan["scope"] != "BOUNDED" || fmt.Sprint(plan["unknown"]) != "[]" ||
				!strings.Contains(nonSource, "class:DECLARED_DOCUMENTATION paths:[guides/notes/plan.md]") ||
				!strings.Contains(nonSource, "class:CORVINT_DECLARATION paths:[.corvint/documentation-paths.json]") {
				t.Fatalf("an in-range declaration must apply and be stated: scope=%v unknown=%v nonSource=%v", plan["scope"], plan["unknown"], nonSource)
			}
		})
	}
}
