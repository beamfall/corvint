package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hunkPaths maps each row's selector to the path its hunk edits, from the
// draft's stderr listing.
func hunkPaths(rows []string, stderr string) map[string]string {
	paths := map[string]string{}
	for _, row := range rows {
		id, _, _ := strings.Cut(row, "\t")
		for _, line := range strings.Split(stderr, "\n") {
			if _, rest, ok := strings.Cut(line, "hunk "+id+" edits "); ok {
				name, _, _ := strings.Cut(rest, " at BASE ")
				paths[id] = name
			}
		}
	}
	return paths
}

// TestProposeCitationsDCWV0842RefreshAcrossAMainlineMerge pins V1-3814 and
// V1-3791: a lane drafts its plan at BASE B1, adds a hunk, merges a mainline
// commit B2 that shifts the cited lines and the edited files, and refreshes
// against B2. Every row is re-keyed with its content unchanged, so no NOTE is
// marked stale and none is dropped; the cite row is re-pinned to the moved
// line; the new hunk of the file that carries a citation is proposed that
// citation, carried forward, while the new hunk of a file with no carried
// citation is drafted afresh; and the refreshed plan passes the precheck.
func TestProposeCitationsDCWV0842RefreshAcrossAMainlineMerge(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	spec := "# Widget\n\n" + numbered("intro", 2) + "The widget contract lives in a.go.\n" + numbered("outro", 4)
	source := func(name string) string {
		return "package w\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 40) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{"docs/spec.md": spec, "a.go": source("A"), "b.go": source("B")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	b1 := testGit(t, root, "rev-parse", "HEAD")
	mainline := testGit(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	testGit(t, root, "checkout", "-q", "-b", "lane")
	laneA := strings.NewReplacer("\t// A line 12\n", "\t// A line 12 lane\n", "\t// A line 24\n", "\t// A line 24 lane\n").Replace(source("A"))
	laneB := strings.Replace(source("B"), "\t// B line 12\n", "\t// B line 12 lane\n", 1)
	writeTree(t, root, map[string]string{"a.go": laneA, "b.go": laneB})
	testGit(t, root, "commit", "-q", "-am", "lane")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b1}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	drafted := textLines(stdout.Bytes())
	paths := hunkPaths(drafted, stderr.String())
	plan, cited := []string{}, ""
	for _, row := range drafted {
		id, _, _ := strings.Cut(row, "\t")
		if paths[id] == "a.go" && cited == "" {
			cited = id
			plan = append(plan, id+"\tdocs/spec.md\t5:5\tspecification")
			continue
		}
		plan = append(plan, id+"\tunknown\tinsufficient-evidence\tthe lane note for "+paths[id])
	}
	if len(plan) != 3 || cited == "" {
		t.Fatalf("lane draft = %q", drafted)
	}
	planPath := writeInput(t, strings.Join(plan, "\n")+"\n")

	// A later lane commit adds a hunk to each file; mainline then shifts
	// every file by three lines far from the lane's edits.
	writeTree(t, root, map[string]string{
		"a.go": strings.Replace(laneA, "\t// A line 36\n", "\t// A line 36 later\n", 1),
		"b.go": strings.Replace(laneB, "\t// B line 36\n", "\t// B line 36 later\n", 1),
	})
	testGit(t, root, "commit", "-q", "-am", "lane later")
	testGit(t, root, "checkout", "-q", mainline)
	shift := "// mainline one\n// mainline two\n// mainline three\n"
	writeTree(t, root, map[string]string{
		"docs/spec.md": "# Widget\n\n" + shift + strings.TrimPrefix(spec, "# Widget\n\n"),
		"a.go":         "package w\n\n" + shift + strings.TrimPrefix(source("A"), "package w\n\n"),
		"b.go":         "package w\n\n" + shift + strings.TrimPrefix(source("B"), "package w\n\n"),
	})
	testGit(t, root, "commit", "-q", "-am", "mainline")
	b2 := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "-q", "lane")
	testGit(t, root, "merge", "-q", "--no-edit", mainline)

	// The map the last bind prepared still names B1.
	writeTree(t, root, map[string]string{".corvint/change.cem.json": `{"baseRevision":"` + b1 + `","hunks":[]}` + "\n"})
	stdout.Reset()
	stderr.Reset()
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b2, Refresh: planPath}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, stderr.String())
	}
	if err := os.Remove(filepath.Join(root, ".corvint", "change.cem.json")); err != nil {
		t.Fatal(err)
	}
	output := stdout.String() + stderr.String()
	if strings.Contains(output, "[stale:") || strings.Contains(output, "marked stale") || strings.Contains(output, " dropped:") || !strings.Contains(output, " dropped=0 ") {
		t.Errorf("a content-unchanged row was marked stale or dropped:\n%s", output)
	}
	if strings.Count(stderr.String(), "content unchanged") != 3 {
		t.Errorf("want three content-unchanged re-keys:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "docs/spec.md 5:5 re-pinned to 8:8") {
		t.Errorf("the cite row was not re-pinned to the moved line:\n%s", stderr.String())
	}
	refreshed := textLines(stdout.Bytes())
	now := hunkPaths(refreshed, stderr.String())
	forward, fresh := 0, 0
	for _, row := range refreshed {
		id, rest, _ := strings.Cut(row, "\t")
		switch {
		case strings.Contains(rest, "the lane note for "):
		case rest == "docs/spec.md\t8:8\tspecification" && proposalRow(stderr.String(), row):
			if now[id] != "a.go" || !strings.Contains(stderr.String(), "carried forward: --refresh carried no row to this hunk") {
				t.Errorf("carried-forward row %q is not the new a.go hunk:\n%s", row, stderr.String())
			}
			forward++
		case rest == "docs/spec.md\t8:8\tspecification":
		default:
			if now[id] != "b.go" {
				t.Errorf("unexpected row %q for %s", row, now[id])
			}
			fresh++
		}
	}
	if forward != 1 || fresh != 1 || len(refreshed) != 5 {
		t.Errorf("refreshed plan: %d carried forward, %d fresh, %d rows; want 1, 1, 5:\n%s\n%s", forward, fresh, len(refreshed), stdout.String(), stderr.String())
	}
	next := filepath.Join(filepath.Dir(planPath), "plan.new")
	if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: b2, Citations: next}); code != 0 {
		t.Fatalf("the refreshed plan was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%s", code, out, errText, stdout.String())
	}
}
