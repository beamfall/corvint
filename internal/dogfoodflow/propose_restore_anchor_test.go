package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// DCW-V0-2141 (V1-4441): a reviewer who replaces a hunk's drafted anchoring
// row with the requirement row it implements leaves the hunk `cited`; an
// explicit `propose-citations --refresh PLAN` keeps the reviewer's row and
// drafts the anchoring row back beside it, while a hunk whose carried rows
// already anchor, and finish-lane's carry-only re-key, add nothing.
func TestProposeCitationsDCWV02141RefreshRestoresAnchoringRow(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 12) +
		"\n## Traceability\n\n| `XYZ-V0-001` | `internal/x/named.go` `Named` | implemented |\n" +
		"| `XYZ-V0-001` | `internal/x/other.go` `Other` | implemented |\n" + numbered("trace", 6)
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 20) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{"docs/specs/x-v0.md": spec, "internal/x/named.go": source("Named"), "internal/x/other.go": source("Other")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"internal/x/named.go": strings.Replace(source("Named"), "\t// Named line 10\n", "\t// Named line 10 changed\n", 1),
		"internal/x/other.go": strings.Replace(source("Other"), "\t// Other line 10\n", "\t// Other line 10 changed\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")

	propose := func(options ProposeOptions) (rows []string, notes string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		run := &proposal{
			flow:    flow{ctx: context.Background(), prefix: "dogfood-propose-citations", root: root, stderr: &stderr},
			options: options, stdout: &stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{},
		}
		if code, err := boundary(context.Background(), run.run, func() {}); err != nil || code != 0 {
			t.Fatalf("propose-citations %+v = %d, %v\n%s", options, code, err, stderr.String())
		}
		return textLines(stdout.Bytes()), stderr.String()
	}
	draft, notes := propose(ProposeOptions{Root: root, Base: base})
	specLines := splitRecords([]byte(testGit(t, root, "show", base+":docs/specs/x-v0.md") + "\n"))
	namedAnchor := "docs/specs/x-v0.md\t" + lineOf(t, specLines, "| `XYZ-V0-001` | `internal/x/named.go`") + "\tspecification"
	otherAnchor := "docs/specs/x-v0.md\t" + lineOf(t, specLines, "| `XYZ-V0-001` | `internal/x/other.go`") + "\tspecification"
	requirement := "docs/specs/x-v0.md\t" + lineOf(t, specLines, "- `XYZ-V0-001`:") + "\tspecification"
	if len(draft) != 2 {
		t.Fatalf("draft rows %q\n%s", draft, notes)
	}
	// The reviewer cites the named.go hunk to its requirement only and keeps
	// the other.go hunk's anchoring row.
	plan, namedID := []string{}, ""
	for _, row := range draft {
		id, rest, _ := strings.Cut(row, "\t")
		switch rest {
		case namedAnchor:
			namedID = id
			plan = append(plan, id+"\t"+requirement)
		case otherAnchor:
			plan = append(plan, row)
		default:
			t.Fatalf("unexpected draft row %q (want %q or %q)", row, namedAnchor, otherAnchor)
		}
	}
	planPath := writeInput(t, strings.Join(plan, "\n")+"\n")

	refreshed, notes := propose(ProposeOptions{Root: root, Base: base, Refresh: planPath})
	want := append(append([]string{}, plan...), namedID+"\t"+namedAnchor)
	if strings.Join(sortedCopy(refreshed), "\n") != strings.Join(sortedCopy(want), "\n") {
		t.Errorf("refresh rows:\n%s\nwant:\n%s", strings.Join(refreshed, "\n"), strings.Join(want, "\n"))
	}
	if !strings.Contains(notes, "hunk "+namedID+" of internal/x/named.go: its carried rows anchor nothing (CEM-V1-081)") ||
		!strings.Contains(notes, " anchors-restored=1") || strings.Count(notes, "anchor nothing") != 1 {
		t.Errorf("refresh notes do not name the one restored anchor:\n%s", notes)
	}
	// A second refresh of the restored plan is a fixed point.
	again, notes := propose(ProposeOptions{Root: root, Base: base, Refresh: writeInput(t, strings.Join(refreshed, "\n")+"\n")})
	if strings.Join(sortedCopy(again), "\n") != strings.Join(sortedCopy(want), "\n") || strings.Contains(notes, "anchors-restored") {
		t.Errorf("refresh of a restored plan changed it:\n%s\n%s", strings.Join(again, "\n"), notes)
	}
	// finish-lane's automatic re-key carries the rows only.
	carried, notes := propose(ProposeOptions{Root: root, Base: base, Refresh: planPath, CarryOnly: true})
	if strings.Join(sortedCopy(carried), "\n") != strings.Join(sortedCopy(plan), "\n") || strings.Contains(notes, "anchor nothing") {
		t.Errorf("carry-only refresh changed the plan:\n%s\n%s", strings.Join(carried, "\n"), notes)
	}
}

func sortedCopy(lines []string) []string {
	out := append([]string{}, lines...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
