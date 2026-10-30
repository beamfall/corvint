package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// DCW-V0-388 (V1-3130): a below-anchored hunk whose basis cites a span of its
// own file, the ordinary citation of the function it edits, draws one
// same-path NOTE count instead of a tier warning; a hunk that cites only other
// files still warns although an anchor is available, and an unanchorable hunk
// stays in the DCW-V0-165 note. Each refusal keeps the warning.
func TestDCWV0388SamePathCitationIsANoteNotAWarning(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 12) +
		"\nOnly its basename names widened.go here.\n" +
		"\n## Traceability\n\n| `XYZ-V0-002` | `internal/x/named.go` `Named` | implemented |\n" + numbered("trace", 6)
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 20) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec, "internal/x/x.go": source("X"), "internal/x/named.go": source("Named"), "internal/x/plain.go": source("Plain"),
		"internal/x/widened.go": source("Widened"),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	for _, name := range []string{"X", "Named", "Plain", "Widened"} {
		file := "internal/x/" + strings.ToLower(name) + ".go"
		writeTree(t, root, map[string]string{file: strings.Replace(source(name), "\t// "+name+" line 10\n", "\t// "+name+" line 10 changed, as XYZ-V0-001 requires\n", 1)})
	}
	// Plain's edit adds no ID, so only its own line could anchor it: none does.
	writeTree(t, root, map[string]string{"internal/x/plain.go": strings.Replace(source("Plain"), "\t// Plain line 10\n", "\t// Plain line 10 changed\n", 1)})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != 4 {
		t.Fatalf("canonical patch: %v (%d hunks)", err, len(parsed.Hunks))
	}
	ids, paths, tiers := map[string]string{}, []string{}, []string{}
	for _, hunk := range parsed.Hunks {
		ids[hunk.DisplayPath] = hunk.ID
		paths, tiers = append(paths, hunk.DisplayPath), append(tiers, "cited")
	}
	line := tierStatusLine(paths, tiers)
	for index, hunk := range parsed.Hunks {
		line = bytes.ReplaceAll(line, []byte(fmt.Sprintf("hunk:sha256:%064x", index+1)), []byte(hunk.ID))
	}
	evidence := []map[string]string{
		{"id": "evidence:x", "path": "internal/x/x.go"},
		{"id": "evidence:plain", "path": "internal/x/plain.go"},
		{"id": "evidence:prefix", "path": "internal/x/widened"},
		{"id": "evidence:named-dir", "path": "x/named.go"},
	}
	// basis maps each hunk path to the evidence IDs it cites.
	write := func(baseRevision string, basis map[string][]string) {
		type citation struct {
			EvidenceID string `json:"evidenceId"`
			Relation   string `json:"relation"`
		}
		hunks := []map[string]any{}
		for _, hunk := range parsed.Hunks {
			cited := []citation{}
			for _, id := range basis[hunk.DisplayPath] {
				cited = append(cited, citation{id, "dependency"})
			}
			hunks = append(hunks, map[string]any{"id": hunk.ID, "path": hunk.DisplayPath, "basis": cited,
				"oldRange": map[string]int{"start": int(hunk.OldRange.Start), "count": int(hunk.OldRange.Count)}})
		}
		document, err := json.Marshal(map[string]any{"baseRevision": baseRevision, "hunks": hunks, "evidence": evidence})
		if err != nil {
			t.Fatal(err)
		}
		writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	}
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: new(bytes.Buffer), base: base, target: target}}
	warned := func(got []string) map[string]bool {
		set := map[string]bool{}
		for _, line := range got {
			for path := range ids {
				if strings.HasPrefix(line, "dogfood-check: WARN ") && strings.Contains(line, fmt.Sprintf("path=%q", path)) {
					set[path] = true
				}
			}
		}
		return set
	}
	samePathNote := func(n int) string {
		return fmt.Sprintf("dogfood-check: NOTE tier-below-anchored-same-path hunks=%d %s", n, tierBelowAnchoredSamePath)
	}
	unanchorableNote := "dogfood-check: NOTE tier-below-anchored-unanchorable hunks=1 " + tierBelowAnchoredUnavailable

	// x.go cites its own file beside another; plain.go cites itself but is
	// unanchorable; named.go cites only another file; widened.go cites a path
	// that only shares a prefix with its own.
	write(base, map[string][]string{
		"internal/x/x.go": {"evidence:plain", "evidence:x"}, "internal/x/plain.go": {"evidence:plain"},
		"internal/x/named.go": {"evidence:plain"}, "internal/x/widened.go": {"evidence:prefix"},
	})
	got := run.tierWarnings(line)
	if w := warned(got); len(got) != 4 || w["internal/x/x.go"] || !w["internal/x/named.go"] || !w["internal/x/widened.go"] || w["internal/x/plain.go"] ||
		got[2] != unanchorableNote || got[3] != samePathNote(1) {
		t.Fatalf("tier lines = %q; want warnings for named.go and widened.go, the unanchorable note, then one same-path note", got)
	}

	// Refusals: each keeps x.go warned.
	for name, basis := range map[string][]string{
		"evidence ID the map does not list": {"evidence:absent"},
		"evidence path is a suffix only":    {"evidence:named-dir"},
		"no basis":                          nil,
	} {
		write(base, map[string][]string{"internal/x/x.go": basis, "internal/x/named.go": {"evidence:named-dir"}})
		if got := run.tierWarnings(line); !warned(got)["internal/x/x.go"] || strings.Contains(strings.Join(got, "\n"), "same-path") {
			t.Fatalf("%s: tier lines = %q; want x.go warned and no same-path note", name, got)
		}
	}
	// A map bound to another base classes nothing: every hunk warns.
	write(strings.Repeat("0", 40), map[string][]string{"internal/x/x.go": {"evidence:x"}})
	if got := run.tierWarnings(line); len(warned(got)) != 4 || strings.Contains(strings.Join(got, "\n"), "NOTE") {
		t.Fatalf("other base: tier lines = %q; want every hunk warned", got)
	}
	// An unparsable map classes nothing.
	writeTree(t, root, map[string]string{".corvint/change.cem.json": "{\n"})
	if got := run.tierWarnings(line); len(warned(got)) != 4 {
		t.Fatalf("unparsable map: tier lines = %q; want every hunk warned", got)
	}
}
