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
		baseFiles := map[string][]byte{}
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
			baseFiles[file] = before
			declarations = append(declarations, typedreach.DiffFile(file, before, head).Declarations(typedreach.ImportPath(module, file))...)
		}
		if unreadable != "" {
			whole(unreadable)
			continue
		}
		if len(declarations) == 0 {
			// The plan reached the package through a change no declaration
			// shows: never slice on an empty change set.
			whole("no changed Go declaration was read")
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
		before, err := typedreach.LoadBase(root, module, relative, baseFiles)
		if err != nil {
			whole(typedreach.WholeBaseSource + ": " + err.Error())
			continue
		}
		if why := typedreach.BaseEffects(before, declarations); why != "" {
			whole(why)
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
// binary can link: non-test Go source files of Go units (AFP-V0-2377).
// Another package's test file is never linked and is left out. Any other
// dirty path in or under the directory of a Go package (embedded, or read at
// run time relative to the package's source), and any path under a package
// that embeds files, is the reason no slice is proved; a path outside every
// Go package directory can reach a test only by a read the per-package
// refusals check (typedReachUnitRefusal).
func typedReachChangedPaths(graph *affected.Graph, dirty []string) ([]string, string) {
	directories, embedding := map[string]bool{}, map[string]bool{}
	for _, id := range graph.UnitIDs() {
		unit, ok := graph.Unit(id)
		files := append(slices.Clone(unit.Sources), unit.Tests...)
		if !ok || !strings.HasPrefix(id, "go:") || len(files) == 0 {
			continue
		}
		directory := path.Dir(files[0])
		directories[directory] = true
		embedding[directory] = embedding[directory] || unit.Embeds || len(unit.EmbedFiles) > 0
	}
	under := func(file string, set map[string]bool) string {
		for directory := path.Dir(file); ; directory = path.Dir(directory) {
			if set[directory] {
				return directory
			}
			if directory == "." || directory == "/" {
				return ""
			}
		}
	}
	var changed []string
	for _, file := range dirty {
		if directory := under(file, embedding); directory != "" {
			return nil, file + " may be embedded by the package in " + directory
		}
		owner, owned := graph.OwnerOf(file)
		switch {
		case owned && strings.HasPrefix(owner, "go:") && strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") && !strings.Contains("/"+file, "/testdata/"):
			changed = append(changed, file)
		case owned && strings.HasPrefix(owner, "go:") && strings.HasSuffix(file, "_test.go"):
		default:
			if directory := under(file, directories); directory != "" {
				return nil, file + " is a data file under the Go package directory " + directory
			}
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
