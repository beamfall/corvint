package testsource

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestCitationsLargeCorpus: the V1-1972 acceptance criterion. A generated corpus of 2,000 Markdown
// documents holding 13,000 citations in the html-sub syntax is checked in one invocation selected
// by one glob, past the former 256-document and 4,096-citation caps. Every finding names its
// document, line and citation; the unparseable marker is reported, not skipped; per-document totals
// add up (TSA-V0-380, TSA-V0-382..385).
func TestCitationsLargeCorpus(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{}
	for p := 0; p < 40; p++ {
		var b strings.Builder
		fmt.Fprintf(&b, "package p%02d\n\n", p)
		for f := 0; f < 10; f++ {
			fmt.Fprintf(&b, "// F%d is fixture %d.\nfunc F%d() int {\n\treturn %d\n}\n\n", f, f, f, f)
		}
		files[fmt.Sprintf("src/p%02d/p.go", p)] = b.String()
	}
	const docs = 2000
	bad := map[int]string{
		17:   "<sub>src/none/x.go line 3</sub>",
		503:  "<sub>src/p03/p.go lines 40-900</sub>",
		1201: "<sub>`src/p04/p.go`, lines 3-6 #F7</sub>",
		1999: "<sub>src/p05/p.go line three</sub>",
	}
	want := 0
	for d := 0; d < docs; d++ {
		var b strings.Builder
		fmt.Fprintf(&b, "# Note %d\n\nSome prose with a <sub>footnote</sub>.\n\n", d)
		n := 6 + d%2
		for c := 0; c < n; c++ {
			p, f := (d+c)%40, (d*7+c)%10
			start := 3 + 5*f
			if c == 0 && bad[d] != "" {
				fmt.Fprintf(&b, "Claim %d.<sub>%s</sub>\n", c, strings.TrimSuffix(strings.TrimPrefix(bad[d], "<sub>"), "</sub>"))
			} else {
				fmt.Fprintf(&b, "Claim %d.<sub>src/p%02d/p.go lines %d-%d #F%d</sub>\n", c, p, start, start+3, f)
			}
			want++
		}
		files[fmt.Sprintf("docs/corpus/d%02d/n%04d.md", d/100, d)] = b.String()
	}
	files["docs/other.txt"] = "<sub>src/none/y.go line 1</sub>\n"
	head := commitFiles(t, root, files)
	began := time.Now()
	report, err := Citations(OpenRepo(context.Background(), root), CitationOptions{Globs: []string{"docs/**/*.md"}, Syntax: CitationSyntaxHTMLSub})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d documents, %d citations in %s", report.Documents, report.Checked, time.Since(began))
	if want != 13000 || report.Documents != docs || report.Checked != want-1 || report.Revision != head || report.Syntax != CitationSyntaxHTMLSub {
		t.Fatalf("documents %d checked %d of %d syntax %s", report.Documents, report.Checked, want, report.Syntax)
	}
	gotFindings := citationRows(report.Findings)
	wantFindings := []string{
		"docs/corpus/d00/n0017.md 05 src/none/x.go line 3 missing-at-revision ",
		"docs/corpus/d05/n0503.md 05 src/p03/p.go lines 40-900 range-out-of-bounds ",
		"docs/corpus/d12/n1201.md 05 `src/p04/p.go`, lines 3-6 #F7 symbol-not-in-range ",
	}
	if !reflect.DeepEqual(gotFindings, wantFindings) || len(report.Abstained) != 0 {
		t.Fatalf("findings\n got %q\nwant %q\nabstained %+v", gotFindings, wantFindings, report.Abstained)
	}
	if got := citationRows(report.Unparseable); !reflect.DeepEqual(got, []string{"docs/corpus/d19/n1999.md 05 src/p05/p.go line three unparseable "}) {
		t.Fatalf("unparseable %q", got)
	}
	sum := CitationDocumentTotal{}
	for _, total := range report.Totals {
		sum.Citations += total.Citations
		sum.Findings += total.Findings
		sum.Unparseable += total.Unparseable
	}
	if len(report.Totals) != docs || sum.Citations != report.Checked || sum.Findings != 3 || sum.Unparseable != 1 ||
		report.Totals[17] != (CitationDocumentTotal{Doc: "docs/corpus/d00/n0017.md", Citations: 7, Findings: 1}) {
		t.Fatalf("totals %+v sum %+v", report.Totals[17], sum)
	}
}

// TestCitationsCustomPattern: a caller pattern reads repository, path, range and revision groups;
// a citation naming another repository abstains; a match whose path cannot be read is unparseable;
// with a marker, every marker candidate the pattern does not read is unparseable (TSA-V0-381,
// TSA-V0-382).
func TestCitationsCustomPattern(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"a/b.go": "package a\n\nfunc B() {}\n"})
	doc := "One [[core:a/b.go#L1-L3]] and [[core:a/b.go#L2-L9]].\n" +
		"Two [[other:x/y.go#L1]] and [[core:../b.go#L1]].\n" +
		"Three [[core:a/b.go#Lx]] and [[core:a/b.go#L3@HEAD]].\n"
	commitFiles(t, root, map[string]string{"docs/r.md": doc})
	repo := OpenRepo(context.Background(), root)
	pattern := `\[\[(?P<repo>[a-z]+):(?P<path>[^#\]@]+)#L(?P<start>[0-9]+)(?:-L(?P<end>[0-9]+))?(?:@(?P<rev>[^\]]+))?\]\]`
	report, err := Citations(repo, CitationOptions{Docs: []string{"docs/r.md"}, Pattern: pattern, Repository: "core"})
	if err != nil {
		t.Fatal(err)
	}
	if got := citationRows(report.Findings); !reflect.DeepEqual(got, []string{"docs/r.md 01 [[core:a/b.go#L2-L9]] range-out-of-bounds "}) {
		t.Fatalf("findings %q", got)
	}
	if got := citationRows(report.Abstained); !reflect.DeepEqual(got, []string{"docs/r.md 02 [[other:x/y.go#L1]] repository-not-checked "}) {
		t.Fatalf("abstained %q", got)
	}
	if got := citationRows(report.Unparseable); !reflect.DeepEqual(got, []string{"docs/r.md 02 [[core:../b.go#L1]] unparseable "}) || report.Checked != 4 || report.Syntax != CitationSyntaxPattern {
		t.Fatalf("unparseable %q checked %d", got, report.Checked)
	}
	report, err = Citations(repo, CitationOptions{Docs: []string{"docs/r.md"}, Pattern: pattern, Marker: `\[\[[^\]]*\]\]`, Repository: "core"})
	if err != nil {
		t.Fatal(err)
	}
	if got := citationRows(report.Unparseable); !reflect.DeepEqual(got, []string{"docs/r.md 02 [[core:../b.go#L1]] unparseable ", "docs/r.md 03 [[core:a/b.go#Lx]] unparseable "}) {
		t.Fatalf("marker unparseable %q", got)
	}
	for _, c := range []CitationOptions{
		{Pattern: `(?P<start>\d+)`}, {Pattern: `(?P<path>\S+)(?P<line>\d+)`}, {Pattern: `(`}, {Marker: `x`},
		{Syntax: "html-sub", Pattern: `(?P<path>\S+)`}, {Syntax: "nosuch"}, {Pattern: strings.Repeat("a", 5000) + `(?P<path>x)`},
		{Globs: []string{"../x/*.md"}}, {Globs: []string{"docs/[.md"}}, {Globs: []string{"/docs/*.md"}}, {Manifests: []string{"../m.txt"}},
		{Docs: []string{"docs/r.md"}, Repository: "a\nb"},
	} {
		if c.Docs == nil {
			c.Docs = []string{"docs/r.md"}
		}
		if _, err := Citations(repo, c); !hasCode(err, "tests-invalid-arguments") {
			t.Errorf("%+v: %v", c, err)
		}
	}
	many := CitationOptions{Docs: []string{"docs/r.md"}}
	for i := 0; i <= maxCitationGlobs; i++ {
		many.Globs = append(many.Globs, "docs/*.md")
	}
	if _, err := Citations(repo, many); !hasCode(err, "tests-bound-exceeded") {
		t.Errorf("globs: %v", err)
	}
}

// TestCitationsManifestGlobAndForms: a committed manifest lists documents (comments, blanks and
// repeats ignored), a glob adds the rest once; html-sub reads its label, code, dash, symbol and
// revision forms, ignores a <sub> without `line` and reports one naming `line` it cannot read; an
// inline-code span opening with a path and `:DIGIT` that does not read is unparseable (TSA-V0-380,
// TSA-V0-382, TSA-V0-383, TSA-V0-385).
func TestCitationsManifestGlobAndForms(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"a/b.go": "package a\n\n// Foo is here.\nfunc Foo() {\n}\n", "README.md": "# r\n"})
	head := commitFiles(t, root, map[string]string{
		"docs/one.md":       "<SUB>source: <code>a/b.go</code>, lines 3–4 #Foo @HEAD</sub> and <sub>README.md line 1</sub>\n",
		"docs/two.md":       "x<sub>2</sub> and <sub>a/b.go line</sub> and <sub class=\"c\">a/b.go line 9</sub>\n```\n<sub>bad line</sub>\n```\n",
		"docs/sub/three.md": "<sub>a/b.go lines 1 to 5</sub>\n",
		"docs/manifest.txt": "# documents\n\ndocs/two.md\r\ndocs/one.md\ndocs/two.md\n",
		"docs/bad.txt":      "docs/one.md\n../x.md\n",
		"notes/inline.md":   "`a/b.go:3x` `a/b.go:4#Foo` `make gate` `a/b.go#heading-x`\n",
	})
	repo := OpenRepo(context.Background(), root)
	report, err := Citations(repo, CitationOptions{Manifests: []string{"docs/manifest.txt"}, Globs: []string{"docs/**/*.md"}, Syntax: CitationSyntaxHTMLSub})
	if err != nil {
		t.Fatal(err)
	}
	totals := []CitationDocumentTotal{
		{Doc: "docs/two.md", Citations: 1, Findings: 1, Unparseable: 1},
		{Doc: "docs/one.md", Citations: 2},
		{Doc: "docs/sub/three.md", Citations: 1},
	}
	if !reflect.DeepEqual(report.Totals, totals) || report.Documents != 3 || report.Checked != 4 || report.Revision != head {
		t.Fatalf("totals %+v", report.Totals)
	}
	if got := citationRows(append(report.Findings, report.Unparseable...)); !reflect.DeepEqual(got, []string{
		"docs/two.md 01 a/b.go line 9 range-out-of-bounds ", "docs/two.md 01 a/b.go line unparseable "}) {
		t.Fatalf("findings %q", got)
	}
	report, err = Citations(repo, CitationOptions{Docs: []string{"notes/inline.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := citationRows(report.Unparseable); !reflect.DeepEqual(got, []string{"notes/inline.md 01 a/b.go:3x unparseable "}) || report.Checked != 1 || len(report.Findings) != 0 || report.Syntax != CitationSyntaxInlineCode {
		t.Fatalf("inline unparseable %q checked %d findings %+v", got, report.Checked, report.Findings)
	}
	for _, c := range []struct {
		options CitationOptions
		code    string
	}{
		{CitationOptions{Globs: []string{"nothing/**/*.md"}}, "tests-invalid-arguments"},
		{CitationOptions{Manifests: []string{"docs/none.txt"}}, "tests-source-unavailable"},
		{CitationOptions{Manifests: []string{"docs/bad.txt"}}, "tests-invalid-arguments"},
		{CitationOptions{Docs: []string{"docs"}}, "tests-source-unavailable"},
	} {
		if _, err := Citations(repo, c.options); !hasCode(err, c.code) {
			t.Errorf("%+v: %v", c.options, err)
		}
	}
}

// TestCitationGlobMatch: `**` spans zero or more segments; other wildcards stay in one segment.
func TestCitationGlobMatch(t *testing.T) {
	for _, c := range []struct {
		glob, name string
		match      bool
	}{
		{"docs/**/*.md", "docs/a.md", true}, {"docs/**/*.md", "docs/x/y/a.md", true}, {"docs/*.md", "docs/x/a.md", false},
		{"**", "a/b/c", true}, {"docs/**", "docs", true}, {"d?cs/[ab].md", "docs/b.md", true}, {"docs/**/x/*.md", "docs/x/a.md", true},
		{"docs/**/x/*.md", "docs/y/a.md", false}, {"docs/a.md", "docs/a.md", true},
	} {
		g, err := compileCitationGlob(c.glob)
		if err != nil {
			t.Fatal(err)
		}
		if g.match(c.name) != c.match {
			t.Errorf("%s ~ %s: want %v", c.glob, c.name, c.match)
		}
	}
	if g, _ := compileCitationGlob("docs/x/*.md"); g.prefix != "docs/x/" {
		t.Errorf("prefix %q", g.prefix)
	}
}
