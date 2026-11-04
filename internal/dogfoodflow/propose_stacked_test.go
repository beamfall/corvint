package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// DCW-V0-385 amended by DWC-V0-926: generic seal instructions do not name
// a future bind commit path; stacked-repair rows retain uncertainty.
func TestProposeCitationsDCWV0385CitesPriorSealOnStackedRepair(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	code := "package x\n\n" + numbered("// c", 60)
	writeTree(t, root, map[string]string{
		"pkg/code.go": code,
		"docs/DOGFOOD.md": "# Dogfood\n\n" + numbered("intro", 4) +
			"The seal renames the checked CEM to `.corvint/changes/<bind-commit>.cem.json`.\n" + numbered("middle", 4) +
			"A sealed CEM stays at `.corvint/changes/<bind-sha>.cem.json` once the branch moves on.\n" + numbered("outro", 4),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	edit := func(lines ...string) string {
		text := code
		for _, line := range lines {
			text = strings.Replace(text, "// c line "+line+"\n", "// c line "+line+" changed\n", 1)
		}
		return text
	}
	seal := func(label string) string {
		bind := testGit(t, root, "rev-parse", "HEAD")
		name := ".corvint/changes/" + bind + ".cem.json"
		writeTree(t, root, map[string]string{name: "{\n  \"baseRevision\": \"" + base + "\",\n  \"label\": \"" + label + "\"\n}\n"})
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-q", "-m", "chore: seal change evidence")
		return name
	}
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	seal("first")
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "40")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "repair")
	seal("second")
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "40", "55")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "second repair")

	var stdout, stderr bytes.Buffer
	status, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr)
	if err != nil || status != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", status, err, stderr.String())
	}
	// DWC-V0-926: a generic seal contract cannot name an as-yet nonexistent
	// bind commit path. Its rows stay unknown, as do unanchored code rows.
	assertAnchoredDraft(t, root, base, stdout.String())
}
