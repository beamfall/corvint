package appmap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func impactRun(t *testing.T, f prefixFixture, l *EdgeLedger, base string, o Options) ImpactReport {
	t.Helper()
	o.Root = f.root
	raw, err := Impact(context.Background(), f.m, l, base, o)
	if err != nil {
		t.Fatal(err)
	}
	var r ImpactReport
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func impactCommit(t *testing.T, f prefixFixture, files map[string]string) {
	t.Helper()
	for p, body := range files {
		dst := filepath.Join(f.root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, f.root, "change")
}

// ANL-V0-015: a router-file change invalidates its screen and every descendant, lists the touched
// edges with the ledger's flipped status and cheapest re-proving test, and names spec files.
func TestANLV0015RouterChangeReachesDescendants(t *testing.T) {
	f := prefixSetup(t)
	for i := range f.m.Screens {
		switch s := &f.m.Screens[i]; s.ID {
		case f.a:
			s.Anchor.Path, s.Parent = "router/parent.ts", ""
		case f.b:
			s.Anchor.Path, s.Parent = "router/child.ts", f.a
		case f.c:
			s.Anchor.Path, s.Parent = "router/other.ts", f.b
		}
	}
	base, err := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	impactCommit(t, f, map[string]string{"router/parent.ts": "export {}\n", "notes.md": "x\n"})
	l := provenLedger(f, [2]string{f.a, f.b})
	l.Revision = gitTest(t, f.root, "rev-parse", "HEAD")
	l.Edges[0].Status = "UNVERIFIED_AT_HEAD"
	r := impactRun(t, f, l, base, Options{})
	if r.Verdict != VerdictTargeted || r.Ignored != 1 || len(r.Unknown) != 0 {
		t.Fatalf("%+v", r)
	}
	got := map[string]string{}
	for _, s := range r.Screens {
		got[s.ID] = s.Basis
	}
	if got[f.a] != "router-anchor" || got[f.b] != "lineage:"+f.a || got[f.c] != "lineage:"+f.a {
		t.Fatalf("%v a=%s b=%s c=%s", got, f.a, f.b, f.c)
	}
	if len(r.Edges) != 1 || !r.Edges[0].Flipped || r.Edges[0].Reprove != "login-test" || len(r.TestKeys) != 1 {
		t.Fatalf("%+v", r.Edges)
	}
	if len(r.SpecFiles) == 0 {
		t.Fatal("no spec files for touched sequence edges")
	}
	for _, p := range r.SpecFiles {
		if strings.HasPrefix(p, "file:") {
			t.Fatal("element ID reported as a path", r.SpecFiles)
		}
	}
	l.MapDigest = "sha256:other"
	if _, err := Impact(context.Background(), f.m, l, base, Options{Root: f.root}); err == nil {
		t.Fatal("foreign ledger accepted")
	}
}

// ANL-V0-016: an unplaceable UI path forces full-suite-required, docs-only changes carry no
// navigation impact, and a bad base fails closed.
func TestANLV0016UnknownForcesFullSuite(t *testing.T) {
	f := prefixSetup(t)
	base, _ := (repo{ctx: context.Background(), root: f.root}).resolve("HEAD")
	impactCommit(t, f, map[string]string{"README.md": "docs\n"})
	if r := impactRun(t, f, nil, base, Options{}); r.Verdict != VerdictNoNavigation || r.Changed != 1 || r.Ignored != 1 {
		t.Fatalf("%+v", r)
	}
	impactCommit(t, f, map[string]string{"app/templates/club.html": "<div></div>\n"})
	r := impactRun(t, f, nil, base, Options{})
	if r.Verdict != VerdictFullSuite || len(r.Unknown) != 1 || r.Unknown[0].Reason != "unmapped-path" {
		t.Fatalf("%+v", r)
	}
	f.m.Screens[0].Specs = append(f.m.Screens[0].Specs, Attribution{File: "app/shared.ts", Basis: "import"})
	f.m.Files = append(f.m.Files, TestFile{ID: "file:alias", Path: "app/shared.ts", Role: roleSpec, Anchor: Anchor{Repo: "tests"}})
	impactCommit(t, f, map[string]string{"app/shared.ts": "export {}\n"})
	if r = impactRun(t, f, nil, base, Options{}); len(r.Unknown) != 2 {
		t.Fatalf("aliased-repo test file placed a primary-repo path: %+v", r)
	}
	if _, err := Impact(context.Background(), f.m, nil, "-bad", Options{Root: f.root}); err == nil {
		t.Fatal("bad base accepted")
	}
	small := impactRun(t, f, nil, base, Options{Budget: 700})
	if small.Verdict != VerdictFullSuite || small.Digest != r.Digest {
		t.Fatalf("%+v", small)
	}
}
