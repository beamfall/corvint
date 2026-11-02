package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestASDV0843ProposeDraftsNoRowForAResealOfTheSameContent (V1-3961): on a
// head that carries its own seal and nothing stacked on it, propose-citations,
// with or without --refresh, drafts no row for that seal's CEM hunk and says
// why; once a later commit changes content on top of the seal, the stacked
// rebind keeps its DCW-V0-385 row.
func TestASDV0843ProposeDraftsNoRowForAResealOfTheSameContent(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	code := "package x\n\n" + numbered("// c", 60)
	writeTree(t, root, map[string]string{
		"pkg/code.go": code,
		"docs/DOGFOOD.md": "# Dogfood\n\n" + numbered("intro", 4) +
			"The seal renames the checked CEM to `.corvint/changes/<bind-commit>.cem.json`.\n" + numbered("outro", 4),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"pkg/code.go": strings.Replace(code, "// c line 10\n", "// c line 10 changed\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "change")
	bind := testGit(t, root, "rev-parse", "HEAD")
	sealed := ".corvint/changes/" + bind + ".cem.json"
	writeTree(t, root, map[string]string{sealed: "{\n  \"baseRevision\": \"" + base + "\"\n}\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "chore: seal change evidence")
	propose := func(refresh string) ([]string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		status, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: refresh}, &stdout, &stderr)
		if err != nil || status != 0 {
			t.Fatalf("ProposeCitations(refresh=%q) = %d, %v\n%s", refresh, status, err, stderr.String())
		}
		return textLines(stdout.Bytes()), stderr.String()
	}
	citesSeal := func(rows []string) bool {
		for _, row := range rows {
			if strings.Contains(row, "\tdocs/DOGFOOD.md\t") {
				return true
			}
		}
		return false
	}
	rows, errText := propose("")
	if len(rows) != 1 || citesSeal(rows) || !strings.Contains(errText, " of "+sealed+" drafted no row: no change is stacked on that seal") {
		t.Fatalf("reseal draft rows=%d, want the one code hunk and the reseal note:\n%s\n%s", len(rows), strings.Join(rows, "\n"), errText)
	}
	plan := filepath.Join(t.TempDir(), "plan.tsv")
	if err := os.WriteFile(plan, []byte(strings.Join(rows, "\n")+"\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if rows, errText = propose(plan); len(rows) != 1 || citesSeal(rows) {
		t.Fatalf("reseal --refresh rows=%d, want only the carried code row:\n%s\n%s", len(rows), strings.Join(rows, "\n"), errText)
	}
	// A repair on top of the seal makes the head a stacked rebind.
	writeTree(t, root, map[string]string{"pkg/code.go": strings.Replace(code, "// c line 40\n", "// c line 40 changed\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "repair")
	if rows, errText = propose(""); !citesSeal(rows) || strings.Contains(errText, "drafted no row") {
		t.Fatalf("stacked rebind lost its DCW-V0-385 row:\n%s\n%s", strings.Join(rows, "\n"), errText)
	}
}
