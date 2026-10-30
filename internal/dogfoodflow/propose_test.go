package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// proposeHunk is one hunk of the test map, taken from git's own diff so the
// ranges carry the canonical three context lines.
type proposeHunk struct {
	Path        string `json:"path"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
	OldRange    struct {
		Start int `json:"start"`
		Count int `json:"count"`
	} `json:"oldRange"`
}

var diffHunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@`)

func diffHunks(t *testing.T, root, base string) []proposeHunk {
	t.Helper()
	hunks := []proposeHunk{}
	current := ""
	for _, line := range strings.Split(testGit(t, root, "diff", "--unified=3", "--inter-hunk-context=0", base, "HEAD", "--", ".", ":(exclude).corvint/change.cem.json"), "\n") {
		switch {
		case strings.HasPrefix(line, "--- a/"):
			current = strings.TrimPrefix(line, "--- a/")
		case strings.HasPrefix(line, "+++ b/"):
			current = strings.TrimPrefix(line, "+++ b/")
		case diffHunkHeader.MatchString(line):
			match := diffHunkHeader.FindStringSubmatch(line)
			hunk := proposeHunk{Path: current, Disposition: "unknown", Reason: "no-evidence"}
			hunk.OldRange.Start, _ = strconv.Atoi(match[1])
			hunk.OldRange.Count = 1
			if match[2] != "" {
				hunk.OldRange.Count, _ = strconv.Atoi(match[2])
			}
			hunks = append(hunks, hunk)
		}
	}
	return hunks
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
}

func numbered(prefix string, count int) string {
	var text strings.Builder
	for index := 1; index <= count; index++ {
		fmt.Fprintf(&text, "%s line %d\n", prefix, index)
	}
	return text.String()
}

// DCW-V0-150 (V1-2283): propose-citations prints one draft row per owed hunk
// of the prepared map, in worklist order. Since DCW-V0-439 (V1-3145) a hunk
// whose own file is the only candidate gets an unknown row naming, as a
// review hint, one BASE line that no hunk's old range (context included)
// covers and whose text occurs once after the change, preferring the
// enclosing heading of a document and the enclosing declaration of code; a
// file absent at BASE and a governing file the change edits get unknown rows
// with no hint. Hunks already cited or kept
// unknown with a recorded reason get none. The draft is not uniform, passes
// the plan grammar, is labelled PROPOSED, and nothing is written.
func TestProposeCitationsDCWV0150DraftsPerHunkPlan(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Purpose\n\n" + numbered("purpose", 12) + "\n## Requirements\n\n" + numbered("requirement", 30) + "\n## Non-goals\n\n" + numbered("non-goal", 10)
	code := "package x\n\n// A is first.\nfunc A() int {\n" + numbered("\t// a", 12) + "\treturn 1\n}\n\n// B is second.\nfunc B() int {\n" + numbered("\t// b", 20) + "\treturn 2\n}\n"
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec, "internal/x/x.go": code, "AGENTS.md": "# Agents\n\n" + numbered("rule", 8),
		"docs/decisions/0001-x.md": "# 0001 X\n\n## Decision\n\n" + numbered("decided", 10),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		// Two hunks in the Requirements section, far enough apart to stay two.
		"docs/specs/x-v0.md": strings.Replace(strings.Replace(spec, "requirement line 5\n", "requirement line 5 amended\n", 1), "requirement line 25\n", "requirement line 25 amended\n", 1),
		// Two edits in A, far enough apart to stay two hunks, become the cited and kept hunks.
		"internal/x/x.go":          strings.NewReplacer("\t// a line 2\n", "\t// a line 2 changed\n", "\t// a line 11\n", "\t// a line 11 changed\n", "\t// b line 10\n", "\t// b line 10 changed\n").Replace(code),
		"AGENTS.md":                "# Agents\n\n" + numbered("rule", 8) + "rule line 9\n",
		"internal/x/new.go":        "package x\n\nfunc New() {}\n",
		"docs/decisions/0001-x.md": strings.Replace("# 0001 X\n\n## Decision\n\n"+numbered("decided", 10), "decided line 8\n", "decided line 8 revised\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	// A hunk already cited and one kept unknown with a recorded reason owe no
	// row; both are real patch hunks, so the map is not stale (DCW-V0-382).
	all := diffHunks(t, root, base)
	hunks, ordinals := []proposeHunk{}, []int{}
	settled := 0
	for index, hunk := range all {
		if hunk.Path == "internal/x/x.go" && hunk.OldRange.Start < 20 {
			all[index].Disposition, all[index].Reason = []string{"supported", "unknown"}[settled], []string{"evidence-backed", "insufficient-evidence"}[settled]
			settled++
			continue
		}
		hunks, ordinals = append(hunks, hunk), append(ordinals, index+1)
	}
	if settled != 2 || len(all) != 8 {
		t.Fatalf("fixture has %d hunks, %d in A; want 8 and 2", len(all), settled)
	}
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": all})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	status := testGit(t, root, "status", "--porcelain", "--untracked-files=all")

	var stdout, stderr bytes.Buffer
	code2, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr)
	if err != nil || code2 != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code2, err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dogfood-propose-citations: PROPOSED rows=6 hunks=8 base="+base) ||
		!strings.Contains(stderr.String(), "this draft is not a plan") {
		t.Errorf("stderr does not label the draft PROPOSED:\n%s", stderr.String())
	}
	plan := stdout.Bytes()
	rows := textLines(plan)
	if len(rows) != len(hunks) {
		t.Fatalf("draft has %d rows for %d owed hunks:\n%s", len(rows), len(hunks), plan)
	}
	// No row cites a line of a file the change modifies (DCW-V0-439): every
	// owed hunk here edits its own file, so each row is a reasoned unknown
	// whose note names the enclosing heading or declaration as a review hint.
	hintPattern := regexp.MustCompile(`nearest stable line (\S+) (\d+):(\d+) (\w+) is a review hint only`)
	byPath := map[string]string{}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		if !nonEmptyFields(row, 4) || fields[0] != strconv.Itoa(ordinals[index]) {
			t.Fatalf("row %d %q is not ORDINAL<TAB>...<TAB>...<TAB>...", index+1, row)
		}
		hunk := hunks[index]
		if !unknownRow(fields) || !reasonedUnknownRow(fields) || !strings.HasPrefix(fields[3], "proposed: ") {
			t.Fatalf("row %q cites a line of %s, a file the change modifies (DCW-V0-439)", row, hunk.Path)
		}
		hint := hintPattern.FindStringSubmatch(fields[3])
		if hint == nil {
			byPath[hunk.Path] += "unknown;"
			continue
		}
		first, _ := strconv.Atoi(hint[2])
		last, _ := strconv.Atoi(hint[3])
		if hint[1] != hunk.Path || first != last || !strings.Contains(fields[3], "(DCW-V0-439)") {
			t.Fatalf("row %q does not hint one line of its own hunk's file %s", row, hunk.Path)
		}
		baseLines := splitRecords([]byte(testGit(t, root, "show", base+":"+hunk.Path) + "\n"))
		if first < 1 || first > len(baseLines) {
			t.Fatalf("row %q does not resolve at BASE (%d lines)", row, len(baseLines))
		}
		for _, other := range hunks {
			if other.Path == hunk.Path && first >= other.OldRange.Start && first < other.OldRange.Start+other.OldRange.Count {
				t.Errorf("row %q hints a line inside a hunk's old range %+v", row, other.OldRange)
			}
		}
		after := []byte(testGit(t, root, "show", "HEAD:"+hunk.Path) + "\n")
		if spanOccurrences(after, baseLines[first-1]) != 1 {
			t.Errorf("row %q hints %q, which does not occur exactly once after the change", row, baseLines[first-1])
		}
		byPath[hunk.Path] += hint[4] + ":" + strings.TrimSpace(string(baseLines[first-1])) + ";"
	}
	for name, want := range map[string]string{
		"docs/specs/x-v0.md":       "specification:## Requirements;specification:",
		"internal/x/x.go":          "dependency:func B() int {;",
		"AGENTS.md":                "unknown;",
		"internal/x/new.go":        "unknown;",
		"docs/decisions/0001-x.md": "decision:## Decision;",
	} {
		if !strings.HasPrefix(byPath[name], want) {
			t.Errorf("%s rows = %q; want prefix %q", name, byPath[name], want)
		}
	}
	mapHunkRows := []map[string]string{}
	for range all {
		mapHunkRows = append(mapHunkRows, map[string]string{"id": "hunk"})
	}
	if _, _, uniform := uniformCitation(plan, mapHunkRows[:len(hunks)]); uniform {
		t.Errorf("draft is uniform:\n%s", plan)
	}
	if got := testGit(t, root, "status", "--porcelain", "--untracked-files=all"); got != status {
		t.Errorf("propose-citations changed the worktree: %q -> %q", status, got)
	}
}

// DCW-V0-150: two cite rows with one span are never proposed for every owed
// hunk, so a draft is never refused as citation-plan-uniform (DCW-V0-048).
func TestProposeCitationsDCWV0150NeverDraftsUniformPlan(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	// Only the heading is unique: every other line repeats, so both hunks
	// can only cite it.
	file := "# Only\n" + strings.Repeat("same\n", 20)
	writeTree(t, root, map[string]string{"a.md": file})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	lines := strings.SplitAfter(file, "\n")
	lines[9], lines[18] = "changed one\n", "changed two\n"
	writeTree(t, root, map[string]string{"a.md": strings.Join(lines, "")})
	testGit(t, root, "commit", "-q", "-am", "change")
	hunks := diffHunks(t, root, base)
	if len(hunks) != 2 {
		t.Fatalf("fixture has %d hunks; want 2", len(hunks))
	}
	document, _ := json.Marshal(map[string]any{"baseRevision": base, "hunks": hunks})
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document)})
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	// Both hunks edit a.md, so neither drafts a line of it (DCW-V0-439);
	// the heading is named only as a review hint, and no uniform plan results.
	rows := textLines(stdout.Bytes())
	if len(rows) != 2 {
		t.Fatalf("draft = %q; want two rows", stdout.String())
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if !reasonedUnknownRow(fields) || !strings.Contains(fields[3], "nearest stable line a.md 1:1 specification is a review hint only") {
			t.Errorf("row %q is not an unknown row hinting a.md 1:1", row)
		}
	}
}

// DCW-V0-350 (V1-2810): with no prepared map, propose-citations drafts one
// row per hunk of the canonical BASE..HEAD patch, keyed by full hunk ID, and
// writes nothing; the keys are exactly the hunk IDs cem prepare then records,
// so the plan is written without a probe bind pass.
func TestProposeCitationsDCWV0350DraftsHunkIDTemplateWithoutMap(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	file := "# Title\n\n" + numbered("body", 30)
	writeTree(t, root, map[string]string{"a.md": file, "x.go": "package x\n\n// F is f.\nfunc F() int {\n" + numbered("\t// f", 8) + "\treturn 1\n}\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	changed := strings.Replace(strings.Replace(file, "body line 5\n", "body line 5 amended\n", 1), "body line 25\n", "body line 25 amended\n", 1)
	writeTree(t, root, map[string]string{"a.md": changed, "y.go": "package x\n\nfunc G() {}\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	status := testGit(t, root, "status", "--porcelain", "--untracked-files=all", "--ignored")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "PROPOSED rows=3 hunks=3 base="+base+" map=NOT_PREPARED keyed=hunk-id") ||
		!strings.Contains(stderr.String(), "this draft is not a plan") {
		t.Errorf("stderr does not label the map-free draft:\n%s", stderr.String())
	}
	if after := testGit(t, root, "status", "--porcelain", "--untracked-files=all", "--ignored"); after != status {
		t.Fatalf("the draft wrote files: %q -> %q", status, after)
	}
	keys := map[string]bool{}
	for _, row := range textLines(stdout.Bytes()) {
		key, _, _ := strings.Cut(row, "\t")
		if _, err := strconv.Atoi(key); err == nil || keys[key] {
			t.Fatalf("row is not keyed by a distinct hunk ID: %q\n%s", row, stdout.String())
		}
		keys[key] = true
	}

	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: testGit(t, root, "rev-parse", "HEAD")}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".corvint", "change.cem.json"))
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		Hunks []struct {
			ID string `json:"id"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(data, &prepared); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, hunk := range prepared.Hunks {
		ids[hunk.ID] = true
	}
	if fmt.Sprint(ids) != fmt.Sprint(keys) {
		t.Errorf("template keys %v differ from the prepared map's hunk IDs %v", keys, ids)
	}
}

// DCW-V0-150: an unreadable map or a map prepared against another BASE
// refuses with a fix line and prints no draft; a missing map drafts the
// DCW-V0-350 template instead.
func TestProposeCitationsDCWV0150RefusesWithoutMatchingMap(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"a.txt": "a\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	run := func() (int, string, string) {
		var stdout, stderr bytes.Buffer
		code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, stdout.String(), stderr.String()
	}
	if code, out, errText := run(); code != 0 || out != "" || !strings.Contains(errText, "PROPOSED rows=0 hunks=0 base="+base+" map=NOT_PREPARED keyed=hunk-id") {
		t.Errorf("missing map, empty change: code=%d stdout=%q stderr=%q", code, out, errText)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": "not json"})
	if code, out, errText := run(); code != 2 || out != "" || !strings.Contains(errText, "REFUSE map-unavailable") || !strings.Contains(errText, "fix: run corvint dogfood change "+base) {
		t.Errorf("unreadable map: code=%d stdout=%q stderr=%q", code, out, errText)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": `{"baseRevision": "` + strings.Repeat("0", 40) + `", "hunks": []}`})
	if code, out, errText := run(); code != 2 || out != "" || !strings.Contains(errText, "REFUSE map-base-mismatch") {
		t.Errorf("other base: code=%d stdout=%q stderr=%q", code, out, errText)
	}
}
