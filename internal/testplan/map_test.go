package testplan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// mapFixture commits the AMAP-V0 fixture app into a fresh repository and writes its map outside
// the worktree; it returns the root, the map path and the commit.
func mapFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := gitRepo(t, map[string]string{"README.md": "fixture\n"})
	src := filepath.Join("..", "appmap", "testdata", "fixture")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		writeFiles(t, root, map[string]string{filepath.ToSlash(rel): string(data)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	revision := commit(t, root)
	m, err := appmap.Build(context.Background(), root, "appmap.json", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	data, err := appmap.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "admin.map.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, path, revision
}

func mapRun(t *testing.T, root, mapPath, revision string, vs ...map[string]any) *Plan {
	t.Helper()
	p, err := Run(context.Background(), Request{Root: root, Input: inputJSON(t, vs...), Maps: []string{mapPath}, Revision: revision})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return p
}

func reasons(p *Plan) map[string]string {
	out := map[string]string{}
	for _, a := range p.Abstained {
		out[a.VariationID] = a.Reason
	}
	for _, test := range p.Tests {
		for _, s := range test.Steps {
			out[s.VariationID] = "placed"
		}
	}
	return out
}

// TestMapValidationAbstains validates screens, routes and lineage freshness against the committed
// AMAP-V0 fixture at the evaluated revision (TCN-V0-003).
func TestMapValidationAbstains(t *testing.T) {
	root, mapPath, first := mapFixture(t)
	screen := func(state string) func(map[string]any) { return set("screen", "screen:admin:"+state) }
	vs := []map[string]any{
		variation("V1", screen("app.clubs.teesheets"), set("route", "/clubs/{clubId}/teesheets")),
		variation("V2", screen("app.home"), set("route", "/home/x")),
		variation("V3", screen("app.dynamic")),
		variation("V4", screen("app")),
		variation("V5", screen("nope")),
		variation("V6", screen("app.home"), set("app", "other")),
		variation("V7", screen("app.home"), drop("org")),
		variation("V8", screen("app.home")),
	}
	p := mapRun(t, root, mapPath, "", vs...)
	want := map[string]string{
		"V1": "placed", "V2": ReasonRouteMismatch, "V3": ReasonUnresolvedScreen, "V4": ReasonUnresolvedScreen,
		"V5": ReasonUnresolvedScreen, "V6": ReasonUnresolvedScreen, "V7": ReasonMissingAnchor, "V8": "placed",
	}
	if got := reasons(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("reasons %v\n%s", got, p.Table())
	}
	if p.AnchorValidation != anchorsValidated || p.Status != statusIncomplete || len(p.Maps) != 1 || p.Maps[0].App != "admin" ||
		p.MapsDigest == none || p.EvaluatedRevision != first {
		t.Fatalf("plan header %+v", p)
	}
	for _, a := range p.Abstained {
		if a.VariationID == "V7" && !reflect.DeepEqual(a.Missing, []string{"org"}) {
			t.Fatalf("missing %v", a.Missing)
		}
		if a.VariationID != "V7" && len(a.Missing) != 0 {
			t.Fatalf("%s names missing members %v", a.VariationID, a.Missing)
		}
	}

	// Edit the teesheets state and commit: its lineage reads STALE at HEAD, FRESH at the old revision.
	routes := filepath.Join(root, "app", "routes.js")
	data, err := os.ReadFile(routes)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "url: '/teesheets?date'", "url: '/teesheets?date&view'", 1)
	if edited == string(data) {
		t.Fatal("fixture routes changed shape")
	}
	writeFiles(t, root, map[string]string{"app/routes.js": edited})
	commit(t, root)
	head := mapRun(t, root, mapPath, "", vs[0], vs[7])
	if got := reasons(head); got["V1"] != ReasonStaleAnchor || got["V8"] != "placed" {
		t.Fatalf("at HEAD: %v", got)
	}
	old := mapRun(t, root, mapPath, first, vs[0], vs[7])
	if got := reasons(old); got["V1"] != "placed" || old.EvaluatedRevision != first {
		t.Fatalf("at the old revision: %v", got)
	}
}

// TestMapUnknownLineageAbstains: an UNKNOWN lineage abstains anchor-unknown (TCN-V0-003).
func TestMapUnknownLineageAbstains(t *testing.T) {
	maps := []MapView{{App: "admin", Digest: "sha256:" + strings.Repeat("0", 64), Screens: map[string]MapScreen{
		exampleScreen: {Resolved: true, Template: "/t", Lineage: "UNKNOWN"},
		"other":       {Resolved: true, Template: "/o", Lineage: "FRESH"},
	}}}
	p := build(t, Evidence{Revision: fixedRevision, TestsDigest: none, Maps: maps}, 8, variation("V1"), variation("V2", set("screen", "other"), set("route", "/o")))
	if got := reasons(p); got["V1"] != ReasonAnchorUnknown || got["V2"] != "placed" {
		t.Fatalf("reasons %v", got)
	}
}

// TestAnchorsNotRunWithoutMap: without --map nothing is validated and evidence reads none.
func TestAnchorsNotRunWithoutMap(t *testing.T) {
	root, _, _ := mapFixture(t)
	p, err := Run(context.Background(), Request{Root: root, Input: inputJSON(t, variation("V1", set("screen", "screen:admin:nope")))})
	if err != nil {
		t.Fatal(err)
	}
	if p.AnchorValidation != anchorsNotRun || p.MapsDigest != none || len(p.Maps) != 0 || p.EvaluatedRevision != none || p.Status != statusComplete {
		t.Fatalf("plan %+v", p)
	}
}

// TestMapRefusals: an undecodable map or two maps of one app refuse appmap-invalid-map.
func TestMapRefusals(t *testing.T) {
	root, mapPath, _ := mapFixture(t)
	in := inputJSON(t, variation("V1"))
	for name, maps := range map[string][]string{
		"undecodable": {writeTemp(t, "bad.json", `{"schema":"application-map/0"`)},
		"missing":     {filepath.Join(t.TempDir(), "absent.json")},
		"same app":    {mapPath, mapPath},
	} {
		if _, err := Run(context.Background(), Request{Root: root, Input: in, Maps: maps}); code(err) != "appmap-invalid-map" {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
