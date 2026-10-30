package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0385CitesPriorSealOnStackedRepair pins the stacked
// repair of V1-3118: a commit after a seal makes BASE..HEAD add each prior
// seal's CEM, .corvint/changes/<bind>.cem.json, as a new hunk no BASE line can
// anchor. Each such hunk gets a cite row on a stable BASE line of
// docs/DOGFOOD.md that defines the sealed path, a different line per sealed
// file, instead of an unknown row the bind loop must rewrite; the code hunks
// stay unknown since they edit their only candidate file (DCW-V0-439), and the rows are keyed by the current patch's
// hunk IDs.
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
	first := seal("first")
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "40")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "repair")
	second := seal("second")
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "40", "55")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "second repair")

	var stdout, stderr bytes.Buffer
	status, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr)
	if err != nil || status != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", status, err, stderr.String())
	}
	out, errText := stdout.String(), stderr.String()
	rows := textLines([]byte(out))
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 3 code hunks and 2 sealed CEMs:\n%s\n%s", len(rows), out, errText)
	}
	spans := map[string]bool{}
	sealed := 0
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 4 || len(fields[0]) < 40 {
			t.Fatalf("row is not a hunk-ID keyed plan row: %q", row)
		}
		mapped := ""
		for _, name := range []string{first, second, "pkg/code.go"} {
			if strings.Contains(errText, "hunk "+fields[0]+" edits "+name+" at BASE ") {
				mapped = name
			}
		}
		if unknownRow(fields) {
			// A code hunk's only candidates are lines of the file it modifies,
			// so it stays unknown (DCW-V0-439); a sealed CEM never does.
			if mapped != "pkg/code.go" || !strings.Contains(fields[3], "(DCW-V0-439)") {
				t.Errorf("hunk of %s drafted unknown: %q\n%s", mapped, row, errText)
			}
			continue
		}
		if spans[fields[1]+":"+fields[2]] {
			t.Errorf("span %s:%s cited twice:\n%s", fields[1], fields[2], out)
		}
		spans[fields[1]+":"+fields[2]] = true
		if mapped == first || mapped == second {
			sealed++
			if fields[1] != "docs/DOGFOOD.md" || fields[3] != "specification" {
				t.Errorf("sealed CEM %s not cited to the seal contract: %q", mapped, row)
			}
		}
	}
	if sealed != 2 {
		t.Fatalf("sealed CEM rows = %d, want 2:\n%s\n%s", sealed, out, errText)
	}
	if !strings.Contains(errText, "cited to the seal contract that writes it (DCW-V0-385)") {
		t.Errorf("stderr does not say why the sealed CEMs are cited:\n%s", errText)
	}
}
