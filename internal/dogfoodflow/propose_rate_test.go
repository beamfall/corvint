package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// selfDevelopmentRepository commits a base shaped like a Corvint
// self-development change and returns the root and base: a governing spec with
// requirements, its code and test, and docs/DOGFOOD.md with the sealed-path
// contract. The change amends the spec (a new requirement refining an existing
// one, and an edit inside another's entry), names the new requirement in the
// code and its test, edits the code and docs/DOGFOOD.md elsewhere, and adds a
// test file.
func selfDevelopmentRepository(t *testing.T) (root, base string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n" +
		"- `XYZ-V0-001`: the widget MUST frob.\n  Evidence: `TestFrob`.\n" + numbered("frob detail", 8) +
		"- `XYZ-V0-002`: the widget MUST log.\n  Evidence: `TestLog`.\n" +
		numbered("requirement", 12) + "\n## Non-goals\n\n" + numbered("goal", 4)
	code := "package x\n\n// Frob frobs.\nfunc Frob() int {\n" + numbered("\t// frob", 12) + "\treturn 1\n}\n\n// Log logs.\nfunc Log() int {\n" + numbered("\t// log", 12) + "\treturn 2\n}\n"
	tests := "package x\n\nimport \"testing\"\n\n// TestFrob covers XYZ-V0-001.\nfunc TestFrob(t *testing.T) {\n" + numbered("\t// case", 8) + "}\n"
	dogfood := "# Dogfood\n\n" + numbered("intro", 10) +
		"The seal renames the checked CEM to `.corvint/changes/<bind-commit>.cem.json`.\n" + numbered("outro", 6)
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec, "internal/x/x.go": code, "internal/x/x_test.go": tests, "docs/DOGFOOD.md": dogfood,
		"docs/specs/daily-change-evidence-workflow-v0.md": "# DCW\n\n## Requirements\n\n- `DCW-V0-385`: a prior seal's CEM is cited to the seal contract.\n" + numbered("dcw", 4),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": strings.NewReplacer(
			"  Evidence: `TestLog`.\n", "  Evidence: `TestLog`, `TestLogTwice`.\n",
			"requirement line 12\n", "requirement line 12\n- `XYZ-V0-003`: (refines `XYZ-V0-001`) the widget MUST frob twice.\n").Replace(spec),
		"internal/x/x.go": strings.NewReplacer("\t// frob line 6\n", "\t// frob line 6: twice, as XYZ-V0-003 requires\n",
			"\t// log line 6\n", "\t// log line 6 changed\n").Replace(code),
		"internal/x/x_test.go":   strings.Replace(tests, "\t// case line 4\n", "\t// case line 4: XYZ-V0-003 frobs twice\n", 1),
		"internal/x/new_test.go": "package x\n\nimport \"testing\"\n\n// TestFrobTwice covers XYZ-V0-003.\nfunc TestFrobTwice(t *testing.T) {}\n",
		"docs/DOGFOOD.md":        strings.Replace(dogfood, "intro line 2\n", "intro line 2 changed\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	return root, base
}

// unknownRows counts a draft's unknown rows.
func unknownRows(plan string) int {
	count := 0
	for _, row := range textLines([]byte(plan)) {
		if unknownRow(strings.Split(row, "\t")) {
			count++
		}
	}
	return count
}

// TestProposeCitationsDCWV0966BoundsTheUnknownRate (V1-3284, V1-3428): on a
// self-development change at most one row in five is unknown. The code, test
// and added test hunks naming the new requirement cite the BASE requirement it
// refines (DCW-V0-967), and the spec, code and docs hunks get their own file's
// marked proposals. The draft passes
// the bind's pre-flight with real cem prepare and cite, so no row is refused
// as cite-span-not-stable.
func TestProposeCitationsDCWV0966BoundsTheUnknownRate(t *testing.T) {
	root, base := selfDevelopmentRepository(t)
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	unknown := unknownRows(stdout.String())
	if len(rows) < 6 || unknown*5 > len(rows) {
		t.Fatalf("draft has %d unknown of %d rows, over one in five:\n%s\n%s", unknown, len(rows), stdout.String(), stderr.String())
	}
	// The code, its test and the added test file all name XYZ-V0-003, so each
	// cites the BASE definition of the XYZ-V0-001 it refines.
	for _, name := range []string{"internal/x/x.go at BASE 7,7", "internal/x/x_test.go at BASE 7,7", "internal/x/new_test.go at BASE 0,0 HEAD 1,6"} {
		found := false
		for _, row := range rows {
			id, _, _ := strings.Cut(row, "\t")
			found = found || strings.Contains(stderr.String(), "  hunk "+id+" edits "+name) && strings.HasSuffix(row, "\tdocs/specs/x-v0.md\t5:5\tspecification")
		}
		if !found {
			t.Errorf("the hunk of %s does not cite the refined XYZ-V0-001 definition:\n%s\n%s", name, stdout.String(), stderr.String())
		}
	}
	for _, want := range []string{
		"the BASE definition of XYZ-V0-001, which the entry of the same-change requirement XYZ-V0-003 refines, amends or names",
		"\tdocs/specs/x-v0.md\t5:5\tspecification\n",
	} {
		if !strings.Contains(stderr.String()+stdout.String(), want) {
			t.Errorf("draft lacks %q:\n%s\n%s", want, stdout.String(), stderr.String())
		}
	}
	code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: writeInput(t, stdout.String())})
	if code != 0 || strings.Contains(out+errText, "cite-span-not-stable") {
		t.Fatalf("the drafted plan failed the pre-flight: exit=%d\n%s\n%s\nplan:\n%s", code, out, errText, stdout.String())
	}
}

// TestProposeCitationsDCWV0966CitesPriorSealInEveryRoute (V1-3284): a prior
// seal's CEM gets its DCW-V0-385 cite with no map, with a prepared map, from
// --refresh over a hand plan that kept it unknown, and when the change also
// edits docs/DOGFOOD.md (the unedited sealed-path line still serves) or that
// very line (the DCW-V0-385 definition serves instead).
func TestProposeCitationsDCWV0966CitesPriorSealInEveryRoute(t *testing.T) {
	root, base := selfDevelopmentRepository(t)
	bind := testGit(t, root, "rev-parse", "HEAD")
	sealed := ".corvint/changes/" + bind + ".cem.json"
	writeTree(t, root, map[string]string{sealed: "{\n  \"baseRevision\": \"" + base + "\"\n}\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "chore: seal change evidence")
	writeTree(t, root, map[string]string{"internal/x/x.go": strings.Replace(readText(t, root, "internal/x/x.go"), "\t// log line 9\n", "\t// log line 9 repaired\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "repair")
	draft := func(options ProposeOptions) (string, string) {
		t.Helper()
		options.Root, options.Base = root, base
		var stdout, stderr bytes.Buffer
		if code, err := ProposeCitations(context.Background(), options, &stdout, &stderr); code != 0 || err != nil {
			t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	sealedRow := func(plan, errText string) string {
		t.Helper()
		for _, row := range textLines([]byte(plan)) {
			id, _, _ := strings.Cut(row, "\t")
			if strings.Contains(errText, "  hunk "+id+" edits "+sealed+" at BASE ") {
				return row
			}
		}
		t.Fatalf("no row for %s:\n%s\n%s", sealed, plan, errText)
		return ""
	}
	contract := "\tdocs/DOGFOOD.md\t13:13\tspecification"
	plan, errText := draft(ProposeOptions{})
	if row := sealedRow(plan, errText); !strings.HasSuffix(row, contract) {
		t.Errorf("no-map route: sealed row = %q; want %q", row, contract)
	}
	// The prepared-map route keys the rows by ordinal.
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": diffHunks(t, root, base)})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	mapped, mappedErr := draft(ProposeOptions{})
	if row := sealedRow(mapped, mappedErr); !strings.HasSuffix(row, contract) || strings.HasPrefix(row, "hunk:") {
		t.Errorf("map route: sealed row = %q; want an ordinal row citing %q\n%s", row, contract, mappedErr)
	}
	if err := os.Remove(filepath.Join(root, ".corvint", "change.cem.json")); err != nil {
		t.Fatal(err)
	}
	// --refresh over a hand plan that kept the sealed hunk unknown redrafts it.
	hand := ""
	for _, row := range textLines([]byte(plan)) {
		id, _, _ := strings.Cut(row, "\t")
		if strings.Contains(errText, "  hunk "+id+" edits "+sealed+" at BASE ") {
			row = id + "\tunknown\tinsufficient-evidence\thand-built plan had no mapping"
		}
		hand += row + "\n"
	}
	refreshed, refreshErr := draft(ProposeOptions{Refresh: writeInput(t, hand)})
	if row := sealedRow(refreshed, refreshErr); !strings.HasSuffix(row, contract) {
		t.Errorf("refresh route: sealed row = %q; want %q\n%s", row, contract, refreshErr)
	}
	// The change edits the sealed-path line itself: the DCW-V0-385 definition.
	writeTree(t, root, map[string]string{"docs/DOGFOOD.md": strings.Replace(readText(t, root, "docs/DOGFOOD.md"), "`.corvint/changes/<bind-commit>.cem.json`.", "`.corvint/changes/<bind-commit>.cem.json`, byte for byte.", 1)})
	testGit(t, root, "commit", "-q", "-am", "edit the seal contract")
	plan, errText = draft(ProposeOptions{})
	if row := sealedRow(plan, errText); !strings.HasSuffix(row, "\tdocs/specs/daily-change-evidence-workflow-v0.md\t5:5\tspecification") {
		t.Errorf("edited-contract route: sealed row = %q; want the DCW-V0-385 definition\n%s", row, errText)
	}
}

// TestProposeCitationsDCWV0967NamesAnEditedRequirementDefinition (V1-3428):
// a hunk naming a requirement whose BASE definition the change edits gets no
// cite of that unstable span; with no other anchor its unknown row says the
// definition is edited and cem cite would refuse it as cite-span-not-stable.
func TestProposeCitationsDCWV0967NamesAnEditedRequirementDefinition(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 8),
		"internal/x/x.go":    "package x\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob, twice.\n" + numbered("requirement", 8),
		"internal/x/x.go":    "package x\n\n// Frob implements XYZ-V0-001.\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	for _, row := range textLines(stdout.Bytes()) {
		if strings.Contains(row, "\tdocs/specs/x-v0.md\t5:5\t") {
			t.Errorf("row cites the edited definition: %q", row)
		}
	}
	if !strings.Contains(stdout.String(), "the BASE definition of XYZ-V0-001 at docs/specs/x-v0.md:5 is edited by this change or repeated at HEAD, so cem cite would refuse it as cite-span-not-stable") {
		t.Errorf("no row names the edited definition:\n%s\n%s", stdout.String(), stderr.String())
	}
}

func readText(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
