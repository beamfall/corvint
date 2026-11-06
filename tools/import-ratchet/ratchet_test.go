//corvint:read-kinds .go go.mod

package importratchet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	manifestPath  = "tools/import-ratchet/areas.json"
	baselinePath  = "tools/import-ratchet/baseline.json"
	manifestShape = "corvint-area-manifest/0"
	baselineShape = "corvint-import-ratchet-baseline/0"
)

// area is one layered group of packages in areas.json (IRT-V0-001).
type area struct {
	Name     string   `json:"name"`
	Layer    int      `json:"layer"`
	Entry    bool     `json:"entry"`
	Summary  string   `json:"summary"`
	Prefixes []string `json:"prefixes"`
}

type manifest struct {
	Profile string `json:"profile"`
	Module  string `json:"module"`
	Rule    string `json:"rule"`
	Areas   []area `json:"areas"`
}

type baseline struct {
	Profile string   `json:"profile"`
	Edges   []string `json:"edges"`
}

// decodeStrict decodes one JSON document and refuses unknown fields, so a
// misspelt key cannot silently drop an area or the baseline.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// validate reports manifest defects: a wrong profile, an empty or duplicate
// area name, a negative layer, or a prefix that is empty, malformed or claimed
// twice.
func (m *manifest) validate() []string {
	var problems []string
	if m.Profile != manifestShape {
		problems = append(problems, fmt.Sprintf("MANIFEST_FORMAT profile %q, want %q", m.Profile, manifestShape))
	}
	if m.Module == "" {
		problems = append(problems, "MANIFEST_FORMAT module is empty")
	}
	names := map[string]bool{}
	owners := map[string]string{}
	for _, a := range m.Areas {
		if a.Name == "" || names[a.Name] {
			problems = append(problems, fmt.Sprintf("MANIFEST_FORMAT area name %q is empty or duplicated", a.Name))
		}
		names[a.Name] = true
		if a.Layer < 0 {
			problems = append(problems, fmt.Sprintf("MANIFEST_FORMAT area %s has negative layer %d", a.Name, a.Layer))
		}
		for _, p := range a.Prefixes {
			if p == "" || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.Contains(p, "//") {
				problems = append(problems, fmt.Sprintf("MANIFEST_FORMAT area %s prefix %q is not a slash-separated package path", a.Name, p))
			}
			if prev, ok := owners[p]; ok {
				problems = append(problems, fmt.Sprintf("MANIFEST_FORMAT prefix %s is claimed by areas %s and %s", p, prev, a.Name))
			}
			owners[p] = a.Name
		}
	}
	return problems
}

// areaOf returns the area whose prefix is the longest path-segment match for
// pkg, and that prefix; ok is false when no prefix matches.
func (m *manifest) areaOf(pkg string) (a *area, prefix string, ok bool) {
	for i := range m.Areas {
		for _, p := range m.Areas[i].Prefixes {
			if (pkg == p || strings.HasPrefix(pkg, p+"/")) && len(p) > len(prefix) {
				a, prefix, ok = &m.Areas[i], p, true
			}
		}
	}
	return a, prefix, ok
}

// wrongWay reports whether an import from area from into area to breaks the
// layering rule (IRT-V0-002): only entry areas, the same area, and strictly
// lower layers are allowed.
func wrongWay(from, to *area) bool {
	return from.Name != to.Name && !from.Entry && to.Layer >= from.Layer
}

func edgeKey(from, to string) string { return from + " -> " + to }

// check compares the module-internal import graph (package path relative to
// the module root -> imported package paths, same form) with the manifest and
// baseline, and returns sorted problems; none means the ratchet holds.
func check(m *manifest, graph map[string][]string, b *baseline) []string {
	problems := m.validate()
	if b.Profile != baselineShape {
		problems = append(problems, fmt.Sprintf("BASELINE_FORMAT profile %q, want %q", b.Profile, baselineShape))
	}
	used := map[string]bool{}
	unmapped := map[string]bool{}
	lookup := func(pkg string) *area {
		a, prefix, ok := m.areaOf(pkg)
		if !ok {
			unmapped[pkg] = true
			return nil
		}
		used[prefix] = true
		return a
	}
	present := map[string]bool{}
	for from, tos := range graph {
		fa := lookup(from)
		for _, to := range tos {
			ta := lookup(to)
			if fa == nil || ta == nil || !wrongWay(fa, ta) {
				continue
			}
			present[edgeKey(from, to)] = true
		}
	}
	for pkg := range unmapped {
		problems = append(problems, fmt.Sprintf("UNMAPPED_PACKAGE %s: add a prefix for it to an area in %s", pkg, manifestPath))
	}
	for _, a := range m.Areas {
		for _, p := range a.Prefixes {
			if !used[p] {
				problems = append(problems, fmt.Sprintf("STALE_AREA_PREFIX %s (area %s) matches no package: delete it from %s", p, a.Name, manifestPath))
			}
		}
	}
	listed := map[string]bool{}
	for i, e := range b.Edges {
		if listed[e] {
			problems = append(problems, fmt.Sprintf("BASELINE_FORMAT duplicate edge %q in %s", e, baselinePath))
		}
		listed[e] = true
		if i > 0 && b.Edges[i-1] > e {
			problems = append(problems, fmt.Sprintf("BASELINE_FORMAT edge %q is out of order in %s: keep edges sorted", e, baselinePath))
		}
		if !present[e] {
			problems = append(problems, fmt.Sprintf("STALE_BASELINE_EDGE %s: this wrong-way import is gone; delete its line from %s (the baseline only shrinks)", e, baselinePath))
		}
	}
	for e := range present {
		if listed[e] {
			continue
		}
		from, to, _ := strings.Cut(e, " -> ")
		fa, _, _ := m.areaOf(from)
		ta, _, _ := m.areaOf(to)
		problems = append(problems, fmt.Sprintf("NEW_WRONG_WAY_EDGE %s: area %s (layer %d) may not import area %s (layer %d); invert the dependency or move the shared code to a lower area (%s). Do not add it to %s",
			e, fa.Name, fa.Layer, ta.Name, ta.Layer, manifestPath, baselinePath))
	}
	sort.Strings(problems)
	return problems
}

// platforms are the release targets whose build constraints select a
// package's files (IRT-V0-003); a file counts when any of them selects it.
var platforms = func() []build.Context {
	var out []build.Context
	for _, p := range [][2]string{{"linux", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		c := build.Default
		c.GOOS, c.GOARCH, c.CgoEnabled = p[0], p[1], false
		out = append(out, c)
	}
	return out
}()

// skipDir reports directories that `go list ./...` never walks: hidden and
// underscore directories, test data, vendor and nested modules. It also skips
// node_modules, which go list would walk, so an installed JavaScript tree is
// never read (IRT-V0-003).
func skipDir(path string, d fs.DirEntry, root string) bool {
	if path == root {
		return false
	}
	name := d.Name()
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "node_modules" || name == "vendor" {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

// scanPackage returns the module-internal imports of the non-test Go files in
// dir that some platform selects, and whether dir holds any such file.
func scanPackage(root, rel, module string) ([]string, bool, error) {
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
	}
	found := false
	set := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		selected := false
		for i := range platforms {
			ok, err := platforms[i].MatchFile(dir, name)
			if err != nil {
				return nil, false, err
			}
			if ok {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		found = true
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			return nil, false, err
		}
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if target, ok := strings.CutPrefix(path, module+"/"); ok && target != rel {
				set[target] = true
			}
		}
	}
	imports := make([]string, 0, len(set))
	for p := range set {
		imports = append(imports, p)
	}
	sort.Strings(imports)
	return imports, found, nil
}

// scanModule builds the production import graph of the module at root: every
// package `go list ./...` walks, plus every module package one of them
// imports (such as a package under a hidden directory), as `-deps` adds.
func scanModule(root, module string) (map[string][]string, error) {
	graph := map[string][]string{}
	var queue []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if skipDir(path, d, root) {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." {
			queue = append(queue, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for len(queue) > 0 {
		rel := queue[0]
		queue = queue[1:]
		if seen[rel] {
			continue
		}
		seen[rel] = true
		imports, found, err := scanPackage(root, rel, module)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		graph[rel] = imports
		queue = append(queue, imports...)
	}
	return graph, nil
}

func loadEmbedded(t *testing.T) (*manifest, *baseline) {
	t.Helper()
	var m manifest
	if err := decodeStrict(areasJSON, &m); err != nil {
		t.Fatalf("%s: %v", manifestPath, err)
	}
	var b baseline
	if err := decodeStrict(baselineJSON, &b); err != nil {
		t.Fatalf("%s: %v", baselinePath, err)
	}
	return &m, &b
}

// TestImportRatchet holds IRT-V0-001, IRT-V0-002 and IRT-V0-004 on this
// checkout: every package maps to an area, no wrong-way cross-area import
// exists outside the baseline, and every baseline entry still exists. It runs
// inside go test ./... (IRT-V0-006) and proves its own controls (IRT-V0-005).
func TestImportRatchet(t *testing.T) {
	m, b := loadEmbedded(t)
	graph, err := scanModule(filepath.Join("..", ".."), m.Module)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph) < 100 {
		t.Fatalf("SCAN_INEFFECTIVE: only %d packages found; the module root walk is broken", len(graph))
	}
	t.Run("IRT-V0-006 the checkout graph holds the baseline under go test", func(t *testing.T) {
		for _, p := range check(m, graph, b) {
			t.Error(p)
		}
		t.Logf("%d packages, %d baselined wrong-way edges", len(graph), len(b.Edges))
	})
	t.Run("IRT-V0-005 controls name an added and a removed real edge", func(t *testing.T) {
		if len(b.Edges) == 0 {
			t.Skip("no baselined edge to remove")
		}
		var low, high string
		for pkg := range graph {
			a, _, _ := m.areaOf(pkg)
			switch {
			case a == nil:
			case a.Layer == 0 && !a.Entry && (low == "" || pkg < low):
				low = pkg
			case a.Name == "cli" && (high == "" || pkg < high):
				high = pkg
			}
		}
		if low == "" || high == "" {
			t.Fatalf("no layer-0 or cli package to build the control edge from (low=%q high=%q)", low, high)
		}
		added := copyGraph(graph)
		added[low] = append(added[low], high)
		want := "NEW_WRONG_WAY_EDGE " + edgeKey(low, high) + ":"
		if got := check(m, added, b); len(got) != 1 || !strings.HasPrefix(got[0], want) {
			t.Errorf("SPY_INEFFECTIVE added edge: got %q, want one problem starting %q", got, want)
		}
		removed := copyGraph(graph)
		from, to, _ := strings.Cut(b.Edges[0], " -> ")
		removed[from] = without(removed[from], to)
		want = "STALE_BASELINE_EDGE " + b.Edges[0] + ":"
		if got := check(m, removed, b); len(got) != 1 || !strings.HasPrefix(got[0], want) {
			t.Errorf("SPY_INEFFECTIVE removed edge: got %q, want one problem starting %q", got, want)
		}
	})
}

func copyGraph(g map[string][]string) map[string][]string {
	out := make(map[string][]string, len(g))
	for k, v := range g {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func without(list []string, drop string) []string {
	var out []string
	for _, s := range list {
		if s != drop {
			out = append(out, s)
		}
	}
	return out
}
