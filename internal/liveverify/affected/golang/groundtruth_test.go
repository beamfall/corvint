package golang_test

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const modulePath = "github.com/Beamfall/corvint"

type listPackage struct {
	ImportPath   string   `json:"ImportPath"`
	Imports      []string `json:"Imports"`
	TestImports  []string `json:"TestImports"`
	XTestImports []string `json:"XTestImports"`
}

// TestSourceParsedEdgesCoverEveryEdgeTheToolchainReports is the correctness
// gate for the Go plugin.
//
// The plugin reads import edges from source text so it can work on a dirty
// worktree with no build cache. That is only sound if it never MISSES an edge
// the toolchain sees: a missed edge under-selects and would let a broken test
// go unrun. An extra edge is the safe direction — the plugin ignores build
// constraints, so it legitimately reports edges that this platform's build
// does not take, and reports that fact as a frontier.
func TestSourceParsedEdgesCoverEveryEdgeTheToolchainReports(t *testing.T) {
	root := repositoryRoot(t)
	truth := listFirstPartyPackages(t, root)
	if len(truth) < 50 {
		t.Fatalf("ground truth too small: %d packages", len(truth))
	}
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	observed := make(map[string]map[string]bool, len(result.Units))
	for _, unit := range result.Units {
		edges := make(map[string]bool, len(unit.Imports))
		for _, target := range unit.Imports {
			edges[strings.TrimPrefix(target, "go:")] = true
		}
		observed[strings.TrimPrefix(unit.ID, "go:")] = edges
	}
	missingPackages := 0
	missingEdges := 0
	for _, pack := range truth {
		edges, known := observed[pack.ImportPath]
		if !known {
			missingPackages++
			t.Errorf("package %s is absent from the source-parsed graph", pack.ImportPath)
			continue
		}
		for _, group := range [][]string{pack.Imports, pack.TestImports, pack.XTestImports} {
			for _, target := range group {
				if !firstParty(target) || target == pack.ImportPath {
					continue
				}
				// A package's external test variant imports the package under
				// test; that is the same unit here, not an edge.
				if strings.TrimSuffix(target, "_test") == pack.ImportPath {
					continue
				}
				if !edges[target] {
					missingEdges++
					t.Errorf("edge %s -> %s reported by the toolchain is missing", pack.ImportPath, target)
				}
			}
		}
	}
	t.Logf("ground truth: %d first-party packages, %d missing from graph, %d missing edges",
		len(truth), missingPackages, missingEdges)
}

func listFirstPartyPackages(t *testing.T, root string) []listPackage {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain is unavailable")
	}
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = root
	stdout := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = &bytes.Buffer{}
	if err := command.Run(); err != nil {
		t.Skipf("go list failed: %v", err)
	}
	decoder := jsontext.NewDecoder(stdout)
	packages := make([]listPackage, 0, 128)
	for {
		var pack listPackage
		if err := json.UnmarshalDecode(decoder, &pack); err != nil {
			break
		}
		if firstParty(pack.ImportPath) {
			packages = append(packages, pack)
		}
	}
	return packages
}

func firstParty(importPath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := affected.SourceFiles(root, func(name string) bool { return name == "go.mod" }); err != nil {
		t.Fatal(err)
	}
	return root
}
