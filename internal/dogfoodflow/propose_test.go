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

	"github.com/Beamfall/corvint/internal/cem/cemcode"
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

// proposalRow reports a draft row stderr marks as a DCW-V0-962 proposal.
func proposalRow(stderr, row string) bool {
	return strings.Contains(stderr, "  drafted proposal: "+row+"\n")
}

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
// of the prepared map, in worklist order. Each cite row names one BASE line
// of a file the change leaves untouched that names the hunk's path, and no
// two rows share a span; the hunk's own modified file is never cited, so a
// hunk with no such anchor, a file absent at BASE and a governing file the
// change edits get unknown rows (DCW-V0-393). Hunks already cited or kept
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
		// The one file the change leaves untouched: it names two changed paths.
		"docs/specs/README.md": "# Specs\n\n- docs/specs/x-v0.md is the X contract.\n- internal/x/x.go implements it.\n",
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
	changed := map[string]bool{}
	settled := 0
	for index, hunk := range all {
		changed[hunk.Path] = true
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
	byPath := map[string]string{}
	spans := map[string]bool{}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		if !nonEmptyFields(row, 4) || fields[0] != strconv.Itoa(ordinals[index]) {
			t.Fatalf("row %d %q is not ORDINAL<TAB>...<TAB>...<TAB>...", index+1, row)
		}
		hunk := hunks[index]
		if unknownRow(fields) {
			if !reasonedUnknownRow(fields) || !strings.HasPrefix(fields[3], "proposed: ") {
				t.Errorf("row %q is not a reasoned proposed unknown row", row)
			}
			byPath[hunk.Path] += "unknown;"
			continue
		}
		// DCW-V0-962: only a marked proposal cites a modified file, its own.
		if proposalRow(stderr.String(), row) && fields[1] == hunk.Path {
			byPath[hunk.Path] += "proposal;"
			continue
		}
		if changed[fields[1]] {
			t.Errorf("row %q cites a file the change modifies", row)
		}
		start, end, _ := strings.Cut(fields[2], ":")
		first, _ := strconv.Atoi(start)
		last, _ := strconv.Atoi(end)
		baseLines := splitRecords([]byte(testGit(t, root, "show", base+":"+fields[1]) + "\n"))
		if first != last || first < 1 || first > len(baseLines) {
			t.Fatalf("row %q does not cite one BASE line (%d lines)", row, len(baseLines))
		}
		if !strings.Contains(string(baseLines[first-1]), hunk.Path) {
			t.Errorf("row %q cites %q, which does not name %s", row, baseLines[first-1], hunk.Path)
		}
		if spans[fields[1]+":"+fields[2]] {
			t.Errorf("row %q repeats a span", row)
		}
		spans[fields[1]+":"+fields[2]] = true
		byPath[hunk.Path] += fields[1] + ";"
	}
	for name, want := range map[string]string{
		"docs/specs/x-v0.md":       "docs/specs/README.md;proposal;",
		"internal/x/x.go":          "docs/specs/README.md;",
		"AGENTS.md":                "unknown;",
		"internal/x/new.go":        "unknown;",
		"docs/decisions/0001-x.md": "proposal;",
	} {
		if byPath[name] != want {
			t.Errorf("%s rows = %q; want %q", name, byPath[name], want)
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
	// One untouched line names the changed file, so only one hunk can cite it;
	// index.md has two lines, since a one-line file is never cited (DCW-V0-440).
	file := "# Only\n" + strings.Repeat("same\n", 20)
	writeTree(t, root, map[string]string{"docs/alpha.md": file, "index.md": "# Index\nSee docs/alpha.md.\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	lines := strings.SplitAfter(file, "\n")
	lines[9], lines[18] = "changed one\n", "changed two\n"
	writeTree(t, root, map[string]string{"docs/alpha.md": strings.Join(lines, "")})
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
	rows := textLines(stdout.Bytes())
	// DCW-V0-962: the second hunk takes its own file's heading as a marked
	// proposal, a second span, so the draft is not uniform.
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "1\tindex.md\t2:2\t") || rows[1] != "2\tdocs/alpha.md\t1:1\tspecification" || !proposalRow(stderr.String(), rows[1]) {
		t.Errorf("draft = %q; want one cite of index.md:2 and one marked proposal", stdout.String())
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

// DCW-V0-394 (V1-3204): a missing map, an unreadable map and a map prepared
// against another BASE all draft the DCW-V0-350 template from the canonical
// patch, keyed by hunk ID, so committing before the first dogfood change works.
func TestProposeCitationsDCWV0394DraftsWithoutUsableMap(t *testing.T) {
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
	// Commit first, then propose: the committed hunk is drafted by its ID.
	writeTree(t, root, map[string]string{"a.txt": "a changed\n"})
	testGit(t, root, "commit", "-q", "-am", "change")
	hunkRow := regexp.MustCompile(`^hunk:sha256:[0-9a-f]{64}\tunknown\tinsufficient-evidence\tproposed: \S`)
	for _, state := range []struct{ name, document, note string }{
		{"UNREADABLE", "not json", "NOTE map-unreadable"},
		{"OTHER_BASE", `{"baseRevision": "` + strings.Repeat("0", 40) + `", "hunks": []}`, "NOTE map-base-mismatch"},
	} {
		writeTree(t, root, map[string]string{".corvint/change.cem.json": state.document})
		code, out, errText := run()
		rows := textLines([]byte(out))
		if code != 0 || len(rows) != 1 || !hunkRow.MatchString(rows[0]) || !strings.Contains(errText, state.note) ||
			!strings.Contains(errText, "PROPOSED rows=1 hunks=1 base="+base+" map="+state.name+" keyed=hunk-id") || strings.Contains(errText, "REFUSE") {
			t.Errorf("%s map: code=%d stdout=%q stderr=%q", state.name, code, out, errText)
		}
	}
}

// DCW-V0-393 (V1-3144), amended by DCW-V0-962 and DCW-V0-966: a draft never
// cites the change's own text: a modified file lends only a BASE line no hunk
// edits, and an added file's row names the runner receipt that carries it to
// the verdict gate (DCW-V0-395).
func TestProposeCitationsDCWV0393NeverCitesChangedPaths(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# P v0\n\n## Requirements\n\n- internal/p/p.go answers.\n" + numbered("requirement", 10)
	writeTree(t, root, map[string]string{"docs/specs/p-v0.md": spec, "internal/p/p.go": "package p\n\nfunc P() int { return 1 }\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/p-v0.md":   spec + "- internal/p/p_test.go proves internal/p/p.go.\n",
		"internal/p/p.go":      "package p\n\nfunc P() int { return 2 }\n",
		"internal/p/p_test.go": "package p\n\n// Tests internal/p/p.go.\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 3 {
		t.Fatalf("draft has %d rows; want 3:\n%s", len(rows), stdout.String())
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		// DCW-V0-962: the spec hunk's own heading is a marked proposal, and
		// DCW-V0-966 anchors p.go on the spec's unedited BASE line naming it.
		if fields[1] == "docs/specs/p-v0.md" && proposalRow(stderr.String(), row) || row == fields[0]+"\tdocs/specs/p-v0.md\t5:5\tspecification" {
			continue
		}
		if !unknownRow(fields) || !reasonedUnknownRow(fields) {
			t.Errorf("row %q cites a changed file; want an unknown row or its own marked proposal", row)
		}
	}
	if !strings.Contains(stdout.String(), "internal/p/p_test.go is absent at BASE") || !strings.Contains(stdout.String(), "runner receipt") {
		t.Errorf("the added file's row does not name the runner receipt:\n%s", stdout.String())
	}
}

// DCW-V0-393: the changed-path inventory comes from git's name list, so a
// mode-only change to a file the patch parser rejects still keeps that file
// out of the draft.
func TestProposeCitationsDCWV0393ModeOnlyChangeIsChanged(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "core.fileMode", "true")
	writeTree(t, root, map[string]string{
		"docs/specs/p-v0.md": "# P v0\n\n- internal/p/p.go answers.\n",
		"internal/p/p.go":    "package p\n\nfunc P() int { return 1 }\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"internal/p/p.go": "package p\n\nfunc P() int { return 2 }\n"})
	testGit(t, root, "commit", "-q", "-am", "change")
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": diffHunks(t, root, base)})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document)})
	testGit(t, root, "update-index", "--chmod=+x", "docs/specs/p-v0.md")
	testGit(t, root, "commit", "-q", "-m", "mode-only contract change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	if strings.Contains(stdout.String(), "\tdocs/specs/p-v0.md\t") {
		t.Errorf("draft cites a file the change made executable:\n%s", stdout.String())
	}
}

// TestProposeCitationsDCWV0802NamesWhyThePatchIsUnavailable pins V1-3279: a
// map-free draft whose canonical patch cem/1.0 cannot represent refuses with
// the code cem prepare refuses the same patch with, never advises running
// dogfood change first, and names the first-run order and the inputs each
// later command needs; a patch over the read bound names the split.
func TestProposeCitationsDCWV0802NamesWhyThePatchIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		change       func(t *testing.T, root string)
		limit        int
	}{
		{name: "empty-file", reason: "metadata-only-change-unsupported", change: func(t *testing.T, root string) {
			writeTree(t, root, map[string]string{"a.md": "# Title\n\nbody amended\n", "empty.txt": ""})
		}},
		{name: "mode-only", reason: "metadata-only-change-unsupported", change: func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "a.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "over-read-bound", reason: "git-output-exceeded", limit: 64, change: func(t *testing.T, root string) {
			writeTree(t, root, map[string]string{"a.md": "# Title\n\n" + numbered("body amended", 20)})
		}},
		// A textual patch between cem prepare's 8 MiB Git output bound and
		// the flow's larger read bound is refused as cem prepare refuses it,
		// with the split guidance, never as a binary patch.
		{name: "over-prepare-bound", reason: "git-output-exceeded", change: func(t *testing.T, root string) {
			writeTree(t, root, map[string]string{"large.txt": strings.Repeat(strings.Repeat("x", 99)+"\n", 9<<20/100)})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			testGit(t, root, "config", "core.fileMode", "true")
			writeTree(t, root, map[string]string{"a.md": "# Title\n\nbody\n"})
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "base")
			base := testGit(t, root, "rev-parse", "HEAD")
			tc.change(t, root)
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "change")
			status := testGit(t, root, "status", "--porcelain", "--untracked-files=all", "--ignored")

			var stdout, stderr bytes.Buffer
			run := &proposal{
				flow:    flow{ctx: context.Background(), prefix: "dogfood-propose-citations", root: root, stderr: &stderr, gitOutputLimit: tc.limit},
				options: ProposeOptions{Root: root, Base: base}, stdout: &stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{},
			}
			if code, err := boundary(context.Background(), run.run, func() {}); err != nil || code != 2 {
				t.Fatalf("ProposeCitations = %d, %v, want exit 2\n%s", code, err, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("a refused draft printed rows: %q", stdout.String())
			}
			for _, want := range []string{
				"dogfood-propose-citations: REFUSE patch-unavailable reason=" + tc.reason + "\n",
				"propose-citations is the first command of the closeout and needs only BASE and a committed HEAD; no prepared map, DOGFOOD_INTENTS_FILE, DOGFOOD_VERIFY_FILE or DOGFOOD_OUTCOME",
				"make dogfood-inputs-check BASE=" + base + " CITATIONS=plan.tsv INTENTS=<intents.txt> VERIFY=<verify.txt> OUTCOME=passed|failed|blocked",
				"then rerun corvint dogfood propose-citations " + base + "\n",
			} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
			if tc.reason == "git-output-exceeded" && (!strings.Contains(stderr.String(), "split the change into smaller commits") || strings.Contains(stderr.String(), "textual hunk")) {
				t.Errorf("an oversized patch lacks the split guidance or names the binary-patch fix:\n%s", stderr.String())
			}
			if strings.Contains(stderr.String(), "run corvint dogfood change") {
				t.Errorf("the refusal advises dogfood change first, which refuses the same patch:\n%s", stderr.String())
			}
			if after := testGit(t, root, "status", "--porcelain", "--untracked-files=all", "--ignored"); after != status {
				t.Fatalf("the refusal wrote files: %q -> %q", status, after)
			}
			if tc.limit != 0 {
				return
			}
			// cem prepare, which dogfood change runs, refuses the same patch
			// with the same code, so the refusal's advice is the only route.
			session, err := workflow.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = session.Prepare(context.Background(), workflow.PrepareOptions{Base: base, Target: testGit(t, root, "rev-parse", "HEAD")})
			if code := cemcode.CodeOf(err); code != tc.reason {
				t.Fatalf("cem prepare = %v (code %q), want %s", err, code, tc.reason)
			}
		})
	}
}

// DCW-V0-920 (V1-3043): a BASE line longer than cem cite's evidence span
// bound, such as the only line of a large one-line JSON document, is never
// drafted as a cite, and the draft names the span it skipped and why; the same
// line within the bound is cited.
func TestProposeCitationsDCWV0920SkipsSpansOverTheCiteBound(t *testing.T) {
	for _, tc := range []struct {
		name    string
		padding int
		cited   bool
	}{{"within-bound", 64, true}, {"over-bound", 1 << 20, false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			spec := "# P v0\n\n## Requirements\n\n- internal/p/p.go answers " + strings.Repeat("a", tc.padding) + "\n" + numbered("requirement", 10)
			writeTree(t, root, map[string]string{"docs/specs/p-v0.md": spec, "internal/p/p.go": "package p\n\nfunc P() int { return 1 }\n"})
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "base")
			base := testGit(t, root, "rev-parse", "HEAD")
			writeTree(t, root, map[string]string{"internal/p/p.go": "package p\n\nfunc P() int { return 2 }\n"})
			testGit(t, root, "commit", "-q", "-am", "change")
			var stdout, stderr bytes.Buffer
			if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
				t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
			}
			out := stdout.String() + stderr.String()
			cites := strings.Contains(stdout.String(), "\tdocs/specs/p-v0.md\t5:5\t")
			skipped := strings.Contains(out, "docs/specs/p-v0.md:5:5 not cited: its BASE line is") && strings.Contains(out, "over the 1048576-byte evidence span cem cite admits (DCW-V0-920)")
			if cites != tc.cited || skipped == tc.cited {
				t.Fatalf("cites=%v skipped=%v, want cited=%v:\n%.2000s", cites, skipped, tc.cited, out)
			}
		})
	}
}

// TestProposeCitationsDCWV0963UnknownRowsNameHunkPath (V1-3270): every
// unknown row names, in its note, the path and BASE range its hunk edits, so
// the stdout draft alone says which file each row stands for: here a one-line
// file (never proposed) and a file the change adds.
func TestProposeCitationsDCWV0963UnknownRowsNameHunkPath(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"one.txt": "one\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"one.txt": "two\n", "internal/n/new.go": "package n\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	want := map[string]bool{"proposed: hunk edits internal/n/new.go at BASE 0,0 HEAD 1,1; ": false, "proposed: hunk edits one.txt at BASE 1,1 HEAD 1,1; ": false}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if !reasonedUnknownRow(fields) {
			t.Fatalf("row %q is not a reasoned unknown row", row)
		}
		for prefix := range want {
			want[prefix] = want[prefix] || strings.HasPrefix(fields[3], prefix)
		}
	}
	for prefix, seen := range want {
		if !seen || len(rows) != 2 {
			t.Errorf("no row's note starts %q:\n%s", prefix, stdout.String())
		}
	}
}
