package main

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang/typedreach"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// typedReachPackages is the AFP-V0-2370 allow-list: the Go packages whose
// DEPENDENCY_PATH selection `corvint test` narrows to the test functions a
// type-checked call graph proves can reach the change. Every other package
// keeps its whole-package selection.
var typedReachPackages = map[string]bool{
	"cmd/corvint":          true,
	"internal/tasks/cli":   true,
	"internal/tasks/store": true,
}

// typedReachLoad and typedReachAnalyze are held in variables so a test can
// count or replace them.
var (
	typedReachLoad    = typedreach.Load
	typedReachAnalyze = typedreach.Analyze
)

// testTypedSlices replaces, for each allow-listed package the plan selects
// by DEPENDENCY_PATH, the whole-package reason in sliced with a typed-reach
// slice, or with that reason extended by why no slice is proved
// (AFP-V0-2370, AFP-V0-2377). The base reads of the changed files are one
// batch for all packages.
func testTypedSlices(ctx context.Context, plan affected.Plan, graph *affected.Graph, root, base string, sliced map[string]sessiontest.GoSlice) {
	module, err := golang.ReadModulePath(root)
	if err != nil {
		return
	}
	var candidates []affected.Selection
	for _, selection := range plan.Selected {
		pkg, ok := strings.CutPrefix(selection.UnitID, "go:")
		if !ok || selection.Witness.Kind != affected.WitnessDependency {
			continue
		}
		if relative := strings.TrimPrefix(strings.TrimPrefix(pkg, module), "/"); typedReachPackages[relative] {
			candidates = append(candidates, selection)
		}
	}
	if len(candidates) == 0 || module == "" {
		return
	}
	changed, refusal := typedReachChangedPaths(graph, plan.Dirty)
	var blobs map[string]testBaseBlob
	if refusal == "" {
		blobs = testBaseBlobs(ctx, root, base, changed)
	}
	for _, selection := range candidates {
		pkg := strings.TrimPrefix(selection.UnitID, "go:")
		whole := func(why string) {
			sliced[pkg] = sessiontest.GoSlice{Reason: "whole: " + selection.Witness.Kind + " via " + selection.Witness.DirtyPath + wholeReader(selection.Witness) + " (typed reach: " + why + ")"}
		}
		if refusal != "" {
			whole(refusal)
			continue
		}
		unit, _ := graph.Unit(selection.UnitID)
		if why := typedReachUnitRefusal(graph, unit, plan.Dirty); why != "" {
			whole(why)
			continue
		}
		var declarations []typedreach.Declaration
		unreadable := ""
		for _, file := range changed {
			blob := blobs[file]
			if blob.reason != "" {
				unreadable = "the base of " + file + " is unreadable: " + blob.reason
				break
			}
			head, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				head = nil
			}
			var before []byte
			if !blob.absent {
				before = blob.content
			}
			declarations = append(declarations, typedreach.DiffFile(file, before, head).Declarations(module+"/"+path.Dir(file))...)
		}
		if unreadable != "" {
			whole(unreadable)
			continue
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(pkg, module), "/")
		program, err := typedReachLoad(root, module, relative)
		if err != nil {
			whole(err.Error())
			continue
		}
		result := typedReachAnalyze(program, declarations, nil)
		if result.Whole != "" {
			whole(result.Whole)
			continue
		}
		kept := make([]string, 0, len(result.Kept))
		for test := range result.Kept {
			kept = append(kept, test)
		}
		slices.Sort(kept)
		changedIn := strings.Join(changed, ", ")
		if len(kept) == 0 {
			sliced[pkg] = sessiontest.GoSlice{Pattern: "^$", Reason: "selected: no test reaches " + changedIn + "; build and vet only"}
			continue
		}
		pattern := "^(" + strings.Join(kept, "|") + ")$"
		if len(pattern) > cishards.MaxPatternBytes {
			whole("the -run pattern of " + strconv.Itoa(len(kept)) + " kept tests exceeds the bound")
			continue
		}
		sliced[pkg] = sessiontest.GoSlice{Pattern: pattern, Reason: "selected: the tests reaching " + changedIn + " (typed reach: " + strconv.Itoa(len(result.Dropped)) + " of " + strconv.Itoa(result.Tests) + " test functions cannot reach it)"}
	}
}

// typedReachChangedPaths returns the dirty paths that change code a test
// binary can link: non-test Go source files of Go units. Another package's
// test files, test data and every other non-Go path are never compiled into
// a binary unless a package embeds files, so they are left out; a reader of
// them by path is refused per package (typedReachUnitRefusal). A non-Go path
// of a package that embeds files is the reason no slice is proved.
func typedReachChangedPaths(graph *affected.Graph, dirty []string) ([]string, string) {
	var changed []string
	for _, file := range dirty {
		owner, owned := graph.OwnerOf(file)
		unit, _ := graph.Unit(owner)
		goUnit := owned && strings.HasPrefix(owner, "go:")
		switch {
		case goUnit && strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") && !strings.Contains("/"+file, "/testdata/"):
			changed = append(changed, file)
		case goUnit && unit.Embeds && !strings.HasSuffix(file, ".go"):
			return nil, file + " may be embedded by " + owner
		}
	}
	return changed, ""
}

// typedReachUnitRefusal names what lets the package's tests see the change
// other than by running linked code: a read of a dirty path by literal,
// glob, declared scope or embedding, an unbounded or root-locating read, or
// a read of the import graph (AFP-V0-2377).
func typedReachUnitRefusal(graph *affected.Graph, unit affected.Unit, dirty []string) string {
	switch {
	case slices.Contains(graph.UnboundedReaders(), unit.ID):
		return "the package's tests read the repository unboundedly"
	case unit.Embeds:
		return "the package embeds files"
	case unit.ImportGraphReadsAll || len(unit.ImportGraphReads) > 0:
		return "the package's tests read the import graph"
	}
	directory := ""
	if files := append(slices.Clone(unit.Sources), unit.Tests...); len(files) > 0 {
		directory = path.Dir(files[0])
	}
	if slices.Contains(graph.ExistenceReaderPackages(), directory) {
		return "the package's tests read path existence"
	}
	if file, glob, ok := graph.GlobRead(unit.ID, dirty); ok {
		return "the package's glob " + glob + " matches " + file
	}
	for _, file := range dirty {
		if graph.NamesPath(unit.ID, file) {
			return "the package names " + file + " as a path"
		}
		if unit.ReadScoped && affected.InReadScope(directory, unit.ReadScope, file) {
			return "the package's declared read scope holds " + file
		}
		if owner, _ := graph.OwnerOf(file); owner == unit.ID {
			return file + " is the package's own source"
		}
	}
	return ""
}
