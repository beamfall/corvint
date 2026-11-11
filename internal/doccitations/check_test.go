package doccitations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func foreignRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		command.Env = append(environment(), "HOME="+root)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	return root
}

// SDD-V0-633: the citation check runs in a repository with no Corvint scripts or layout.
func TestSDDV0633CitationsCheckForeignRepository(t *testing.T) {
	source := "package lib\n\nfunc Run() {\n}\n"
	good := Anchor(strings.Split(strings.TrimSuffix(source, "\n"), "\n"), []int{3, 4})[:8]
	readme := strings.Join([]string{
		"Good `lib/run.go:3-4@" + good + "` and `lib/run.go:3`.",
		"Range `lib/run.go:9` and reversed `lib/run.go:4-3`.",
		"Blank `lib/run.go:2` and closing `lib/run.go:4,3`.",
		"Moved `lib/run.go:3@deadbeef`.",
		"Gone `lib/gone.go:1` in a tracked directory; `other/x.go:1` is gone too; `make check` is not a citation.",
		"",
	}, "\n")
	root := foreignRepository(t, map[string]string{"README.md": readme, "lib/run.go": source, "docs/clean.md": "See `lib/run.go:3`.\n"})
	report, err := Check(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, failure := range report.Failures {
		got = append(got, failure.Document+":"+strconv.Itoa(failure.Line)+" "+failure.Token+" "+failure.Kind)
	}
	want := []string{
		"README.md:2 `lib/run.go:9` out-of-range",
		"README.md:2 `lib/run.go:4-3` out-of-range",
		"README.md:3 `lib/run.go:2` trivial-span",
		"README.md:3 `lib/run.go:4,3` trivial-span",
		"README.md:4 `lib/run.go:3@deadbeef` anchor-mismatch",
		"README.md:5 `lib/gone.go:1` missing-file",
		"README.md:5 `other/x.go:1` missing-file",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || report.Documents != 2 || report.Citations != 10 {
		t.Fatalf("documents=%d citations=%d failures:\n%s", report.Documents, report.Citations, strings.Join(got, "\n"))
	}
	clean, err := Check(context.Background(), root, []string{"docs/clean.md"})
	if err != nil || len(clean.Failures) != 0 || clean.Documents != 1 || clean.Citations != 1 {
		t.Fatalf("named clean document: %+v %v", clean, err)
	}
	if _, err := Check(context.Background(), root, []string{"untracked.md"}); err == nil {
		t.Fatal("untracked named document admitted")
	}
	if _, err := Check(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatal("non-repository admitted")
	}
}

// SDD-V0-633: line 0 and a range far past the file end are out-of-range; the
// check neither indexes line 0 nor expands the range before validating it.
func TestSDDV0633CitationsCheckBoundsBeforeExpanding(t *testing.T) {
	readme := "Zero `lib/run.go:0`, zero list `lib/run.go:3,0`, huge `lib/run.go:1-1000000000`.\n"
	root := foreignRepository(t, map[string]string{"README.md": readme, "lib/run.go": "package lib\n\nfunc Run() {}\n"})
	report, err := Check(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, failure := range report.Failures {
		got = append(got, failure.Token+" "+failure.Kind+": "+failure.Detail)
	}
	want := []string{
		"`lib/run.go:0` out-of-range: line 0 is outside lib/run.go (3 lines)",
		"`lib/run.go:3,0` out-of-range: line 0 is outside lib/run.go (3 lines)",
		"`lib/run.go:1-1000000000` out-of-range: line 1000000000 is outside lib/run.go (3 lines)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || report.Citations != 3 {
		t.Fatalf("citations=%d failures:\n%s", report.Citations, strings.Join(got, "\n"))
	}
}

// DMF-V1-004: a cited untracked top-level file is missing-file, a citation inside fenced code is
// skipped and counted, and a tracked citation with no pin is listed as unpinned, not failed.
func TestDMFV1004CitationsMissingFencedUnpinned(t *testing.T) {
	source := "package lib\n\nfunc Run() {\n}\n"
	pin := Anchor(strings.Split(strings.TrimSuffix(source, "\n"), "\n"), []int{3})[:8]
	doc := strings.Join([]string{
		"Top-level `NOTES.md:10` is not tracked.",
		"```go",
		"// `a.go:3` is example text, as is `lib/run.go:99`.",
		"```",
		"~~~~",
		"`lib/run.go:98`",
		"~~~~",
		"Unpinned `lib/run.go:3`, pinned `lib/run.go:3@" + pin + "`.",
		"",
	}, "\n")
	root := foreignRepository(t, map[string]string{"docs/guide.md": doc, "lib/run.go": source})
	report, err := Check(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 1 || report.Failures[0].Kind != MissingFile || report.Failures[0].Token != "`NOTES.md:10`" || report.Failures[0].Line != 1 {
		t.Fatalf("failures %+v", report.Failures)
	}
	if report.Fenced != 3 || report.Citations != 3 {
		t.Fatalf("fenced=%d citations=%d", report.Fenced, report.Citations)
	}
	if len(report.Unpinned) != 1 || report.Unpinned[0] != (Citation{Document: "docs/guide.md", Line: 8, Token: "`lib/run.go:3`"}) {
		t.Fatalf("unpinned %+v", report.Unpinned)
	}
}
