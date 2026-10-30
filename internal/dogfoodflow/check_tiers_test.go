package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// tierStatusLine is a stable `cem status` line whose map declares the given
// paths and whose recomputed citation block gives them the given tiers.
func tierStatusLine(paths, tiers []string) []byte {
	hunks, rows := []string{}, []string{}
	for i, path := range paths {
		id := fmt.Sprintf("hunk:sha256:%064x", i+1)
		hunks = append(hunks, fmt.Sprintf(`{"id":%q,"path":%q}`, id, path))
		rows = append(rows, fmt.Sprintf(`{"hunkId":%q,"tier":%q,"anchors":[]}`, id, tiers[i]))
	}
	return []byte(`{"ok":true,"state":"accepted","tool":"cem-status","stableVerification":{"outcome":"ACCEPT","hunks":[` +
		strings.Join(hunks, ",") + `],"citation":{"profile":"cem-citation/1","hunks":[` + strings.Join(rows, ",") + `]}}}` + "\n")
}

// DCW-V0-112 (V1-1760): a docs-only change bound with a non-anchoring
// citation (tier `cited`) is named before seal with its path and the CEM-V1-081
// rule it misses; an anchoring citation, and a line with no citation block,
// produce no warning; the warnings are bounded.
func TestDCWV0112CheckWarnsOnHunksBelowAnchored(t *testing.T) {
	got := belowAnchoredWarnings(tierStatusLine([]string{"docs/build-log/entry.md", "docs/specs/x.md", "a.go"}, []string{"cited", "anchored", "unknown"}), nil)
	if len(got) != 2 ||
		got[0] != `dogfood-check: WARN tier-below-anchored path="docs/build-log/entry.md" hunk="hunk:sha256:`+fmt.Sprintf("%064x", 1)+`" tier=cited `+tierBelowAnchoredFix ||
		!strings.HasPrefix(got[1], `dogfood-check: WARN tier-below-anchored path="a.go" `) || !strings.Contains(got[1], " tier=unknown rule=CEM-V1-081 ") {
		t.Fatalf("warnings = %q", got)
	}
	for name, line := range map[string][]byte{
		"anchored":    tierStatusLine([]string{"docs/build-log/entry.md"}, []string{"anchored"}),
		"mechanical":  tierStatusLine([]string{"a.go"}, []string{"mechanical-proved"}),
		"no-citation": []byte(`{"ok":true,"stableVerification":{"outcome":"ACCEPT","citation":null}}`),
		"not-json":    []byte("not json\n"),
	} {
		if got := belowAnchoredWarnings(line, nil); len(got) != 0 {
			t.Errorf("%s: warnings = %q, want none", name, got)
		}
	}
	paths, tiers := []string{}, []string{}
	for i := range maxTierWarnings + 3 {
		paths, tiers = append(paths, fmt.Sprintf("p%d\nforged", i)), append(tiers, "cited")
	}
	got = belowAnchoredWarnings(tierStatusLine(paths, tiers), nil)
	if len(got) != maxTierWarnings+1 || got[maxTierWarnings] != "dogfood-check: WARN tier-below-anchored more=3" || strings.Contains(strings.Join(got, ""), "\n") {
		t.Fatalf("bounded warnings: %d lines, last %q", len(got), got[len(got)-1])
	}
}

// DCW-V0-165 (V1-2367, V1-2379): a hunk cited from an unchanged file that no
// stable BASE line could anchor (none defines an ID it adds, none names its
// path or its basename of 6 bytes or more) draws no tier warning, only one
// NOTE count; a hunk cited at a weaker tier although an anchoring BASE line
// exists still warns. Without a readable bound map every below-anchored hunk
// keeps its warning.
func TestDCWV0165CheckWarnsOnlyWhereAnAnchorIsAvailable(t *testing.T) {
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
	writeTree(t, root, map[string]string{
		"internal/x/x.go":       strings.Replace(source("X"), "\t// X line 10\n", "\t// X line 10 frobs, as XYZ-V0-001 requires\n", 1),
		"internal/x/named.go":   strings.Replace(source("Named"), "\t// Named line 10\n", "\t// Named line 10 changed\n", 1),
		"internal/x/plain.go":   strings.Replace(source("Plain"), "\t// Plain line 10\n", "\t// Plain line 10 changed\n", 1),
		"internal/x/widened.go": strings.Replace(source("Widened"), "\t// Widened line 10\n", "\t// Widened line 10 changed\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != 4 {
		t.Fatalf("canonical patch: %v (%d hunks)", err, len(parsed.Hunks))
	}
	type mapHunk struct {
		ID       string         `json:"id"`
		Path     string         `json:"path"`
		OldRange map[string]int `json:"oldRange"`
	}
	mapped, paths, tiers := []mapHunk{}, []string{}, []string{}
	for _, hunk := range parsed.Hunks {
		mapped = append(mapped, mapHunk{hunk.ID, hunk.DisplayPath, map[string]int{"start": int(hunk.OldRange.Start), "count": int(hunk.OldRange.Count)}})
		paths, tiers = append(paths, hunk.DisplayPath), append(tiers, "cited")
	}
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": mapped})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	// The status line names the map's real hunk IDs, every hunk at `cited`.
	line := tierStatusLine(paths, tiers)
	for index, hunk := range parsed.Hunks {
		line = bytes.ReplaceAll(line, []byte(fmt.Sprintf("hunk:sha256:%064x", index+1)), []byte(hunk.ID))
	}
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: &stderr, base: base, target: target}}
	got := run.tierWarnings(line)
	joined := strings.Join(got, "\n")
	warned := strings.Join(got[:min(len(got), 3)], "\n")
	if len(got) != 4 || !strings.Contains(warned, `path="internal/x/x.go"`) || !strings.Contains(warned, `path="internal/x/named.go"`) ||
		!strings.Contains(warned, `path="internal/x/widened.go"`) || strings.Contains(joined, `path="internal/x/plain.go"`) ||
		got[3] != "dogfood-check: NOTE tier-below-anchored-unanchorable hunks=1 "+tierBelowAnchoredUnavailable || stderr.Len() != 0 {
		t.Fatalf("tier lines = %q (stderr %q); want warnings for x.go, named.go and widened.go and one NOTE for plain.go", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(root, ".corvint", "change.cem.json")); err != nil {
		t.Fatal(err)
	}
	if got := run.tierWarnings(line); len(got) != 4 || strings.Contains(strings.Join(got, "\n"), "NOTE") {
		t.Fatalf("without a bound map: %q; want every hunk warned", got)
	}
}
