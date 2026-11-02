package dogfoodflow

import (
	"context"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// anchorCandidates parses a prepareAnchors read into the (pattern, path, line)
// candidates its loop appends: each pattern its record's text contains.
func anchorCandidates(base, out string, patterns []string) map[string]bool {
	found := map[string]bool{}
	for _, record := range strings.Split(out, "\n") {
		name, rest, named := strings.Cut(strings.TrimPrefix(record, base+":"), "\x00")
		number, text, numbered := strings.Cut(rest, "\x00")
		if !named || !numbered {
			continue
		}
		for _, pattern := range patterns {
			if strings.Contains(text, pattern) {
				found[pattern+"|"+name+"|"+number] = true
			}
		}
	}
	return found
}

// DCW-V0-984 (V1-3810): the check's anchor read lists the matching BASE files
// with `git grep -l`, reads them in one `cat-file --batch` and yields every
// candidate the former `git grep -o` read did, plus a pattern -o skipped
// inside an earlier overlapping match; a draft still runs the -o read; the
// flow's output bound and a read with no match keep their meaning.
func TestDCWV0984CheckAnchorGrepKeepsEveryGrepCandidate(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	long := strings.Repeat("lib/a.go.md and lib/b.go ", 40)
	writeTree(t, root, map[string]string{
		"docs/one.md":       "# One\n\nlib/a.go.md is here.\nNothing.\nlib/b.go twice lib/b.go\n",
		"docs/long.md":      long + "\n",
		"docs/later.md":     "x\ny\nz\nDCW-V0-001: lib/a.go\n",
		"bin/blob.dat":      "lib/a.go\x00binary\n",
		".taskman/t.json":   "{\"lib/a.go\": 1}\n",
		"internal/x/q.go":   "package x\n// lib/b.go\n",
		"docs/unrelated.md": "nothing here\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	patterns := []string{"a.go.md", "lib/a.go", "lib/b.go", "DCW-V0-001:"}
	arguments := []string{"grep", "-z", "-n", "-o", "-F", "-I"}
	for _, pattern := range patterns {
		arguments = append(arguments, "-e", pattern)
	}
	arguments = append(arguments, base, "--", ".", ":(exclude).corvint/", ":(exclude).taskman/")
	search := func(basenames bool) *proposal {
		return &proposal{flow: flow{ctx: context.Background(), root: root, stderr: io.Discard, base: base}, basenames: basenames}
	}
	former, status, bounded := search(true).boundedGit(arguments...)
	if status != 0 || !bounded {
		t.Fatalf("grep -o: status %d bounded %v", status, bounded)
	}
	out, status, bounded := search(true).anchorGrep(arguments, patterns)
	if status != 0 || !bounded {
		t.Fatalf("anchorGrep: status %d bounded %v", status, bounded)
	}
	before, after := anchorCandidates(base, former, patterns), anchorCandidates(base, out, patterns)
	for candidate := range before {
		if !after[candidate] {
			t.Errorf("anchorGrep lost %q", candidate)
		}
	}
	// -o resumes after lib/a.go, so the overlapping a.go.md is new here.
	for _, candidate := range []string{"a.go.md|docs/long.md|1", "lib/b.go|docs/one.md|5", "DCW-V0-001:|docs/later.md|4", "lib/b.go|internal/x/q.go|2"} {
		if !after[candidate] {
			t.Errorf("anchorGrep misses %q:\n%q", candidate, out)
		}
	}
	for candidate := range after {
		if strings.Contains(candidate, "bin/blob.dat") || strings.Contains(candidate, ".taskman/") {
			t.Errorf("anchorGrep read an excluded or binary path: %q", candidate)
		}
	}
	if before["a.go.md|docs/long.md|1"] {
		t.Fatal("the fixture no longer has an overlap grep -o skips")
	}
	if drafted, _, _ := search(false).anchorGrep(arguments, patterns); drafted != former {
		t.Fatal("a draft's anchor read is no longer the grep -o read")
	}
	small := search(true)
	small.gitOutputLimit = 256
	if _, _, bounded := small.anchorGrep(arguments, patterns); bounded {
		t.Fatal("records over the output bound were kept")
	}
	none := append(append([]string{}, arguments[:6]...), "-e", "absent/path.go", base, "--", ".")
	if out, status, bounded := search(true).anchorGrep(none, []string{"absent/path.go"}); out != "" || status != 1 || !bounded {
		t.Fatalf("no match: %q status %d bounded %v", out, status, bounded)
	}
}

// DCW-V0-984 (V1-3810): the binding phase names its sub-phases, and a
// concurrent cem status pair has its own row instead of falling into binding.
func TestDCWV0984BindingSubphaseRows(t *testing.T) {
	sleeper := func(delay time.Duration) Runner {
		return Runner{Path: filepath.Join(t.TempDir(), "verifier"), Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			time.Sleep(delay)
			return 0
		}}
	}
	evidence := t.TempDir()
	c := newCheck(context.Background(), "dogfood-check", CheckOptions{BaseVerifier: sleeper(40 * time.Millisecond), TreeVerifier: sleeper(10 * time.Millisecond), ConcurrentVerifiers: true}, io.Discard, io.Discard)
	c.evidence = evidence
	c.operationUnlock = func() {}
	c.report = filepath.Join(t.TempDir(), "missing", "dogfood-report.json")
	c.clock.to("binding")
	c.verifyAll(true, "cem", "status", "--map", "x")
	_ = c.record(true, 0)
	writeLines(io.Discard, c.tierWarnings(nil))
	c.writeTimings()
	seconds := map[string]float64{}
	var got []string
	for _, row := range readTimingRows(t, evidence) {
		value, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, row[0]+"\t"+row[1])
		seconds[row[0]+"\t"+row[1]] = value
	}
	want := "startup\t-\nbinding\t-\nverifier-pair\tcem status\nbinding\trecord\nbinding\ttiers"
	if strings.Join(got, "\n") != want {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), want)
	}
	if pair := seconds["verifier-pair\tcem status"]; pair < 0.04 {
		t.Fatalf("pair seconds %.3f", pair)
	}
}
