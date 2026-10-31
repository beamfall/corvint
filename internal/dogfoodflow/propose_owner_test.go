package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0886PrefersTheGoverningSpec reproduces the
// dogfood-w6 case of V1-3283 in a fixture: five changed paths, each named by a
// bare line of an unrelated spec that sorts first and by a requirement line
// of the spec that names them all. Four rows cite the governing spec; the
// path only the unrelated spec names is still cited but marked
// low-confidence. Fails on BASE, which cited the first line naming each path.
func TestProposeCitationsDCWV0886PrefersTheGoverningSpec(t *testing.T) {
	body := func(name string) string {
		return "// " + name + " head\n" + numbered("// "+name, 8) + "// " + name + " tail\n"
	}
	files := map[string]string{
		"docs/specs/aaa-loose-v0.md": "# Loose\n\nSee internal/x/a.go for an example.\nAlso script/c.sh is used.\nAlso internal/x/b_test.go here.\nAnd docs/GUIDE.md there.\nAnd internal/x/d.go once.\n",
		"docs/specs/gov-v0.md":       "# Gov\n\n## Requirements\n\n- `GOV-V0-001`: internal/x/a.go does a.\n- `GOV-V0-002`: internal/x/b_test.go tests a.\n- `GOV-V0-003`: script/c.sh runs it.\n- `GOV-V0-004`: docs/GUIDE.md explains it.\n",
	}
	changed := []string{"docs/GUIDE.md", "internal/x/a.go", "internal/x/b_test.go", "internal/x/d.go", "script/c.sh"}
	for _, name := range changed {
		files[name] = body(name)
	}
	root, _ := preflightRepository(t, nil)
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base for the governing-spec draft")
	base := testGit(t, root, "rev-parse", "HEAD")
	edits := map[string]string{}
	for _, name := range changed {
		edits[name] = strings.Replace(body(name), "line 4\n", "line 4 changed\n", 1)
	}
	writeTree(t, root, edits)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change five paths")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != len(changed) {
		t.Fatalf("rows=%q\n%s", rows, stderr.String())
	}
	want := map[string]string{"docs/GUIDE.md": "docs/specs/gov-v0.md\t8:8", "internal/x/a.go": "docs/specs/gov-v0.md\t5:5",
		"internal/x/b_test.go": "docs/specs/gov-v0.md\t6:6", "internal/x/d.go": "docs/specs/aaa-loose-v0.md\t7:7", "script/c.sh": "docs/specs/gov-v0.md\t7:7"}
	for index, name := range changed {
		if fields := strings.Split(rows[index], "\t"); len(fields) != 4 || fields[1]+"\t"+fields[2] != want[name] {
			t.Errorf("row for %s = %q; want %s", name, rows[index], want[name])
		}
	}
	notes := stderr.String()
	if strings.Count(notes, "cited low-confidence (DCW-V0-886)") != 1 ||
		!strings.Contains(notes, "docs/specs/aaa-loose-v0.md:7 names internal/x/d.go outside the change's governing spec docs/specs/gov-v0.md, no stable line of which names it") {
		t.Errorf("the bare-mention row is not marked low-confidence alone:\n%s", notes)
	}
}

// TestDCWV0886AModifiedSpecGovernsTheChange: when the change amends a spec
// whose requirement lines name its paths, that spec governs, though it names
// fewer of them than an untouched spec and is never cited; the untouched
// spec's cites are kept and marked low-confidence.
func TestDCWV0886AModifiedSpecGovernsTheChange(t *testing.T) {
	body := func(name string) string {
		return "// " + name + " head\n" + numbered("// "+name, 8) + "// " + name + " tail\n"
	}
	own := "# Own\n\n## Requirements\n\n- `OWN-V0-001`: internal/x/a.go and internal/x/b.go.\n- `OWN-V0-002`: more.\n"
	files := map[string]string{
		"docs/specs/own-v0.md":   own,
		"docs/specs/broad-v0.md": "# Broad\n\n## Requirements\n\n- `BRD-V0-001`: internal/x/a.go.\n- `BRD-V0-002`: internal/x/b.go.\n- `BRD-V0-003`: internal/x/c.go.\n",
	}
	changed := []string{"internal/x/a.go", "internal/x/b.go", "internal/x/c.go"}
	for _, name := range changed {
		files[name] = body(name)
	}
	root, _ := preflightRepository(t, nil)
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	edits := map[string]string{"docs/specs/own-v0.md": strings.Replace(own, "# Own\n", "# Own, amended\n", 1)}
	for _, name := range changed {
		edits[name] = strings.Replace(body(name), "line 4\n", "line 4 changed\n", 1)
	}
	writeTree(t, root, edits)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "amend own spec and code")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	if got := strings.Count(stderr.String(), "outside the change's governing spec docs/specs/own-v0.md (modified by this change)"); got != 3 {
		t.Errorf("low-confidence notes naming the amended spec = %d, want 3:\n%s", got, stderr.String())
	}
	if strings.Contains(stderr.String(), "governing spec docs/specs/broad-v0.md") {
		t.Errorf("the untouched spec was named governing:\n%s", stderr.String())
	}
}
