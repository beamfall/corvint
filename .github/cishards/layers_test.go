package cishards

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This file reads the checkout (TestLayerDriftLive), so it is not one of the files the isolated
// helper modules copy; the go-static job runs it in the checkout (AFP-V0-2792).

// layerFixture is a five-area manifest shaped like decision 0594 item 8, under a synthetic zz/
// tree, with packages that exercise every selection rule.
type layerFixture struct {
	areas     []LayerArea
	edges     []string
	unbounded []string
	scopes    map[string][]string
	pkgs      []LayerPackage
}

func newLayerFixture() *layerFixture {
	return &layerFixture{
		areas: []LayerArea{
			{Name: "shared", Layer: 0, Prefixes: []string{"zz/shared"}},
			{Name: "analyzers", Layer: 1, Prefixes: []string{"zz/analyzers"}},
			{Name: "tests", Layer: 1, Prefixes: []string{"zz/tests"}},
			{Name: "docs", Layer: 1, Prefixes: []string{"zz/docs"}},
			{Name: "release", Layer: 2, Prefixes: []string{"zz/release"}},
		},
		edges:     []string{},
		unbounded: []string{"zz/tests/unbounded"},
		scopes:    map[string][]string{"zz/analyzers/scoped": {"zz/docs/data/"}},
		pkgs: []LayerPackage{
			{Dir: "zz/shared/a"},
			{Dir: "zz/analyzers/b", Imports: []string{"zz/shared/a"}},
			{Dir: "zz/analyzers/named", Refs: []string{"zz/docs/fixtures"}},
			{Dir: "zz/analyzers/one", Refs: []string{"zz"}},
			{Dir: "zz/analyzers/scoped"},
			{Dir: "zz/tests/c", Imports: []string{"zz/shared/a"}},
			{Dir: "zz/tests/imports-release", TestImports: []string{"zz/release/e"}},
			{Dir: "zz/tests/unbounded"},
			{Dir: "zz/docs/d", Imports: []string{"zz/shared/a"}},
			{Dir: "zz/release/e", Imports: []string{"zz/docs/d", "zz/analyzers/b"}},
			{Dir: "zz/orphan", TestOnly: true},
		},
	}
}

func (f *layerFixture) inputs(t *testing.T) LayerInputs {
	t.Helper()
	manifest, _ := json.Marshal(map[string]any{"profile": layerManifestProfile, "module": LayerModule, "areas": f.areas})
	baseline, _ := json.Marshal(map[string]any{"profile": layerBaselineProfile, "edges": f.edges})
	unbounded, _ := json.Marshal(map[string]any{"profile": layerUnboundedProfile, "units": f.unbounded})
	scopes, _ := json.Marshal(map[string]any{"profile": layerReadScopesProfile, "packages": f.scopes})
	in, err := ParseLayerInputs(manifest, baseline, unbounded, scopes)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func layerPaths(dirs ...string) []string {
	out := make([]string, len(dirs))
	for i, d := range dirs {
		out[i] = LayerImportPath(d)
	}
	return out
}

// TestPlanLayers_AFPV02790 holds the derivation: a change selects its areas and every area that
// may import them, plus the packages whose tests import, declare or name what changed and the
// unbounded readers; anything it cannot place selects the whole suite.
func TestPlanLayers_AFPV02790(t *testing.T) {
	always := []string{"zz/orphan", "zz/tests/unbounded"}
	cases := []struct {
		name     string
		changed  []string
		edit     func(*layerFixture)
		universe []string
		mode     string
		reason   string   // a FULL reason fragment
		areas    []string // selected areas, in manifest order
		packages []string // selected package dirs beyond always
	}{
		{name: "docs only", changed: []string{"zz/docs/d/x.go"}, mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "shared only selects everything", changed: []string{"zz/shared/a/x.go"}, mode: LayerModeFull, reason: "every area depends"},
		{name: "analyzer only", changed: []string{"zz/analyzers/b/x.go"}, mode: LayerModeLayers, areas: []string{"analyzers", "release"},
			packages: []string{"zz/analyzers/b", "zz/analyzers/named", "zz/analyzers/one", "zz/analyzers/scoped", "zz/release/e", "zz/tests/imports-release"}},
		{name: "mixed docs and analyzers", changed: []string{"zz/docs/d/x.go", "zz/analyzers/b/y.go"}, mode: LayerModeLayers,
			areas:    []string{"analyzers", "docs", "release"},
			packages: []string{"zz/analyzers/b", "zz/analyzers/named", "zz/analyzers/one", "zz/analyzers/scoped", "zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "release only", changed: []string{"zz/release/e/x.go"}, mode: LayerModeLayers, areas: []string{"release"},
			packages: []string{"zz/release/e", "zz/tests/imports-release"}},
		{name: "unmapped path", changed: []string{"zz/docs/d/x.go", "zz/nowhere/x.go"}, mode: LayerModeFull, reason: "zz/nowhere/x.go maps to no area"},
		{name: "manifest change", changed: []string{"zz/docs/d/x.go", LayerManifestFile}, mode: LayerModeFull, reason: "layer derivation's inputs"},
		{name: "baseline change", changed: []string{LayerBaselineFile}, mode: LayerModeFull, reason: "layer derivation's inputs"},
		{name: "unbounded set change", changed: []string{LayerUnboundedFile}, mode: LayerModeFull, reason: "layer derivation's inputs"},
		{name: "read scope change", changed: []string{LayerReadScopesFile}, mode: LayerModeFull, reason: "layer derivation's inputs"},
		{name: "go.mod change", changed: []string{"go.mod"}, mode: LayerModeFull, reason: "layer derivation's inputs"},
		{name: "workflow change", changed: []string{"zz/docs/d/x.go", LayerWorkflowFile}, mode: LayerModeFull, reason: "CI workflows"},
		{name: "shard table change", changed: []string{LayerCostsFile}, mode: LayerModeFull, reason: "CI workflows"},
		{name: "no change", mode: LayerModeFull, reason: "no changed path"},
		{name: "unclean path", changed: []string{"zz/docs/../shared/a/x.go"}, mode: LayerModeFull, reason: "not a clean relative path"},
		{name: "declared read scope", changed: []string{"zz/docs/data/table.json"}, mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/analyzers/scoped", "zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "test literal names a directory", changed: []string{"zz/docs/fixtures/in.txt"}, mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/analyzers/named", "zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "package directory holds the change", changed: []string{"zz/docs/d/sub/x.go"},
			edit: func(f *layerFixture) { f.pkgs = append(f.pkgs, LayerPackage{Dir: "zz/docs/d/sub"}) },
			mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/docs/d", "zz/docs/d/sub", "zz/release/e", "zz/tests/imports-release"}},
		{name: "baselined edge joins its importer", changed: []string{"zz/docs/d/x.go"},
			edit: func(f *layerFixture) {
				f.pkgs[1].Imports = append(f.pkgs[1].Imports, "zz/docs/d")
				f.edges = []string{"zz/analyzers/b -> zz/docs/d"}
			},
			mode: LayerModeLayers, areas: []string{"analyzers", "docs", "release"},
			packages: []string{"zz/analyzers/b", "zz/analyzers/named", "zz/analyzers/one", "zz/analyzers/scoped", "zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "entry area always joins", changed: []string{"zz/docs/d/x.go"},
			edit: func(f *layerFixture) { f.areas[2].Entry = true },
			mode: LayerModeLayers, areas: []string{"tests", "docs", "release"},
			packages: []string{"zz/docs/d", "zz/release/e", "zz/tests/c", "zz/tests/imports-release"}},
		{name: "unmapped production package", changed: []string{"zz/docs/d/x.go"},
			edit: func(f *layerFixture) { f.pkgs = append(f.pkgs, LayerPackage{Dir: "zz/nowhere/p"}) },
			mode: LayerModeFull, reason: "zz/nowhere/p has no layer"},
		{name: "wrong-way import", changed: []string{"zz/docs/d/x.go"},
			edit: func(f *layerFixture) { f.pkgs[0].Imports = []string{"zz/docs/d"} },
			mode: LayerModeFull, reason: "against the layer direction"},
		{name: "baselined upward edge narrows", changed: []string{"zz/release/e/x.go"},
			edit: func(f *layerFixture) {
				f.pkgs[8].Imports = append(f.pkgs[8].Imports, "zz/release/e")
				f.edges = []string{"zz/docs/d -> zz/release/e"}
			},
			mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "import of an unmapped directory", changed: []string{"zz/docs/d/x.go"},
			edit: func(f *layerFixture) { f.pkgs[8].Imports = append(f.pkgs[8].Imports, "zz/nowhere/q") },
			mode: LayerModeFull, reason: "imports zz/nowhere/q, which has no layer"},
		{name: "nested prefix: a skipped area's package directory holds the change", changed: []string{"zz/analyzers/b/docs/notes.md"},
			edit: func(f *layerFixture) { f.areas[3].Prefixes = append(f.areas[3].Prefixes, "zz/analyzers/b/docs") },
			mode: LayerModeLayers, areas: []string{"docs", "release"},
			packages: []string{"zz/analyzers/b", "zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
		{name: "universe package unknown", changed: []string{"zz/docs/d/x.go"}, universe: layerPaths("zz/docs/d", "zz/elsewhere"),
			mode: LayerModeFull, reason: "not in the scanned tree"},
		{name: "universe known", changed: []string{"zz/docs/d/x.go"}, universe: layerPaths("zz/docs/d", "zz/shared/a"),
			mode: LayerModeLayers, areas: []string{"docs", "release"}, packages: []string{"zz/docs/d", "zz/release/e", "zz/tests/imports-release"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newLayerFixture()
			if c.edit != nil {
				c.edit(f)
			}
			plan := PlanLayers(f.inputs(t), f.pkgs, c.changed, c.universe)
			if plan.Profile != LayerPlanProfile || plan.Mode != c.mode {
				t.Fatalf("plan %s %s, want %s; reasons %q", plan.Profile, plan.Mode, c.mode, plan.Reasons)
			}
			if c.mode == LayerModeFull {
				if len(plan.Reasons) != 1 || !strings.Contains(plan.Reasons[0], c.reason) || len(plan.Selected) != 0 {
					t.Fatalf("FULL reasons %q selected %d, want one naming %q and no selection", plan.Reasons, len(plan.Selected), c.reason)
				}
				return
			}
			var areas []string
			for _, a := range plan.SelectedAreas {
				areas = append(areas, a.Name)
				if a.Why == "" {
					t.Errorf("area %s selected without a reason", a.Name)
				}
			}
			if !reflect.DeepEqual(areas, c.areas) {
				t.Errorf("areas %q, want %q", areas, c.areas)
			}
			want := layerPaths(append(append([]string{}, always...), c.packages...)...)
			wantSet := map[string]bool{}
			for _, w := range want {
				wantSet[w] = true
			}
			if got := layerKeys(wantSet); !reflect.DeepEqual(plan.Selected, got) {
				t.Errorf("selected %q,\nwant %q", plan.Selected, got)
			}
			for _, x := range plan.Extra {
				if x.Why == "" {
					t.Errorf("extra package %s without a reason", x.Name)
				}
			}
			if len(plan.SkippedAreas)+len(plan.SelectedAreas) != 5 || len(plan.Reasons) != 0 {
				t.Errorf("skipped %q selected %d reasons %q", plan.SkippedAreas, len(plan.SelectedAreas), plan.Reasons)
			}
		})
	}
}

// TestParseLayerInputsRefuses_AFPV02790: a declaration the derivation cannot read exactly is an
// error, which the workflow turns into the whole suite.
func TestParseLayerInputsRefuses_AFPV02790(t *testing.T) {
	cases := map[string]func(*layerFixture){
		"duplicate prefix": func(f *layerFixture) { f.areas[1].Prefixes = append(f.areas[1].Prefixes, "zz/docs") },
		"unclean prefix":   func(f *layerFixture) { f.areas[1].Prefixes = []string{"zz/analyzers/"} },
		"duplicate area":   func(f *layerFixture) { f.areas[1].Name = "shared" },
		"baseline edge":    func(f *layerFixture) { f.edges = []string{"zz/analyzers/b zz/docs/d"} },
		"unbounded unit":   func(f *layerFixture) { f.unbounded = []string{"/zz/tests"} },
		"read scope entry": func(f *layerFixture) { f.scopes["zz/docs/d"] = []string{"../zz"} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			f := newLayerFixture()
			edit(f)
			manifest, _ := json.Marshal(map[string]any{"profile": layerManifestProfile, "module": LayerModule, "areas": f.areas})
			baseline, _ := json.Marshal(map[string]any{"profile": layerBaselineProfile, "edges": f.edges})
			unbounded, _ := json.Marshal(map[string]any{"profile": layerUnboundedProfile, "units": f.unbounded})
			scopes, _ := json.Marshal(map[string]any{"profile": layerReadScopesProfile, "packages": f.scopes})
			if _, err := ParseLayerInputs(manifest, baseline, unbounded, scopes); err == nil {
				t.Fatal("parsed")
			}
		})
	}
	f := newLayerFixture()
	if _, err := ParseLayerInputs([]byte(`{"profile":"other"}`), nil, nil, nil); err == nil {
		t.Fatal("parsed a manifest of another profile")
	}
	_ = f.inputs(t) // the unedited fixture parses
}

// layerShardData returns cost, slice and split tables naming the given package dirs.
func layerShardData(dirs ...string) (costs, slices, split []byte) {
	ms, packages := map[string]int{}, map[string]any{}
	for _, d := range dirs {
		ms[LayerImportPath(d)] = 1
		packages[LayerImportPath(d)] = "x"
	}
	costs, _ = json.Marshal(map[string]any{"milliseconds": ms})
	slices, _ = json.Marshal(map[string]any{"profile": "p", "packages": packages})
	split, _ = json.Marshal(map[string]any{"profile": "p", "packages": packages})
	return costs, slices, split
}

// TestLayerDrift_AFPV02792 fails once for each kind of drift between the manifest and the
// committed shard and layer data, and passes the consistent fixture.
func TestLayerDrift_AFPV02792(t *testing.T) {
	workflow := []byte(strings.Join(LayerWorkflowMarkers, "\n"))
	unmapped := func(f *layerFixture) { f.pkgs = append(f.pkgs, LayerPackage{Dir: "zz/nowhere/p"}) }
	noLayer := "NO_LAYER package zz/nowhere/p"
	cases := []struct {
		name  string
		edit  func(f *layerFixture, costs, slices, split, workflow *[]byte)
		wants []string // problem prefixes, sorted
	}{
		{name: "consistent"},
		{name: "stale cost row of a removed package",
			edit: func(_ *layerFixture, costs, _, _, _ *[]byte) { *costs, _, _ = layerShardData("zz/docs/d", "zz/gone") }},
		{name: "test-only package declared and unbounded",
			edit: func(f *layerFixture, _, _, _, _ *[]byte) {
				f.scopes["zz/orphan"] = []string{}
				f.unbounded = append(f.unbounded, "zz/orphan")
			}},
		{name: "package without a layer", wants: []string{noLayer},
			edit: func(f *layerFixture, _, _, _, _ *[]byte) { unmapped(f) }},
		{name: "prefix without a package", wants: []string{"EMPTY_PREFIX area docs prefix zz/docs-gone"},
			edit: func(f *layerFixture, _, _, _, _ *[]byte) {
				f.areas[3].Prefixes = append(f.areas[3].Prefixes, "zz/docs-gone")
			}},
		{name: "wrong-way import", wants: []string{"WRONG_WAY zz/shared/a (shared) imports zz/docs/d (docs)"},
			edit: func(f *layerFixture, _, _, _, _ *[]byte) { f.pkgs[0].Imports = []string{"zz/docs/d"} }},
		{name: "baselined wrong-way import",
			edit: func(f *layerFixture, _, _, _, _ *[]byte) {
				f.pkgs[8].Imports = append(f.pkgs[8].Imports, "zz/release/e")
				f.edges = []string{"zz/docs/d -> zz/release/e"}
			}},
		{name: "unreadable cost table", wants: []string{"SHARD_DATA " + LayerCostsFile + ` has no readable "milliseconds" object`},
			edit: func(_ *layerFixture, costs, _, _, _ *[]byte) { *costs = []byte(`{"milliseconds": []}`) }},
		{name: "cost key outside the module", wants: []string{"SHARD_DATA " + LayerCostsFile + " names example.com/other/x, which is not a package of"},
			edit: func(_ *layerFixture, costs, _, _, _ *[]byte) {
				*costs = []byte(`{"milliseconds": {"example.com/other/x": 1}}`)
			}},
		{name: "cost key without an area", wants: []string{noLayer, "SHARD_DATA " + LayerCostsFile + " names " + LayerImportPath("zz/nowhere/p") + ", which maps to no area"},
			edit: func(f *layerFixture, costs, _, _, _ *[]byte) {
				unmapped(f)
				*costs, _, _ = layerShardData("zz/nowhere/p")
			}},
		{name: "slice key without an area", wants: []string{noLayer, "SHARD_DATA " + LayerSlicesFile + " names " + LayerImportPath("zz/nowhere/p")},
			edit: func(f *layerFixture, _, slices, _, _ *[]byte) {
				unmapped(f)
				_, *slices, _ = layerShardData("zz/nowhere/p")
			}},
		{name: "split key without an area", wants: []string{noLayer, "SHARD_DATA " + LayerSplitFile + " names " + LayerImportPath("zz/nowhere/p")},
			edit: func(f *layerFixture, _, _, split, _ *[]byte) {
				unmapped(f)
				_, _, *split = layerShardData("zz/nowhere/p")
			}},
		{name: "unbounded unit without an area", wants: []string{noLayer, "SHARD_DATA " + LayerUnboundedFile + " unit zz/nowhere/p"},
			edit: func(f *layerFixture, _, _, _, _ *[]byte) {
				unmapped(f)
				f.unbounded = append(f.unbounded, "zz/nowhere/p")
			}},
		{name: "read scope package without an area", wants: []string{noLayer, "SHARD_DATA " + LayerReadScopesFile + " package zz/nowhere/p"},
			edit: func(f *layerFixture, _, _, _, _ *[]byte) { unmapped(f); f.scopes["zz/nowhere/p"] = []string{} }},
		{name: "workflow lost its wiring", wants: []string{"WORKFLOW " + LayerWorkflowFile + " lost the layer selection wiring"},
			edit: func(_ *layerFixture, _, _, _, workflow *[]byte) {
				*workflow = bytes.Replace(*workflow, []byte(LayerWorkflowMarkers[0]), nil, 1)
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newLayerFixture()
			costs, slices, split := layerShardData("zz/docs/d", "zz/release/e")
			wf := append([]byte{}, workflow...)
			if c.edit != nil {
				c.edit(f, &costs, &slices, &split, &wf)
			}
			problems := LayerDrift(f.inputs(t), f.pkgs, costs, slices, split, wf)
			ok := len(problems) == len(c.wants)
			for i := 0; ok && i < len(problems); i++ {
				ok = strings.HasPrefix(problems[i], c.wants[i])
			}
			if !ok {
				t.Fatalf("problems %q,\nwant prefixes %q", problems, c.wants)
			}
		})
	}
}

// TestScanLayerPackages_AFPV02790 holds the scan on a tracked tree: testdata, vendor,
// node_modules, "_" directories, nested modules and untracked files hold no package, and each
// package keeps its module imports, test imports, test literal paths and test-only state.
func TestScanLayerPackages_AFPV02790(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":               "module " + LayerModule + "\n",
		"a/a.go":               "package a\n\nimport (\n\t\"fmt\"\n\n\t\"" + LayerModule + "/b\"\n)\n\nvar _ = fmt.Sprint\n",
		"a/a_test.go":          "package a\n\nvar _ = \"testdata/in.txt\"\n",
		"b/b.go":               "package b\n",
		"d/d_test.go":          "package d\n\nimport _ \"" + LayerModule + "/b\"\n",
		"a/testdata/t.go":      "package t\n",
		"b/testdata/deep/y.go": "package y\n",
		"_x/x.go":              "package x\n",
		"vendor/v/v.go":        "package v\n",
		"node_modules/n/n.go":  "package n\n",
		"nested/go.mod":        "module example.com/nested\n",
		"nested/n.go":          "package nested\n",
		"nested/inner/i.go":    "package inner\n",
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package corvint\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, err := ScanLayerPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	none := []string{}
	want := []LayerPackage{
		{Dir: "a", Imports: []string{"b"}, TestImports: none, Refs: []string{"a/testdata/in.txt", "testdata/in.txt"}},
		{Dir: "b", Imports: none, TestImports: none, Refs: none},
		{Dir: "d", TestOnly: true, Imports: none, TestImports: []string{"b"},
			Refs: []string{"d/" + LayerModule + "/b", LayerModule + "/b"}},
	}
	if !reflect.DeepEqual(pkgs, want) {
		t.Fatalf("scanned %+v,\nwant %+v", pkgs, want)
	}
}

// TestLayerSummary_AFPV02793: the job summary names the mode and why, the selected areas and
// their reasons, the skipped areas and each changed path's area.
func TestLayerSummary_AFPV02793(t *testing.T) {
	f := newLayerFixture()
	for _, c := range []struct {
		changed []string
		wants   []string
	}{
		{[]string{"zz/docs/d/x.go"}, []string{"Layer selection (AFP-V0-2791): LAYERS", "Changed areas: docs", "Skipped areas: shared, analyzers, tests",
			"| `release` | `layer 2 may import docs (layer 1)` |", "| `zz/tests/unbounded` | `unbounded reader", "| `zz/docs/d/x.go` | docs |"}},
		{[]string{"zz/a|b"}, []string{"Layer selection (AFP-V0-2791): FULL", "Every shard runs its whole partition: zz/a|b maps to no area", "| `zz/a\\|b` | (none) |"}},
	} {
		var b bytes.Buffer
		if err := WriteLayerSummary(&b, PlanLayers(f.inputs(t), f.pkgs, c.changed, nil)); err != nil {
			t.Fatal(err)
		}
		for _, want := range c.wants {
			if !strings.Contains(b.String(), want) {
				t.Errorf("summary lacks %q:\n%s", want, b.String())
			}
		}
	}
}

// TestLayerDriftLive_AFPV02792 holds the checkout: every Go package of the tree has a layer, every
// manifest prefix holds a package, the shard tables and declarations map to areas, ci.yml keeps
// the wiring, and a change inside the docs layer leaves the shared, analyzer and test areas out.
func TestLayerDriftLive_AFPV02792(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	in, err := LoadLayerInputs(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := ScanLayerPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	var raw [4][]byte
	for i, name := range []string{LayerCostsFile, LayerSlicesFile, LayerSplitFile, LayerWorkflowFile} {
		if raw[i], err = os.ReadFile(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range LayerDrift(in, pkgs, raw[0], raw[1], raw[2], raw[3]) {
		t.Error(p)
	}
	var docs *LayerArea
	for i := range in.Areas {
		if in.Areas[i].Name == "docs" {
			docs = &in.Areas[i]
		}
	}
	if docs == nil {
		t.Fatalf("%s has no docs area", LayerManifestFile)
	}
	plan := PlanLayers(in, pkgs, []string{docs.Prefixes[0] + "/layer_probe.go"}, nil)
	if plan.Mode != LayerModeLayers || !reflect.DeepEqual(plan.ChangedAreas, []string{"docs"}) {
		t.Fatalf("a docs-layer change plans %s %q (areas %q)", plan.Mode, plan.Reasons, plan.ChangedAreas)
	}
	skipped := strings.Join(plan.SkippedAreas, " ")
	for _, area := range []string{"shared", "analyzers", "tests"} {
		if !strings.Contains(" "+skipped+" ", " "+area+" ") {
			t.Errorf("a docs-layer change selects area %s (skipped: %s)", area, skipped)
		}
	}
	t.Logf("docs-layer change: %d of %d packages selected, areas %v", len(plan.Selected), plan.Known, plan.SelectedAreas)
}
