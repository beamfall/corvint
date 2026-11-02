package appmap

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ANL-V0-810: a manifest-declared exemption moves a matching non-UI changed path from unknown
// non-ui-path to ignored_paths and names the declaring manifest line; an undeclared one still
// forces full-suite-required, and an invalid declaration is refused.
func TestANLV0810DeclaredImpactExemption(t *testing.T) {
	root, _ := fixtureRepo(t)
	mpath := filepath.Join(root, "appmap.json")
	raw, err := os.ReadFile(mpath)
	if err != nil {
		t.Fatal(err)
	}
	decl := `  "impact_exempt": [
    {"path": "docs/**", "reason": "documentation is never served"},
    {"path": "*.md", "reason": "root markdown is never served"}
  ],
`
	text := strings.Replace(string(raw), `  "page_object_screens"`, decl+`  "page_object_screens"`, 1)
	writeFile(t, root, "appmap.json", text)
	f := prefixFixture{root: root, m: build(t, root, commitAll(t, root, "exempt"))}
	docsLine := strings.Count(text[:strings.Index(text, `"docs/**"`)], "\n") + 1
	mdLine := strings.Count(text[:strings.Index(text, `"*.md"`)], "\n") + 1

	base, _ := (repo{ctx: context.Background(), root: root}).resolve("HEAD")
	impactCommit(t, f, map[string]string{"docs/guide/setup.md": "setup\n", "README.md": "read me\n"})
	r := impactRun(t, f, nil, base, Options{})
	if r.Verdict != VerdictNoNavigation || r.Changed != 2 || r.Ignored != 2 || len(r.Unknown) != 0 || len(r.Exempt) != 2 {
		t.Fatalf("docs-only change under an exemption: %+v", r)
	}
	for _, e := range r.Exempt {
		want := map[string]string{"README.md": "appmap.json:" + strconv.Itoa(mdLine), "docs/guide/setup.md": "appmap.json:" + strconv.Itoa(docsLine)}[e.Path]
		if e.Declared != want || e.Reason == "" || e.Pattern == "" {
			t.Errorf("exempt %+v, want declared %s", e, want)
		}
	}

	head := gitTest(t, root, "rev-parse", "HEAD")
	// A template or script extension the classifier does not read as UI is never exempt, even
	// under a matching glob: it may still render a screen.
	impactCommit(t, f, map[string]string{"docs/more.md": "more\n", "notes/plan.txt": "plan\n", "app/models/club.rb": "class Club; end\n",
		"docs/views/club.html.erb": "<p>club</p>\n", "docs/tools/build.mts": "export {};\n"})
	r = impactRun(t, f, nil, head, Options{})
	got := impactReasons(r)
	if r.Verdict != VerdictFullSuite || r.Ignored != 1 || len(got) != 4 || got["notes/plan.txt"] != "non-ui-path" || got["app/models/club.rb"] != "non-ui-path" ||
		got["docs/views/club.html.erb"] != "non-ui-path" || got["docs/tools/build.mts"] != "non-ui-path" {
		t.Fatalf("change outside the exemption: %+v", r)
	}

	for _, bad := range []string{`[]`, `[{"path": "docs/**", "reason": " "}]`, `[{"path": "../x", "reason": "r"}]`,
		`[{"path": "a/**b", "reason": "r"}]`, `[{"path": "d", "reason": "r"}, {"path": "d", "reason": "s"}]`} {
		if _, err := decodeManifest([]byte(strings.Replace(string(raw), `"page_object_screens"`, `"impact_exempt": `+bad+`, "page_object_screens"`, 1))); codeOf(err) == "" {
			t.Errorf("accepted impact_exempt %s", bad)
		}
	}
}
