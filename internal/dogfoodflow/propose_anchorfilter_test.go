package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// anchorFilterDraft commits files as BASE, then changed over it, and returns
// the draft propose-citations prints with its stderr.
func anchorFilterDraft(t *testing.T, files, changed map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, changed)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// draftRowFor returns the draft row whose hunk edits name, found by the
// unknown note or by the order of hunks (one hunk per file here).
func draftRowFor(t *testing.T, draft, name string, index int) string {
	t.Helper()
	rows := textLines([]byte(draft))
	for _, row := range rows {
		if strings.Contains(row, "hunk edits "+name+" ") {
			return row
		}
	}
	if index < len(rows) {
		return rows[index]
	}
	t.Fatalf("no draft row for %s:\n%s", name, draft)
	return ""
}

// DCW-V0-847 (V1-3511): a spec-traceability hunk is not anchored on another
// spec's traceability row that merely mentions its path. Before the filter,
// the other family's row (AFP-V0-072 naming docs/specs/README.md) was drafted
// for the README table hunk because the change's other edits named a
// family and the row named the whole path.
func TestProposeCitationsDCWV0847TraceabilityRowOfAnotherSpec(t *testing.T) {
	other := "# A v0\n\n## Traceability\n\n| `AFP-V0-072` | the test lists docs/specs/README.md rows | test |\n" + numbered("a", 6)
	readme := "# Specs\n\n| Spec | State |\n| --- | --- |\n| one | open |\n" + numbered("r", 6)
	draft, stderr := anchorFilterDraft(t,
		map[string]string{"docs/specs/a-v0.md": other, "docs/specs/README.md": readme, "internal/q/q.go": "package q\n\n// Q answers.\nfunc Q() int { return 1 }\n"},
		map[string]string{
			"docs/specs/README.md": strings.Replace(readme, "| one | open |", "| one | open |\n| two | open |", 1),
			"internal/q/q.go":      "package q\n\n// Q answers QQQ-V0-001.\nfunc Q() int { return 2 }\n",
		})
	if strings.Contains(draft, "docs/specs/a-v0.md\t5:5") {
		t.Fatalf("the README hunk was anchored on another spec's traceability row:\n%s\n%s", draft, stderr)
	}
}

// DCW-V0-847 (V1-3511): a generated index hunk naming GEN-V0-002 is not
// anchored on the GEN-V0-001 row that names the index path; with the
// definition list present, it is drafted GEN-V0-002's definition line.
func TestProposeCitationsDCWV0847GeneratedIndexHunk(t *testing.T) {
	index := "# Index\n\n" + numbered("entry", 8)
	changed := map[string]string{"docs/specs/INDEX.md": strings.Replace(index, "entry line 4\n", "entry line 4\nentry new for GEN-V0-002\n", 1)}
	table := "# G v0\n\n## Traceability\n\n| GEN-V0-001 | the generator writes docs/specs/INDEX.md |\n| GEN-V0-002 | rows are sorted |\n" + numbered("g", 6)
	draft, stderr := anchorFilterDraft(t, map[string]string{"docs/specs/g-v0.md": table, "docs/specs/INDEX.md": index}, changed)
	if row := draftRowFor(t, draft, "docs/specs/INDEX.md", 0); strings.Contains(row, "docs/specs/g-v0.md\t5:5") {
		t.Fatalf("the index hunk naming GEN-V0-002 was anchored on the GEN-V0-001 row:\n%s\n%s", draft, stderr)
	}
	if !strings.Contains(stderr, "docs/specs/g-v0.md:5 names GEN-V0-001 but none of the hunk's GEN-V0-002, so it is another requirement's line (DCW-V0-847)") {
		t.Fatalf("the draft does not name the set-aside row:\n%s\n%s", draft, stderr)
	}
	list := "# G v0\n\n## Requirements\n\n- `GEN-V0-001`: the generator writes docs/specs/INDEX.md.\n- `GEN-V0-002`: rows are sorted.\n" + numbered("g", 6)
	draft, stderr = anchorFilterDraft(t, map[string]string{"docs/specs/g-v0.md": list, "docs/specs/INDEX.md": index}, changed)
	if row := draftRowFor(t, draft, "docs/specs/INDEX.md", 0); !strings.Contains(row, "docs/specs/g-v0.md\t6:6") {
		t.Fatalf("the index hunk naming GEN-V0-002 was not drafted its definition line 6:\n%s\n%s", draft, stderr)
	}
}

// DCW-V0-847 (V1-3833): of two consecutive requirement rows, only the first
// names the hunk's path; a hunk naming the second requirement is never drafted
// the first row, and is drafted the second requirement's definition line when
// the spec has one.
func TestProposeCitationsDCWV0847ConsecutiveRequirementLines(t *testing.T) {
	rows := "| SRG-V0-002 | the regex in `script/gen.sh` | check |\n| SRG-V0-003 | the rank patterns | check |\n"
	script := "#!/bin/sh\n# gen\n" + numbered("# body", 8)
	changed := map[string]string{"script/gen.sh": strings.Replace(script, "# body line 4\n", "# body line 4\n# SRG-V0-003: rank 2 is discarded\n", 1)}
	draft, stderr := anchorFilterDraft(t, map[string]string{"docs/specs/s-v0.md": "# S v0\n\n## Traceability\n\n" + rows + numbered("s", 6), "script/gen.sh": script}, changed)
	if row := draftRowFor(t, draft, "script/gen.sh", 0); strings.Contains(row, "docs/specs/s-v0.md\t5:5") {
		t.Fatalf("the SRG-V0-003 hunk was drafted the neighbouring SRG-V0-002 row:\n%s\n%s", draft, stderr)
	}
	spec := "# S v0\n\n## Requirements\n\n- `SRG-V0-002`: the regex.\n- `SRG-V0-003`: the rank.\n\n## Traceability\n\n" + rows + numbered("s", 6)
	draft, stderr = anchorFilterDraft(t, map[string]string{"docs/specs/s-v0.md": spec, "script/gen.sh": script}, changed)
	if row := draftRowFor(t, draft, "script/gen.sh", 0); !strings.Contains(row, "docs/specs/s-v0.md\t6:6") {
		t.Fatalf("the SRG-V0-003 hunk was not drafted its definition line 6:\n%s\n%s", draft, stderr)
	}
}

// DCW-V0-847 (V1-3694): a test hunk whose only link to ASD-V0-032 is fixture
// text gets no anchor for that ID, and the row's note names the fixture
// mention; a test whose tested file names the ID keeps the anchor.
func TestProposeCitationsDCWV0847FixtureOnlyMention(t *testing.T) {
	spec := "# D v0\n\n## Requirements\n\n- `ASD-V0-032`: the agent reads its route.\n" + numbered("d", 6)
	test := "#!/bin/sh\n# collisions test\n" + numbered("# case", 8)
	changed := strings.Replace(test, "# case line 4\n", "# case line 4\nprintf '%s\\n' '- `ASD-V0-032`: fixture' > dup.md\n", 1)
	draft, stderr := anchorFilterDraft(t,
		map[string]string{"docs/specs/d-v0.md": spec, "script/collisions_test.sh": test, "script/collisions.sh": "#!/bin/sh\n# finds collisions\n"},
		map[string]string{"script/collisions_test.sh": changed})
	row := draftRowFor(t, draft, "script/collisions_test.sh", 0)
	if strings.Contains(row, "docs/specs/d-v0.md\t5:5") {
		t.Fatalf("a fixture-only mention of ASD-V0-032 was drafted its definition:\n%s\n%s", draft, stderr)
	}
	if strings.Contains(row, "\tunknown\t") && !strings.Contains(row, "names ASD-V0-032 only in test or fixture text, which is not an anchor (DCW-V0-847)") {
		t.Fatalf("the unknown row does not name the fixture-only mention:\n%s", draft)
	}
	draft, stderr = anchorFilterDraft(t,
		map[string]string{"docs/specs/d-v0.md": spec, "script/collisions_test.sh": test, "script/collisions.sh": "#!/bin/sh\n# implements ASD-V0-032\n"},
		map[string]string{"script/collisions_test.sh": changed})
	if row := draftRowFor(t, draft, "script/collisions_test.sh", 0); !strings.Contains(row, "docs/specs/d-v0.md\t5:5") {
		t.Fatalf("a test of a file naming ASD-V0-032 lost its definition anchor:\n%s\n%s", draft, stderr)
	}
}
