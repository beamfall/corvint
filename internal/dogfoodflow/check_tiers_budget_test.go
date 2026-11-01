package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// anchorBudgetFixture builds a change of anchorSearchProgressHunks one-line
// hunks, each in its own Go file, plus the BASE files extra. Its bound map
// gives hunk 0 a basis citing evidence on its own path, so a same-path
// classification would quiet it (DCW-V0-388).
func anchorBudgetFixture(t *testing.T, extra map[string]string) (root, base, target string, line []byte, count int) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q")
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 20) + "\treturn 1\n}\n"
	}
	count = anchorSearchProgressHunks
	files, changed := map[string]string{}, map[string]string{}
	for path, text := range extra {
		files[path] = text
	}
	for i := range count {
		name := fmt.Sprintf("P%02d", i)
		path := fmt.Sprintf("internal/x/p%02d.go", i)
		files[path] = source(name)
		changed[path] = strings.Replace(source(name), "\t// "+name+" line 10\n", "\t// "+name+" line 10 changed\n", 1)
	}
	writeTree(t, root, files)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, changed)
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	target = testGit(t, root, "rev-parse", "HEAD")
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != count {
		t.Fatalf("canonical patch: %v (%d hunks)", err, len(parsed.Hunks))
	}
	type mapHunk struct {
		ID       string              `json:"id"`
		Path     string              `json:"path"`
		OldRange map[string]int      `json:"oldRange"`
		Basis    []map[string]string `json:"basis,omitempty"`
	}
	mapped, paths, tiers := []mapHunk{}, []string{}, []string{}
	for index, hunk := range parsed.Hunks {
		entry := mapHunk{ID: hunk.ID, Path: hunk.DisplayPath, OldRange: map[string]int{"start": int(hunk.OldRange.Start), "count": int(hunk.OldRange.Count)}}
		if index == 0 {
			entry.Basis = []map[string]string{{"evidenceId": "ev-same"}}
		}
		mapped = append(mapped, entry)
		paths, tiers = append(paths, hunk.DisplayPath), append(tiers, "cited")
	}
	evidence := []map[string]string{{"id": "ev-same", "path": parsed.Hunks[0].DisplayPath}}
	document, err := json.Marshal(map[string]any{"baseRevision": base, "evidence": evidence, "hunks": mapped})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	line = tierStatusLine(paths, tiers)
	for index, hunk := range parsed.Hunks {
		line = bytes.ReplaceAll(line, []byte(fmt.Sprintf("hunk:sha256:%064x", index+1)), []byte(hunk.ID))
	}
	return root, base, target, line, count
}

// DCW-V0-818 (V1-3623): the citation-anchor search the check runs after its
// SUMMARY line announces itself for a large set and stops at its budget,
// classing no hunk at all, by anchor or by same-path basis, so every
// below-anchored hunk keeps its warning; an interrupt of the check itself
// still interrupts it, before the search and between hunks.
func TestDCWV0818AnchorSearchIsBoundedAndAnnounced(t *testing.T) {
	root, base, target, line, count := anchorBudgetFixture(t, nil)
	announce := fmt.Sprintf("dogfood-check: NOTE tier-anchor-search hunks=%d budget=", count)
	exceeded := fmt.Sprintf("dogfood-check: NOTE tier-anchor-search NOT_OBSERVED budget-exceeded hunks=%d;", count)

	// Within budget: announced, and every hunk (no BASE line names it) is unanchorable.
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: &stderr, base: base, target: target}}
	if got, classified := run.unanchorableHunks(line); !classified || len(got) != count || !strings.HasPrefix(stderr.String(), announce) || strings.Contains(stderr.String(), "budget-exceeded") {
		t.Fatalf("within budget: %d unanchorable, stderr %q", len(got), stderr.String())
	}

	// Over budget: none classed, so every hunk keeps its warning, and the NOTE says why.
	saved := anchorSearchBudget
	t.Cleanup(func() { anchorSearchBudget = saved })
	anchorSearchBudget = time.Nanosecond
	stderr.Reset()
	if got, classified := run.unanchorableHunks(line); got != nil || classified || !strings.Contains(stderr.String(), exceeded) {
		t.Fatalf("over budget: %v classified=%v, stderr %q", got, classified, stderr.String())
	}
	stderr.Reset()
	// Hunk 0's same-path basis is not a reason to quiet it past the budget.
	if got := run.tierWarnings(line); len(got) != count || strings.Contains(strings.Join(got, "\n"), "unanchorable") || strings.Contains(strings.Join(got, "\n"), "same-path") {
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

	// An interrupt between hunks unwinds too, rather than returning a
	// classification the caller would follow with PASS.
	live, interrupt := context.WithCancel(context.Background())
	defer interrupt()
	run.ctx = live
	savedHook := anchorSearchBeforeHunk
	t.Cleanup(func() { anchorSearchBeforeHunk = savedHook })
	anchorSearchBeforeHunk = interrupt
	stderr.Reset()
	func() {
		defer func() {
			if unwound, ok := recover().(stop); !ok || unwound.code != -1 {
				t.Fatalf("search interrupted between hunks: recovered %v; want stop{-1}", unwound)
			}
		}()
		got, classified := run.unanchorableHunks(line)
		t.Fatalf("search interrupted between hunks returned %v classified=%v", got, classified)
	}()
}

// DCW-V0-818: the deadline reaches the anchor search's own candidate loop,
// so a large candidate set cannot run past the budget between Git reads.
func TestDCWV0818AnchorCandidateLoopHonoursDeadline(t *testing.T) {
	names := "internal/x/p00.go is named here.\n"
	root, base, target, _, _ := anchorBudgetFixture(t, map[string]string{"docs/notes.md": names})
	data, err := readPrefix(root+"/.corvint/change.cem.json", maxPlanBytes)
	if err != nil {
		t.Fatal(err)
	}
	_, hunks, ok := preparedHunks(data)
	if !ok {
		t.Fatal("fixture map unreadable")
	}
	live := flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: io.Discard, base: base, target: target}
	search := &proposal{flow: live, blobs: map[string][]byte{}, lines: map[string][][]byte{}, basenames: true}
	if !search.prepareAnchors(hunks[:1]) {
		t.Fatal("prepareAnchors did not complete")
	}
	if row, _ := search.anchoredProposal(hunks[0], "", editedRanges(hunks), map[string]bool{}); row == "" {
		t.Fatal("fixture: no anchor for hunk 0 within budget")
	}
	expired, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-expired.Done()
	search.ctx = expired
	defer func() {
		if unwound, ok := recover().(stop); !ok || unwound.code != -1 {
			t.Fatalf("candidate loop past its deadline: recovered %v; want stop{-1}", unwound)
		}
	}()
	row, _ := search.anchoredProposal(hunks[0], "", editedRanges(hunks), map[string]bool{})
	t.Fatalf("candidate loop past its deadline returned %q", row)
}
