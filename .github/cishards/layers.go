package cishards

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// AFP-V0-2790..2793 (V1-5235): pull-request shards select their packages by the layers of
// decision 0594 item 8. tools/import-ratchet/areas.json maps every package to an area and each
// area to a layer, and the ratchet holds the import direction: a package imports only its own
// area or a strictly lower layer, except for baselined edges and entry areas. A change in one
// area can therefore reach only that area, every area of a higher layer, the areas a baselined
// edge or an entry area joins to it, and the packages whose tests import, declare or name what
// changed. Anything the derivation cannot place, or any doubt, selects the whole suite.
//
// This file is not one of the files the isolated helper modules of the go-product shards copy
// for partitioning; the layer selection step copies it with layerplan/main.go into its own
// isolated module, and the go-static job runs its tests in the checkout.

// The repository inputs of the layer derivation, relative to the repository root.
const (
	LayerModule         = "github.com/Beamfall/corvint"
	LayerManifestFile   = "tools/import-ratchet/areas.json"
	LayerBaselineFile   = "tools/import-ratchet/baseline.json"
	LayerUnboundedFile  = ".corvint/unbounded-readers.json"
	LayerReadScopesFile = ".corvint/test-read-scopes.json"
	LayerWorkflowFile   = ".github/workflows/ci.yml"
	LayerCostsFile      = ".github/cishards/package-costs.json"
	LayerSlicesFile     = ".github/cishards/test-slices.json"
	LayerSplitFile      = ".github/cishards/test-split-allow.json"

	LayerPlanProfile = "corvint-ci-layer-plan/0"
	LayerModeLayers  = "LAYERS"
	LayerModeFull    = "FULL"

	layerManifestProfile   = "corvint-area-manifest/0"
	layerBaselineProfile   = "corvint-import-ratchet-baseline/0"
	layerUnboundedProfile  = "corvint-unbounded-reader-set/0"
	layerReadScopesProfile = "corvint-test-read-scopes/0"
)

// layerControlFiles change what the derivation itself computes, so a change to any of them, or
// to anything under .github, selects the whole suite.
var layerControlFiles = []string{LayerManifestFile, LayerBaselineFile, LayerUnboundedFile, LayerReadScopesFile, "go.mod", "go.sum"}

// LayerWorkflowMarkers are the ci.yml lines that wire the layer selection; the drift check
// fails when one is missing (AFP-V0-2792).
var LayerWorkflowMarkers = []string{
	"        id: layers\n",
	"        id: layers-record\n",
	`"$RUNNER_TEMP/ci-layers" --root . --changed`,
	"LAYER_TEST_P: ${{ github.event_name == 'pull_request' && '4' || '1' }}\n",
	`export SHARD_TEST_P="$LAYER_TEST_P"`,
	"needs.narrowing-verdict.outputs.verdict != 'MET'",
	`[ "${LAYER_MODE:-}" = LAYERS ]`,
	"name: ci-narrowing-${{ strategy.job-index }}\n          # AFP-V0-2791",
	"needs.go-product-shard.outputs.layers != 'LAYERS'",
	"_AFPV0279[0-3]$",
}

// LayerArea is one area of the manifest.
type LayerArea struct {
	Name     string   `json:"name"`
	Layer    int      `json:"layer"`
	Entry    bool     `json:"entry"`
	Prefixes []string `json:"prefixes"`
}

// LayerInputs are the parsed repository declarations the derivation reads.
type LayerInputs struct {
	Module     string
	Areas      []LayerArea
	Edges      [][2]string         // baselined wrong-way imports, module-relative package dirs
	Unbounded  []string            // test units selected on every change (AFP-V0-012 rule (d))
	ReadScopes map[string][]string // declared test read scopes by package dir (AFP-V0-023)
}

// LayerPackage is one Go package directory of the tree.
type LayerPackage struct {
	Dir         string   // module-relative, slash-separated; "." for the module root
	TestOnly    bool     // only _test.go files: nothing can import it and the ratchet gives it no area
	Imports     []string // module-relative dirs its non-test files import
	TestImports []string // module-relative dirs its _test.go files import
	Refs        []string // repository paths its _test.go string literals may name
}

// LayerChange is one changed path and the area it maps to ("" when none).
type LayerChange struct {
	Path string `json:"path"`
	Area string `json:"area"`
}

// LayerSelect is a selected area or an extra package and why it was selected.
type LayerSelect struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// LayerPlan is the derivation's result. Mode LAYERS runs only Selected; FULL runs everything.
type LayerPlan struct {
	Profile       string        `json:"profile"`
	Mode          string        `json:"mode"`
	Reasons       []string      `json:"reasons"`
	Changed       []LayerChange `json:"changed"`
	ChangedAreas  []string      `json:"changedAreas"`
	SelectedAreas []LayerSelect `json:"selectedAreas"`
	SkippedAreas  []string      `json:"skippedAreas"`
	Extra         []LayerSelect `json:"extra"`
	Selected      []string      `json:"selected"`
	Known         int           `json:"known"`
}

// LoadLayerInputs reads the derivation's declarations from the repository at root.
func LoadLayerInputs(root string) (LayerInputs, error) {
	var raw [4][]byte
	for i, name := range []string{LayerManifestFile, LayerBaselineFile, LayerUnboundedFile, LayerReadScopesFile} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return LayerInputs{}, fmt.Errorf("layers: %w", err)
		}
		raw[i] = data
	}
	return ParseLayerInputs(raw[0], raw[1], raw[2], raw[3])
}

// ParseLayerInputs parses the area manifest, the ratchet baseline, the unbounded-reader set and
// the test read-scope declaration, and refuses any document it cannot read exactly.
func ParseLayerInputs(manifest, baseline, unbounded, scopes []byte) (LayerInputs, error) {
	var m struct {
		Profile string      `json:"profile"`
		Module  string      `json:"module"`
		Areas   []LayerArea `json:"areas"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		return LayerInputs{}, fmt.Errorf("layers: %s: %w", LayerManifestFile, err)
	}
	if m.Profile != layerManifestProfile || m.Module != LayerModule || len(m.Areas) == 0 {
		return LayerInputs{}, fmt.Errorf("layers: %s: profile %q module %q with %d areas, want %s of %s with areas",
			LayerManifestFile, m.Profile, m.Module, len(m.Areas), layerManifestProfile, LayerModule)
	}
	names, prefixes := map[string]bool{}, map[string]string{}
	for _, a := range m.Areas {
		if a.Name == "" || names[a.Name] || a.Layer < 0 || len(a.Prefixes) == 0 {
			return LayerInputs{}, fmt.Errorf("layers: %s: area %q is unnamed, repeated, negative or has no prefix", LayerManifestFile, a.Name)
		}
		names[a.Name] = true
		for _, p := range a.Prefixes {
			if !layerCleanPath(p) {
				return LayerInputs{}, fmt.Errorf("layers: %s: area %s prefix %q is not a clean relative path", LayerManifestFile, a.Name, p)
			}
			if other, dup := prefixes[p]; dup {
				return LayerInputs{}, fmt.Errorf("layers: %s: prefix %q is in areas %s and %s", LayerManifestFile, p, other, a.Name)
			}
			prefixes[p] = a.Name
		}
	}
	in := LayerInputs{Module: m.Module, Areas: m.Areas, ReadScopes: map[string][]string{}}

	var b struct {
		Profile string   `json:"profile"`
		Edges   []string `json:"edges"`
	}
	if err := json.Unmarshal(baseline, &b); err != nil || b.Profile != layerBaselineProfile {
		return LayerInputs{}, fmt.Errorf("layers: %s: profile %q, want %s (%v)", LayerBaselineFile, b.Profile, layerBaselineProfile, err)
	}
	for _, e := range b.Edges {
		from, to, ok := strings.Cut(e, " -> ")
		if !ok || !layerCleanPath(from) || !layerCleanPath(to) {
			return LayerInputs{}, fmt.Errorf("layers: %s: edge %q is not \"from -> to\"", LayerBaselineFile, e)
		}
		in.Edges = append(in.Edges, [2]string{from, to})
	}

	var u struct {
		Profile string   `json:"profile"`
		Units   []string `json:"units"`
	}
	if err := json.Unmarshal(unbounded, &u); err != nil || u.Profile != layerUnboundedProfile {
		return LayerInputs{}, fmt.Errorf("layers: %s: profile %q, want %s (%v)", LayerUnboundedFile, u.Profile, layerUnboundedProfile, err)
	}
	for _, unit := range u.Units {
		if !layerCleanPath(unit) {
			return LayerInputs{}, fmt.Errorf("layers: %s: unit %q is not a clean relative path", LayerUnboundedFile, unit)
		}
	}
	in.Unbounded = u.Units

	var s struct {
		Profile  string              `json:"profile"`
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(scopes, &s); err != nil || s.Profile != layerReadScopesProfile {
		return LayerInputs{}, fmt.Errorf("layers: %s: profile %q, want %s (%v)", LayerReadScopesFile, s.Profile, layerReadScopesProfile, err)
	}
	for dir, entries := range s.Packages {
		if !layerCleanPath(dir) {
			return LayerInputs{}, fmt.Errorf("layers: %s: package %q is not a clean relative path", LayerReadScopesFile, dir)
		}
		for _, entry := range entries {
			if !layerCleanPath(strings.TrimSuffix(entry, "/")) {
				return LayerInputs{}, fmt.Errorf("layers: %s: %s entry %q is not a clean relative path", LayerReadScopesFile, dir, entry)
			}
		}
		in.ReadScopes[dir] = entries
	}
	return in, nil
}

// layerCleanPath reports whether p is a canonical, non-empty, relative slash path inside the tree.
func layerCleanPath(p string) bool {
	return p != "" && p != "." && path.Clean(p) == p && !path.IsAbs(p) && p != ".." && !strings.HasPrefix(p, "../") &&
		!strings.ContainsAny(p, "\\\x00\n")
}

// AreaOf returns the area whose prefix is the longest path-segment match for rel, as the import
// ratchet maps packages, or nil when none matches.
func (in LayerInputs) AreaOf(rel string) *LayerArea {
	var found *LayerArea
	longest := -1
	for i := range in.Areas {
		for _, p := range in.Areas[i].Prefixes {
			if (rel == p || strings.HasPrefix(rel, p+"/")) && len(p) > longest {
				found, longest = &in.Areas[i], len(p)
			}
		}
	}
	return found
}

// ScanLayerPackages lists the Go packages of the repository at root from its tracked files: every
// directory with a .go file outside testdata, vendor, node_modules, "_" directories and nested
// modules. Hidden directories stay in, so .github packages need a layer too. It records each
// package's module-internal imports and the paths its test files' string literals may name.
func ScanLayerPackages(root string) ([]LayerPackage, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go", "*go.mod").Output()
	if err != nil {
		return nil, fmt.Errorf("layers: git ls-files in %s: %w", root, err)
	}
	var files, modules []string
	for _, name := range strings.Split(string(bytes.TrimRight(out, "\x00")), "\x00") {
		switch {
		case name == "" || name == "go.mod":
		case path.Base(name) == "go.mod":
			modules = append(modules, path.Dir(name)+"/")
		case strings.HasSuffix(name, ".go"):
			files = append(files, name)
		}
	}
	type acc struct {
		production                 bool
		imports, testImports, refs map[string]bool
	}
	byDir := map[string]*acc{}
	for _, name := range files {
		if !layerSource(name, modules) {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return nil, fmt.Errorf("layers: %w", err)
		}
		dir := path.Dir(name)
		a := byDir[dir]
		if a == nil {
			a = &acc{imports: map[string]bool{}, testImports: map[string]bool{}, refs: map[string]bool{}}
			byDir[dir] = a
		}
		test := strings.HasSuffix(name, "_test.go")
		a.production = a.production || !test
		mode := parser.ImportsOnly | parser.SkipObjectResolution
		if test {
			mode = parser.SkipObjectResolution
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, src, mode)
		if err != nil {
			return nil, fmt.Errorf("layers: %w", err)
		}
		into := a.imports
		if test {
			into = a.testImports
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("layers: %s: import %s: %w", name, spec.Path.Value, err)
			}
			if rel, ok := layerModuleDir(imported); ok {
				into[rel] = true
			}
		}
		if test {
			ast.Inspect(file, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if v, err := strconv.Unquote(lit.Value); err == nil {
					for _, ref := range layerRefs(dir, v) {
						a.refs[ref] = true
					}
				}
				return true
			})
		}
	}
	packages := make([]LayerPackage, 0, len(byDir))
	for dir, a := range byDir {
		packages = append(packages, LayerPackage{Dir: dir, TestOnly: !a.production, Imports: layerKeys(a.imports), TestImports: layerKeys(a.testImports), Refs: layerKeys(a.refs)})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Dir < packages[j].Dir })
	return packages, nil
}

// layerSource reports whether a tracked .go file belongs to a package of the root module.
func layerSource(name string, modules []string) bool {
	for _, module := range modules {
		if strings.HasPrefix(name, module) {
			return false
		}
	}
	if dir := path.Dir(name); dir != "." {
		for _, part := range strings.Split(dir, "/") {
			if part == "testdata" || part == "vendor" || part == "node_modules" || strings.HasPrefix(part, "_") {
				return false
			}
		}
	}
	return true
}

// layerModuleDir returns the module-relative dir of an import path of this module.
func layerModuleDir(imported string) (string, bool) {
	if imported == LayerModule {
		return ".", true
	}
	rel, ok := strings.CutPrefix(imported, LayerModule+"/")
	return rel, ok && rel != ""
}

// layerRefs returns the repository paths a test string literal in dir may name: the literal as a
// repository-relative path and resolved against the package directory.
func layerRefs(dir, v string) []string {
	if v == "" || len(v) > 512 || strings.ContainsAny(v, "\x00\n\r\t*?") || path.IsAbs(v) || strings.Contains(v, "://") {
		return nil
	}
	v = filepath.ToSlash(v)
	var refs []string
	for _, ref := range []string{path.Clean(v), path.Clean(path.Join(dir, v))} {
		if layerCleanPath(ref) {
			refs = append(refs, ref)
		}
	}
	return refs
}

func layerKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LayerImportPath is the import path of a module-relative package dir.
func LayerImportPath(dir string) string {
	if dir == "." {
		return LayerModule
	}
	return LayerModule + "/" + dir
}

// layerUnder reports whether p is dir or a path below it.
func layerUnder(p, dir string) bool {
	return dir == "." || p == dir || strings.HasPrefix(p, dir+"/")
}

// PlanLayers derives the packages a change set needs. universe, when not nil, is the import path
// list the shards partition (go list ./...); a universe package the scan does not know selects
// the whole suite. It never fails: every doubt is a FULL plan with its reason.
func PlanLayers(in LayerInputs, pkgs []LayerPackage, changed []string, universe []string) LayerPlan {
	plan := LayerPlan{Profile: LayerPlanProfile, Mode: LayerModeFull, Reasons: []string{}, Changed: []LayerChange{}, ChangedAreas: []string{},
		SelectedAreas: []LayerSelect{}, SkippedAreas: []string{}, Extra: []LayerSelect{}, Selected: []string{}, Known: len(pkgs)}
	full := func(format string, args ...any) LayerPlan {
		plan.Reasons = append(plan.Reasons, fmt.Sprintf(format, args...))
		return plan
	}
	known := map[string]*LayerPackage{}
	for i := range pkgs {
		known[pkgs[i].Dir] = &pkgs[i]
	}
	byName := map[string]*LayerArea{}
	for i := range in.Areas {
		byName[in.Areas[i].Name] = &in.Areas[i]
	}

	// Every package must have a layer and keep the import direction, or the layers prove nothing.
	edges := map[[2]string]bool{}
	for _, e := range in.Edges {
		edges[e] = true
	}
	unmapped := map[string]bool{} // production packages of the tree without an area
	for _, p := range pkgs {
		if !p.TestOnly && in.AreaOf(p.Dir) == nil {
			unmapped[p.Dir] = true
		}
	}
	for _, p := range pkgs {
		from := in.AreaOf(p.Dir)
		if from == nil && p.TestOnly {
			continue // selected on every change below
		}
		if from == nil {
			return full("package %s has no layer in %s", p.Dir, LayerManifestFile)
		}
		for _, imported := range p.Imports {
			to := in.AreaOf(imported)
			if to == nil {
				return full("package %s imports %s, which has no layer", p.Dir, imported)
			}
			if from.Name != to.Name && !from.Entry && to.Layer >= from.Layer && !edges[[2]string{p.Dir, imported}] {
				return full("package %s (%s, layer %d) imports %s (%s, layer %d) against the layer direction", p.Dir, from.Name, from.Layer, imported, to.Name, to.Layer)
			}
		}
	}
	for _, ip := range universe {
		dir, ok := layerModuleDir(ip)
		if !ok || known[dir] == nil {
			return full("the shard universe package %s is not in the scanned tree", ip)
		}
	}

	if len(changed) == 0 {
		return full("no changed path was found")
	}
	changedAreas := map[string]bool{}
	for _, p := range changed {
		a := in.AreaOf(p)
		c := LayerChange{Path: p}
		if a != nil {
			c.Area = a.Name
		}
		plan.Changed = append(plan.Changed, c)
	}
	for _, c := range plan.Changed {
		switch {
		case !layerCleanPath(c.Path):
			return full("changed path %q is not a clean relative path", c.Path)
		case c.Path == ".github" || strings.HasPrefix(c.Path, ".github/"):
			return full("%s changes the CI workflows or their helpers", c.Path)
		case layerControl(c.Path):
			return full("%s changes the layer derivation's inputs", c.Path)
		case c.Area == "":
			return full("%s maps to no area of %s", c.Path, LayerManifestFile)
		}
		changedAreas[c.Area] = true
	}
	plan.ChangedAreas = layerKeys(changedAreas)

	// Dependent areas, to a fixpoint: an entry area, an area of a higher layer than a selected
	// one, and the importing side of a baselined edge into a selected area.
	why := map[string]string{}
	for name := range changedAreas {
		why[name] = "changed"
	}
	for grew := true; grew; {
		grew = false
		for _, a := range in.Areas {
			if why[a.Name] != "" {
				continue
			}
			reason := ""
			if a.Entry {
				reason = "entry area: imports any layer"
			}
			for name := range why {
				if reason == "" && a.Layer > byName[name].Layer {
					reason = fmt.Sprintf("layer %d may import %s (layer %d)", a.Layer, name, byName[name].Layer)
				}
			}
			for e := range edges {
				from, to := in.AreaOf(e[0]), in.AreaOf(e[1])
				if reason == "" && from != nil && to != nil && from.Name == a.Name && why[to.Name] != "" {
					reason = fmt.Sprintf("baselined import %s -> %s", e[0], e[1])
				}
			}
			if reason != "" {
				why[a.Name], grew = reason, true
			}
		}
	}
	if len(why) == len(in.Areas) {
		return full("every area depends on the changed areas %s", strings.Join(plan.ChangedAreas, ", "))
	}
	for _, a := range in.Areas {
		if why[a.Name] != "" {
			plan.SelectedAreas = append(plan.SelectedAreas, LayerSelect{Name: a.Name, Why: why[a.Name]})
		} else {
			plan.SkippedAreas = append(plan.SkippedAreas, a.Name)
		}
	}

	unbounded := map[string]bool{}
	for _, u := range in.Unbounded {
		unbounded[u] = true
	}
	selectedDir := func(dir string) bool {
		a := in.AreaOf(dir)
		return a != nil && why[a.Name] != ""
	}
	for _, p := range pkgs {
		reason := ""
		switch {
		case selectedDir(p.Dir):
		case unbounded[p.Dir]:
			reason = "unbounded reader (" + LayerUnboundedFile + ")"
		case p.TestOnly && in.AreaOf(p.Dir) == nil:
			reason = "test-only package without an area"
		default:
			for _, imported := range p.TestImports {
				if reason == "" && selectedDir(imported) {
					reason = "its tests import " + imported
				}
			}
			for _, c := range changed {
				if reason != "" {
					break
				}
				switch {
				case layerUnder(c, p.Dir):
					reason = "its directory holds " + c
				case layerScopeHolds(in.ReadScopes[p.Dir], c):
					reason = "its declared read scope holds " + c
				case layerRefHolds(p.Refs, c) != "":
					reason = "its tests name " + layerRefHolds(p.Refs, c)
				}
			}
			if reason == "" {
				continue
			}
		}
		if reason != "" {
			plan.Extra = append(plan.Extra, LayerSelect{Name: p.Dir, Why: reason})
		}
		plan.Selected = append(plan.Selected, LayerImportPath(p.Dir))
	}
	sort.Strings(plan.Selected)
	if len(plan.Selected) == 0 {
		return full("the selection names no package")
	}
	plan.Mode = LayerModeLayers
	return plan
}

func layerControl(p string) bool {
	for _, c := range layerControlFiles {
		if p == c {
			return true
		}
	}
	return false
}

// layerScopeHolds mirrors the AFP-V0-023 grammar: an entry names a file, or a subtree with "/".
func layerScopeHolds(entries []string, p string) bool {
	for _, e := range entries {
		if p == strings.TrimSuffix(e, "/") || strings.HasPrefix(p, strings.TrimSuffix(e, "/")+"/") {
			return true
		}
	}
	return false
}

// layerRefHolds returns the longest literal path that names p, or a directory of two or more
// segments above it: a one-segment literal such as "internal" is a path element far more often
// than a read of a whole top-level tree, and a test that walks one is an unbounded reader.
func layerRefHolds(refs []string, p string) string {
	held := ""
	for _, r := range refs {
		if (p == r || (strings.Contains(r, "/") && strings.HasPrefix(p, r+"/"))) && len(r) > len(held) {
			held = r
		}
	}
	return held
}

// LayerDrift checks the committed layer and shard data against the manifest (AFP-V0-2792): a
// package without a layer, a manifest prefix that holds no package, a non-test import against
// the layer direction outside the baseline, an unreadable shard table or a key outside the
// module, a cost, slice or split key, unbounded unit or read-scope package that names a
// production package of the tree with no area, and ci.yml losing the layer selection's wiring.
// A shard-table row for a package the tree no longer holds selects nothing here; it is the cost
// tables' own staleness, not layer drift.
func LayerDrift(in LayerInputs, pkgs []LayerPackage, costs, slices, split, workflow []byte) []string {
	var problems []string
	edges := map[[2]string]bool{}
	for _, e := range in.Edges {
		edges[e] = true
	}
	unmapped := map[string]bool{} // production packages of the tree without an area
	for _, p := range pkgs {
		if !p.TestOnly && in.AreaOf(p.Dir) == nil {
			unmapped[p.Dir] = true
		}
	}
	for _, p := range pkgs {
		from := in.AreaOf(p.Dir)
		if from == nil && p.TestOnly {
			continue // the ratchet maps production packages only; PlanLayers always selects it
		}
		if from == nil {
			problems = append(problems, fmt.Sprintf("NO_LAYER package %s matches no prefix of %s", p.Dir, LayerManifestFile))
			continue
		}
		for _, imported := range p.Imports {
			if to := in.AreaOf(imported); to != nil && from.Name != to.Name && !from.Entry && to.Layer >= from.Layer && !edges[[2]string{p.Dir, imported}] {
				problems = append(problems, fmt.Sprintf("WRONG_WAY %s (%s) imports %s (%s)", p.Dir, from.Name, imported, to.Name))
			}
		}
	}
	for _, a := range in.Areas {
		for _, prefix := range a.Prefixes {
			held := false
			for _, p := range pkgs {
				held = held || layerUnder(p.Dir, prefix)
			}
			if !held {
				problems = append(problems, fmt.Sprintf("EMPTY_PREFIX area %s prefix %s holds no Go package", a.Name, prefix))
			}
		}
	}
	for _, table := range []struct {
		file, field string
		raw         []byte
	}{{LayerCostsFile, "milliseconds", costs}, {LayerSlicesFile, "packages", slices}, {LayerSplitFile, "packages", split}} {
		var doc map[string]json.RawMessage
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(table.raw, &doc); err != nil || json.Unmarshal(doc[table.field], &keys) != nil {
			problems = append(problems, fmt.Sprintf("SHARD_DATA %s has no readable %q object", table.file, table.field))
			continue
		}
		for _, key := range layerKeys(layerSet(keys)) {
			if dir, ok := layerModuleDir(key); !ok {
				problems = append(problems, fmt.Sprintf("SHARD_DATA %s names %s, which is not a package of %s", table.file, key, in.Module))
			} else if unmapped[dir] {
				problems = append(problems, fmt.Sprintf("SHARD_DATA %s names %s, which maps to no area", table.file, key))
			}
		}
	}
	for _, u := range in.Unbounded {
		if unmapped[u] {
			problems = append(problems, fmt.Sprintf("SHARD_DATA %s unit %s maps to no area", LayerUnboundedFile, u))
		}
	}
	for _, dir := range layerKeys(layerScopeSet(in.ReadScopes)) {
		if unmapped[dir] {
			problems = append(problems, fmt.Sprintf("SHARD_DATA %s package %s maps to no area", LayerReadScopesFile, dir))
		}
	}
	for _, marker := range LayerWorkflowMarkers {
		if !bytes.Contains(workflow, []byte(marker)) {
			problems = append(problems, fmt.Sprintf("WORKFLOW %s lost the layer selection wiring %q", LayerWorkflowFile, marker))
		}
	}
	sort.Strings(problems)
	return problems
}

func layerSet(m map[string]json.RawMessage) map[string]bool {
	set := map[string]bool{}
	for k := range m {
		set[k] = true
	}
	return set
}

func layerScopeSet(m map[string][]string) map[string]bool {
	set := map[string]bool{}
	for k := range m {
		set[k] = true
	}
	return set
}

// WriteLayerSummary writes the plan as the Markdown the shard step appends to the job summary
// (AFP-V0-2793): the mode and why, each changed path and its area, the selected areas and why,
// the skipped areas and the packages selected outside them.
func WriteLayerSummary(w io.Writer, plan LayerPlan) error {
	var b strings.Builder
	fmt.Fprintf(&b, "### Layer selection (AFP-V0-2791): %s\n\n", plan.Mode)
	if plan.Mode == LayerModeFull {
		fmt.Fprintf(&b, "Every shard runs its whole partition: %s.\n\n", strings.Join(plan.Reasons, "; "))
	} else {
		fmt.Fprintf(&b, "%d of %d packages selected. Changed areas: %s. Skipped areas: %s. The merge queue runs the full suite.\n\n",
			len(plan.Selected), plan.Known, strings.Join(plan.ChangedAreas, ", "), strings.Join(plan.SkippedAreas, ", "))
	}
	if len(plan.SelectedAreas) > 0 {
		b.WriteString("| Selected area | Why |\n|---|---|\n")
		for _, s := range plan.SelectedAreas {
			fmt.Fprintf(&b, "| %s | %s |\n", layerCell(s.Name), layerCell(s.Why))
		}
		b.WriteString("\n")
	}
	if len(plan.Extra) > 0 {
		b.WriteString("| Package outside the selected areas | Why |\n|---|---|\n")
		for _, s := range plan.Extra {
			fmt.Fprintf(&b, "| %s | %s |\n", layerCell(s.Name), layerCell(s.Why))
		}
		b.WriteString("\n")
	}
	if len(plan.Changed) > 0 {
		b.WriteString("| Changed path | Area |\n|---|---|\n")
		for i, c := range plan.Changed {
			if i == 200 {
				fmt.Fprintf(&b, "| ... %d more | |\n", len(plan.Changed)-i)
				break
			}
			area := c.Area
			if area == "" {
				area = "(none)"
			}
			fmt.Fprintf(&b, "| %s | %s |\n", layerCell(c.Path), area)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func layerCell(s string) string {
	return "`" + strings.NewReplacer("`", "'", "|", "\\|", "\n", " ").Replace(s) + "`"
}
