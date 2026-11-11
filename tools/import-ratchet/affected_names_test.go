//corvint:read-kinds .go go.mod

package importratchet

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// affectedPackage is the package IRT-V0-021 guards. It is compared against the scanned graph and
// names no read of its own, so it carries the fixture marker; the reads are scanModule's and the
// importer parses below, bounded by the read-kinds directive above.
const affectedPackage = "internal/liveverify/affected" //corvint:fixture-path

// leafOnlyNames are the affected names that justify no cross-layer import on their own
// (IRT-V0-021): ValidRelativePath forwards to the shared leaf internal/repopath (Valid), and
// Language is the plugin seam, whose signature carries Result and Unit.
var leafOnlyNames = []string{"Language", "ValidRelativePath"}

// affectedSelectors returns the exported names the platform-selected production files of the
// package at rel select from the import of target, sorted and unique, and whether some file
// dot-imports it (which hides the names, so it cannot be told apart from a leaf-only use).
func affectedSelectors(root, rel, module, target string) (names []string, dot bool, err error) {
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
	}
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
			selected = selected || ok
		}
		if !selected {
			continue
		}
		// Object resolution leaves a package reference unresolved (Obj nil), so a local
		// variable, parameter or receiver named like the import is not taken for it.
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, false, err
		}
		local := ""
		for _, spec := range file.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path != module+"/"+target {
				continue
			}
			local = filepath.Base(target)
			if spec.Name != nil {
				local = spec.Name.Name
			}
		}
		switch local {
		case "", "_":
			continue
		case ".":
			dot = true
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == local && x.Obj == nil {
					set[sel.Sel.Name] = true
				}
			}
			return true
		})
	}
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, dot, nil
}

// leafOnlyAffectedImports returns one LEAF_ONLY_AFFECTED_IMPORT problem, sorted, for each package
// outside the tests and release layers that imports affectedPackage only for leafOnlyNames, and
// the number of importers it inspected (IRT-V0-021).
func leafOnlyAffectedImports(root string, m *manifest, graph map[string][]string) ([]string, int, error) {
	var problems []string
	inspected := 0
	for from, imports := range graph {
		a, _, ok := m.areaOf(from)
		if !ok || a.Name == "tests" || a.Name == "release" || !slices.Contains(imports, affectedPackage) {
			continue
		}
		inspected++
		names, dot, err := affectedSelectors(root, from, m.Module, affectedPackage)
		if err != nil {
			return nil, 0, err
		}
		leafOnly := true
		for _, n := range names {
			leafOnly = leafOnly && slices.Contains(leafOnlyNames, n)
		}
		if dot || leafOnly {
			problems = append(problems, fmt.Sprintf("LEAF_ONLY_AFFECTED_IMPORT %s (area %s) imports %s only for %v (dot import %v): use internal/repopath.Valid for ValidRelativePath, and do not import the language seam from another layer (IRT-V0-021)",
				from, a.Name, affectedPackage, names, dot))
		}
	}
	sort.Strings(problems)
	return problems, inspected, nil
}

// TestIRTV0021NoLeafOnlyAffectedImport holds IRT-V0-021 on this checkout and proves its control on
// a synthetic module: an importer that selects only Language fails, one that also selects Plan
// passes, a dot import fails, and a Plan selected from a parameter named affected is not counted.
func TestIRTV0021NoLeafOnlyAffectedImport(t *testing.T) {
	m, _ := loadEmbedded(t)
	t.Run("IRT-V0-021 no checkout importer outside tests and release uses only leaf names", func(t *testing.T) {
		root := filepath.Join("..", "..")
		graph, err := scanModule(root, m.Module)
		if err != nil {
			t.Fatal(err)
		}
		problems, inspected, err := leafOnlyAffectedImports(root, m, graph)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range problems {
			t.Error(p)
		}
		t.Logf("%d importers of %s outside the tests and release layers inspected", inspected, affectedPackage)
	})
	t.Run("IRT-V0-021 control names a leaf-only and a dot importer", func(t *testing.T) {
		root := t.TempDir()
		const module = "example.com/m"
		files := map[string]string{
			"go.mod":                            "module " + module + "\n",
			affectedPackage + "/a.go":           "package affected\n\ntype Language interface{}\n\ntype Plan struct{}\n\nfunc ValidRelativePath(string) bool { return true }\n",
			"internal/seamonly/s.go":            "package seamonly\n\nimport \"" + module + "/" + affectedPackage + "\"\n\nvar _ affected.Language\n\nvar _ = affected.ValidRelativePath\n",
			"internal/shadowed/s.go":            "package shadowed\n\nimport \"" + module + "/" + affectedPackage + "\"\n\ntype plan struct{ Plan int }\n\nvar _ affected.Language\n\nfunc use(affected plan) int { return affected.Plan }\n",
			"internal/planner/p.go":             "package planner\n\nimport aff \"" + module + "/" + affectedPackage + "\"\n\nvar _ aff.Language\n\nvar _ aff.Plan\n",
			"internal/dotted/d.go":              "package dotted\n\nimport . \"" + module + "/" + affectedPackage + "\"\n\nvar _ Plan\n",
			"internal/seamonly/s_test.go":       "package seamonly\n\nimport \"" + module + "/" + affectedPackage + "\"\n\nvar _ affected.Plan\n",
			"internal/liveverify/consumer/c.go": "package consumer\n\nimport \"" + module + "/" + affectedPackage + "\"\n\nvar _ affected.Language\n",
			"tools/anything/t.go":               "package anything\n\nimport \"" + module + "/" + affectedPackage + "\"\n\nvar _ affected.Language\n",
		}
		for name, body := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		synthetic := &manifest{Module: module, Areas: []area{
			{Name: "shared", Layer: 0, Prefixes: []string{"internal"}},
			{Name: "tests", Layer: 1, Prefixes: []string{"internal/liveverify"}},
			{Name: "release", Layer: 2, Prefixes: []string{"tools"}},
		}}
		graph, err := scanModule(root, module)
		if err != nil {
			t.Fatal(err)
		}
		problems, inspected, err := leafOnlyAffectedImports(root, synthetic, graph)
		if err != nil {
			t.Fatal(err)
		}
		if inspected != 4 {
			t.Errorf("SPY_INEFFECTIVE inspected %d importers, want 4 (seamonly, shadowed, planner, dotted)", inspected)
		}
		var named []string
		for _, p := range problems {
			named = append(named, strings.Fields(p)[1])
		}
		// internal/shadowed selects Plan only from a parameter named affected, so it is leaf-only.
		if want := []string{"internal/dotted", "internal/seamonly", "internal/shadowed"}; !slices.Equal(named, want) {
			t.Errorf("SPY_INEFFECTIVE flagged %q, want %q; problems %q", named, want, problems)
		}
	})
}
