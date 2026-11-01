package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// DCW-V0-818 (V1-3623): the citation-anchor search the check runs after its
// SUMMARY line announces itself for a large set and stops at its budget,
// classing no hunk unanchorable so every below-anchored hunk keeps its
// warning; an interrupt of the check itself still interrupts it.
func TestDCWV0818AnchorSearchIsBoundedAndAnnounced(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 20) + "\treturn 1\n}\n"
	}
	count := anchorSearchProgressHunks
	files, changed := map[string]string{}, map[string]string{}
	for i := range count {
		name := fmt.Sprintf("P%02d", i)
		path := fmt.Sprintf("internal/x/p%02d.go", i)
		files[path] = source(name)
		changed[path] = strings.Replace(source(name), "\t// "+name+" line 10\n", "\t// "+name+" line 10 changed\n", 1)
	}
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, changed)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != count {
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
	line := tierStatusLine(paths, tiers)
	for index, hunk := range parsed.Hunks {
		line = bytes.ReplaceAll(line, []byte(fmt.Sprintf("hunk:sha256:%064x", index+1)), []byte(hunk.ID))
	}
	announce := fmt.Sprintf("dogfood-check: NOTE tier-anchor-search hunks=%d budget=", count)
	exceeded := fmt.Sprintf("dogfood-check: NOTE tier-anchor-search NOT_OBSERVED budget-exceeded hunks=%d;", count)

	// Within budget: announced, and every hunk (no BASE line names it) is unanchorable.
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: &stderr, base: base, target: target}}
	if got := run.unanchorableHunks(line); len(got) != count || !strings.HasPrefix(stderr.String(), announce) || strings.Contains(stderr.String(), "budget-exceeded") {
		t.Fatalf("within budget: %d unanchorable, stderr %q", len(got), stderr.String())
	}

	// Over budget: none classed, so every hunk keeps its warning, and the NOTE says why.
	saved := anchorSearchBudget
	t.Cleanup(func() { anchorSearchBudget = saved })
	anchorSearchBudget = time.Nanosecond
	stderr.Reset()
	if got := run.unanchorableHunks(line); got != nil || !strings.Contains(stderr.String(), exceeded) {
		t.Fatalf("over budget: %v, stderr %q", got, stderr.String())
	}
	stderr.Reset()
	if got := run.tierWarnings(line); len(got) != count || strings.Contains(strings.Join(got, "\n"), "unanchorable") {
		t.Fatalf("over budget tier lines: %q; want one warning per hunk", got)
	}

	// An interrupt of the check itself is not a budget stop: it unwinds as one.
	anchorSearchBudget = saved
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	run.ctx = cancelled
	stderr.Reset()
	func() {
		defer func() {
			if unwound, ok := recover().(stop); !ok || unwound.code != -1 {
				t.Fatalf("interrupted search: recovered %v; want stop{-1}", unwound)
			}
		}()
		run.unanchorableHunks(line)
	}()
	if strings.Contains(stderr.String(), "budget-exceeded") {
		t.Fatalf("interrupt reported as a budget stop: %q", stderr.String())
	}
}
