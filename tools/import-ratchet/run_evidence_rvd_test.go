//corvint:read-kinds .go go.mod

// SPDX-License-Identifier: Apache-2.0

package importratchet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The run evidence store and its readers (RVD-V1-007, RVD-V1-011). Each literal is compared
// against the scanned graph and names no read of its own, so it carries the fixture marker.
const (
	rvdStore        = "internal/runevidence"     //corvint:fixture-path
	rvdEvidenceDoc  = "internal/testevidencedoc" //corvint:fixture-path
	rvdAppflows     = "internal/appflows"        //corvint:fixture-path
	rvdCorvint      = "cmd/corvint"              //corvint:fixture-path
	rvdStoreReaders = "runstore.go"              //corvint:fixture-path
)

// rvdSelectionPrefixes are the packages that choose what runs: corvint affected, the corvint test
// planner, the gate and runner selection and the plan snapshot.
var rvdSelectionPrefixes = []string{
	"internal/liveverify/affected", //corvint:fixture-path
	"internal/testplan",            //corvint:fixture-path
	"internal/testsource",          //corvint:fixture-path
	"internal/gateselect",          //corvint:fixture-path
	"internal/runnerselection",     //corvint:fixture-path
	"internal/plansnapshot",        //corvint:fixture-path
	"internal/sessiontest",         //corvint:fixture-path
}

// rvdAppflowsReaders are the appflows functions that read the store.
var rvdAppflowsReaders = []string{"ReadStoredRunEvidence", "RunEvidenceFor", "WithStoredRunEvidence"}

// rvdCorvintAllowed is every cmd/corvint function that may touch the store or a store reader: the
// one writer of corvint test and corvint watch (RVD-V1-004) and the test-evidence and context
// readers (RVD-V1-007).
var rvdCorvintAllowed = []string{
	"context_path.go pathContextTests testevidencedoc",
	"flows_test_evidence.go flowsTestEvidence testevidencedoc",
	"test.go appendRunEvidence runevidence",
	"test.go writeFailed runevidence",
	"test_evidence.go runTestEvidence testevidencedoc",
}

// TestRVDV1011NoSelectionPackageReadsTheStore pins RVD-V1-011 in the import graph: no package
// that selects tests imports the store or the test-evidence reader, or calls an appflows store
// reader. A spy planner shows the scan sees such a call.
func TestRVDV1011NoSelectionPackageReadsTheStore(t *testing.T) {
	m, _ := loadEmbedded(t)
	root := filepath.Join("..", "..")
	graph, err := scanModule(root, m.Module)
	if err != nil {
		t.Fatal(err)
	}
	selection := 0
	for _, pkg := range slices.Sorted(maps.Keys(graph)) {
		if !rvdSelection(pkg) {
			continue
		}
		selection++
		for _, banned := range []string{rvdStore, rvdEvidenceDoc} {
			if slices.Contains(graph[pkg], banned) {
				t.Errorf("RUN_EVIDENCE_SELECTION_IMPORT %s imports %s (RVD-V1-011)", pkg, banned)
			}
		}
		refs, err := rvdStoreReferences(root, pkg, m.Module)
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range refs {
			t.Errorf("RUN_EVIDENCE_SELECTION_READ %s: %s (RVD-V1-011)", pkg, ref)
		}
	}
	if selection < len(rvdSelectionPrefixes) {
		t.Fatalf("only %d selection packages found; a prefix no longer names a package", selection)
	}

	spy := t.TempDir()
	planner := filepath.Join(spy, "internal", "testplan")
	if err := os.MkdirAll(planner, 0o700); err != nil {
		t.Fatal(err)
	}
	source := "package testplan\n\nimport flows \"" + m.Module + "/" + rvdAppflows + "\"\n\nfunc Plan() { _, _, _ = flows.RunEvidenceFor(nil, \"\", nil, false) }\n"
	if err := os.WriteFile(filepath.Join(planner, "plan.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := rvdStoreReferences(spy, "internal/testplan", m.Module); err != nil || !slices.Equal(got, []string{"plan.go Plan appflows.RunEvidenceFor"}) {
		t.Errorf("SPY_INEFFECTIVE a planner calling the store reader: got %q %v", got, err)
	}
}

// TestRVDV1011StoredResultsDoNotChangePlan pins RVD-V1-011 where the store is reachable: inside
// appflows only runstore.go reaches it, and in cmd/corvint, which holds the corvint test and
// corvint affected planning, only the one writer and the named test-evidence readers reference the
// store or a store reader. A stored result therefore cannot reach a plan.
func TestRVDV1011StoredResultsDoNotChangePlan(t *testing.T) {
	m, _ := loadEmbedded(t)
	root := filepath.Join("..", "..")
	refs, err := rvdStoreReferences(root, rvdAppflows, m.Module)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) == 0 {
		t.Fatalf("no appflows function reaches the store; the scan no longer sees %s", rvdStoreReaders)
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, rvdStoreReaders+" ") {
			t.Errorf("RUN_EVIDENCE_APPFLOWS_READ %s: only %s reaches the store", ref, rvdStoreReaders)
		}
	}
	refs, err = rvdStoreReferences(root, rvdCorvint, m.Module)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, rvdCorvintAllowed) {
		t.Errorf("RUN_EVIDENCE_CORVINT_READ cmd/corvint store references\n got %q\nwant %q", refs, rvdCorvintAllowed)
	}
}

func rvdSelection(pkg string) bool {
	for _, prefix := range rvdSelectionPrefixes {
		if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") {
			return true
		}
	}
	return false
}

// rvdStoreReferences lists, sorted, each "file function reference" in the non-test Go files of
// pkg that names the store package, the test-evidence reader, or an appflows store reader (by its
// package or, inside appflows, by its bare name).
func rvdStoreReferences(root, pkg, module string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(pkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		local := map[string]string{}
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			target, ok := strings.CutPrefix(path, module+"/")
			if !ok || (target != rvdStore && target != rvdEvidenceDoc && target != rvdAppflows) {
				continue
			}
			alias := filepath.Base(target)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			local[alias] = filepath.Base(target)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.SelectorExpr:
					ident, ok := n.X.(*ast.Ident)
					if !ok {
						return true
					}
					switch local[ident.Name] {
					case "runevidence", "testevidencedoc":
						found[name+" "+fn.Name.Name+" "+local[ident.Name]] = true
					case "appflows":
						if slices.Contains(rvdAppflowsReaders, n.Sel.Name) {
							found[name+" "+fn.Name.Name+" appflows."+n.Sel.Name] = true
						}
					}
					return false
				case *ast.Ident:
					if pkg == rvdAppflows && slices.Contains(rvdAppflowsReaders, n.Name) {
						found[name+" "+fn.Name.Name+" "+n.Name] = true
					}
				}
				return true
			})
		}
	}
	refs := slices.Collect(maps.Keys(found))
	sort.Strings(refs)
	return refs, nil
}
