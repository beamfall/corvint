package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// DCW-V0-161 (V1-2355): propose-citations prefers a stable BASE line that
// earns the hunk a CEM-V1-081 anchor, the line defining a requirement ID the
// hunk adds, then a line naming the hunk path, so the check that recomputes
// the tier prints no tier-below-anchored warning for it. A hunk with no such
// line still gets a nearby line, and stderr records why that row fell back.
func TestProposeCitationsDCWV0161DraftsAnchoredRows(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 12) +
		"\n## Traceability\n\n| `XYZ-V0-002` | `internal/x/named.go` `Named` | implemented |\n" + numbered("trace", 6)
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 20) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec, "internal/x/x.go": source("X"), "internal/x/named.go": source("Named"), "internal/x/plain.go": source("Plain"),
		// One long record naming a path, as a one-line JSON document does:
		// the grep must report the match, not the whole record.
		"docs/notes.json": `{"note":"` + strings.Repeat("n", 40<<10) + ` internal/x/named.go"}` + "\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"internal/x/x.go":     strings.Replace(source("X"), "\t// X line 10\n", "\t// X line 10 frobs, as XYZ-V0-001 requires\n", 1),
		"internal/x/named.go": strings.Replace(source("Named"), "\t// Named line 10\n", "\t// Named line 10 changed\n", 1),
		"internal/x/plain.go": strings.Replace(source("Plain"), "\t// Plain line 10\n", "\t// Plain line 10 changed\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	hunks := diffHunks(t, root, base)
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": hunks})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})

	// A Git output limit below the long record's size proves the anchor read
	// stays within the flow's bound however long the matching lines are.
	var stdout, stderr bytes.Buffer
	run := &proposal{
		flow:    flow{ctx: context.Background(), prefix: "dogfood-propose-citations", root: root, stderr: &stderr, gitOutputLimit: 32 << 10},
		options: ProposeOptions{Root: root, Base: base}, stdout: &stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{},
	}
	if code, err := boundary(context.Background(), run.run, func() {}); err != nil || code != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(hunks) != 3 || len(rows) != 3 {
		t.Fatalf("draft has %d rows for %d hunks:\n%s", len(rows), len(hunks), stdout.String())
	}
	specLines := splitRecords([]byte(testGit(t, root, "show", base+":docs/specs/x-v0.md") + "\n"))
	want := map[string]string{
		"internal/x/x.go":     "docs/specs/x-v0.md\t" + lineOf(t, specLines, "- `XYZ-V0-001`:") + "\tspecification",
		"internal/x/named.go": "docs/specs/x-v0.md\t" + lineOf(t, specLines, "| `XYZ-V0-002` |") + "\tspecification",
		"internal/x/plain.go": "internal/x/plain.go\t4:4\tdependency",
	}
	for index, row := range rows {
		ordinal, rest, _ := strings.Cut(row, "\t")
		if ordinal != strconv.Itoa(index+1) || rest != want[hunks[index].Path] {
			t.Errorf("row %d = %q; want %q for %s", index+1, row, want[hunks[index].Path], hunks[index].Path)
		}
	}
	note := "  row 2 cited, not anchored (CEM-V1-081): no stable BASE line names internal/x/plain.go or defines a requirement ID the hunk adds\n"
	if !strings.Contains(stderr.String(), note) || strings.Count(stderr.String(), "not anchored") != 1 {
		t.Errorf("stderr does not record the one fallback:\n%s", stderr.String())
	}

	// The check's own tiering of the drafted rows: anchored rows draw no
	// CEM-V1-081 warning, the fallback row draws one.
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != 3 {
		t.Fatalf("canonical patch: %v (%d hunks)", err, len(parsed.Hunks))
	}
	verified := &wire.Map{Spec: wire.StableSpec}
	spans := map[string][]byte{}
	type statusHunk struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	statusHunks := []statusHunk{}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		first, _ := strconv.Atoi(strings.SplitN(fields[2], ":", 2)[0])
		evidenceID := "evidence:" + fields[0]
		blob := splitRecords([]byte(testGit(t, root, "show", base+":"+fields[1]) + "\n"))
		spans[evidenceID] = blob[first-1]
		verified.Evidence = append(verified.Evidence, wire.Evidence{ID: evidenceID, Path: fields[1]})
		hunk := parsed.Hunks[index]
		verified.Hunks = append(verified.Hunks, wire.Hunk{ID: hunk.ID, Disposition: "supported", Basis: []wire.Basis{{EvidenceID: evidenceID, Relation: fields[3]}}})
		statusHunks = append(statusHunks, statusHunk{hunk.ID, hunk.DisplayPath})
	}
	line, err := json.Marshal(map[string]any{"stableVerification": map[string]any{"hunks": statusHunks, "citation": verify.ComputeCitation(verified, parsed, spans)}})
	if err != nil {
		t.Fatal(err)
	}
	warnings := belowAnchoredWarnings(line, nil)
	if len(warnings) != 1 || !strings.Contains(warnings[0], `path="internal/x/plain.go"`) || !strings.Contains(warnings[0], "rule=CEM-V1-081") {
		t.Errorf("tier warnings = %q; want one, for the fallback row", warnings)
	}
}

// lineOf returns "N:N" for the one line of lines starting with prefix.
func lineOf(t *testing.T, lines [][]byte, prefix string) string {
	t.Helper()
	for index, line := range lines {
		if strings.HasPrefix(string(line), prefix) {
			return strconv.Itoa(index+1) + ":" + strconv.Itoa(index+1)
		}
	}
	t.Fatalf("no line starts with %q", prefix)
	return ""
}
