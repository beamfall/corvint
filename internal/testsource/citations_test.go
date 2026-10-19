package testsource

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// citationRepo commits a source file, then a later file, then a review document whose default
// citation revision is the first commit. It returns the root and the three commits.
func citationRepo(t *testing.T) (string, [3]string) {
	t.Helper()
	root := t.TempDir()
	first := commitFiles(t, root, map[string]string{"a/old.go": "package a\n\nfunc Old() {}\n"})
	second := commitFiles(t, root, map[string]string{"a/new.go": "package a\n\n// Foo is new.\nfunc Foo() {\n}\n"})
	doc := "# Review\n\n<!-- corvint-cite-revision: " + first + " -->\n\n" +
		"- `a/new.go:1-2` was cited before it existed.\n" +
		"- `a/old.go:2-9` runs past the end.\n" +
		"- `a/old.go:1-3` and `a/old.go:3#Old` are valid.\n" +
		"- `a/old.go:1-2#Old` misses its symbol.\n" +
		"- `a/new.go:4-5#Foo@HEAD` pins its own revision.\n" +
		"- `a/old.go:1@nosuchrev` cannot be resolved; `make gate` is not a citation.\n" +
		"\n```\n`a/new.go:9`\n```\n"
	third := commitFiles(t, root, map[string]string{"docs/review.md": doc})
	return root, [3]string{first, second, third}
}

func citationRows(findings []CitationFinding) []string {
	rows := []string{}
	for _, f := range findings {
		rows = append(rows, strings.Join([]string{f.Doc, itoa(f.Line), f.Citation, f.Reason, f.Suggestion}, " "))
	}
	return rows
}

func itoa(n int) string { return fmt.Sprintf("%02d", n) }

// TestCitationsAcceptance: the V1-1934 acceptance criteria. A citation to a file added after the
// cited revision is missing-at-revision with the adding commit as the nearest revision; a range
// past the end is reported; valid citations, prose and fenced code produce nothing; a symbol
// outside the range is reported; an unresolvable revision abstains (TSA-V0-250..255).
func TestCitationsAcceptance(t *testing.T) {
	root, commits := citationRepo(t)
	report, err := Citations(OpenRepo(context.Background(), root), CitationOptions{Docs: []string{"docs/review.md"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docs/review.md 05 a/new.go:1-2 missing-at-revision " + commits[1],
		"docs/review.md 06 a/old.go:2-9 range-out-of-bounds ",
		"docs/review.md 08 a/old.go:1-2#Old symbol-not-in-range ",
	}
	if got := citationRows(report.Findings); !reflect.DeepEqual(got, want) {
		t.Fatalf("findings\n got %q\nwant %q", got, want)
	}
	if got := citationRows(report.Abstained); !reflect.DeepEqual(got, []string{"docs/review.md 10 a/old.go:1@nosuchrev revision-unresolved "}) {
		t.Fatalf("abstained %q", got)
	}
	if report.Checked != 7 || report.Revision != commits[2] || report.Schema != CitationsSchema {
		t.Fatalf("report %+v", report)
	}
	if report.Findings[0].Revision != commits[0] || report.Findings[0].Line != 5 {
		t.Fatalf("finding %+v", report.Findings[0])
	}
}

// TestCitationsReadOnlyObjectStore: the check reads the object store only. With every working-tree
// file removed and the index unchanged, the report is the same and the index and HEAD are
// untouched (TSA-V0-256).
func TestCitationsReadOnlyObjectStore(t *testing.T) {
	root, _ := citationRepo(t)
	repo := OpenRepo(context.Background(), root)
	before, err := Citations(repo, CitationOptions{Docs: []string{"docs/review.md"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a/old.go", "a/new.go", "docs/review.md"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	state := func() [2][32]byte {
		index, err := os.ReadFile(filepath.Join(root, ".git", "index"))
		if err != nil {
			t.Fatal(err)
		}
		return [2][32]byte{sha256.Sum256(index), sha256.Sum256([]byte(gitTest(t, root, "rev-parse", "HEAD") + gitTest(t, root, "status", "--porcelain")))}
	}
	was := state()
	after, err := Citations(repo, CitationOptions{Docs: []string{"docs/review.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("report changed without a working tree\nbefore %+v\nafter  %+v", before, after)
	}
	if state() != was {
		t.Fatal("the check changed the index, HEAD or the working tree status")
	}
}

// TestCitationsDeletedPathAndRefusals: a path deleted before the cited revision suggests the last
// commit that had it; a bad document path, no documents and a missing document are refused
// (TSA-V0-254, TSA-V0-252).
func TestCitationsDeletedPathAndRefusals(t *testing.T) {
	root := t.TempDir()
	had := commitFiles(t, root, map[string]string{"a/gone.go": "package a\n"})
	commitFiles(t, root, map[string]string{"a/gone.go": "", "a/keep.go": "package a\n"})
	commitFiles(t, root, map[string]string{"docs/r.md": "See `a/gone.go:1`.\n"})
	repo := OpenRepo(context.Background(), root)
	report, err := Citations(repo, CitationOptions{Docs: []string{"docs/r.md", "docs/r.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Reason != CitationMissingAtRevision || report.Findings[0].Suggestion != had {
		t.Fatalf("findings %+v", report.Findings)
	}
	for _, c := range []struct {
		docs []string
		code string
	}{{[]string{"../x.md"}, "tests-invalid-arguments"}, {nil, "tests-invalid-arguments"}, {[]string{"docs/none.md"}, "tests-source-unavailable"}} {
		if _, err := Citations(repo, CitationOptions{Docs: c.docs}); !hasCode(err, c.code) {
			t.Errorf("%v: %v", c.docs, err)
		}
	}
}

// TestCitationsReviewForms: an explicit `@HEAD` names the repository HEAD, not the read revision;
// `:0` is out of bounds; a default-revision comment inside a fence is ignored; a fence closes only
// on its own character with at least its length (TSA-V0-250, TSA-V0-251, TSA-V0-253).
func TestCitationsReviewForms(t *testing.T) {
	root := t.TempDir()
	doc := "~~~~\n```\n`a/zz.go:3`\n```\n<!-- corvint-cite-revision: nosuchrev -->\n~~~~\n\n" +
		"- `a/x.go:1@HEAD` exists at HEAD.\n- `a/y.go:0` has no line zero.\n"
	read := commitFiles(t, root, map[string]string{"a/y.go": "package a\n", "docs/r.md": doc})
	commitFiles(t, root, map[string]string{"a/x.go": "package a\n"})
	report, err := Citations(OpenRepo(context.Background(), root), CitationOptions{Revision: read, Docs: []string{"docs/r.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := citationRows(report.Findings); !reflect.DeepEqual(got, []string{"docs/r.md 09 a/y.go:0 range-out-of-bounds "}) || report.Checked != 2 || len(report.Abstained) != 0 {
		t.Fatalf("findings %q abstained %+v checked %d", got, report.Abstained, report.Checked)
	}
}
