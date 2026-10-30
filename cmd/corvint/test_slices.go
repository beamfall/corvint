package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// testGoSlices decides, per selected Go package, whether `corvint test` may
// run only named test functions (STR-V0-063). A package is sliced only when
// every proof holds; any missing proof keeps it whole with the reason named:
//
//   - the plan selects it by DIRECT_TEST_CHANGE alone, and the same graph
//     planned without the package's own changed test files no longer selects
//     it (so no source change, importer, path reader or declared read reaches
//     it); one walk over the whole dirty set answers this for every package
//     at once (STR-V0-072);
//   - every test file of the package exists and parses, none of the changed
//     ones declares init or TestMain, initializes a package variable with a
//     call, or has a blank or dot import, and no unchanged test file mentions
//     a name a changed one declares, test functions included (a shared
//     helper, type or variable, or a wrapper test calling a changed one,
//     could carry the edit into a test the pattern would skip: STR-V0-070),
//     and none of them had such an initialization effect at base either (a
//     removed initialization effect changes what every test observes);
//   - the import paths of the package's test files, taken together, are the
//     same as at base (an added or removed import changes which packages the
//     test binary initializes: STR-V0-071);
//   - the changed files declare at least one Test, Example or Fuzz function,
//     and the anchored pattern of all of them fits the -run bound.
//
// The slice runs every test function of every changed test file, so a test
// moved, added or edited there runs; which assertion changed is not modelled.
// The base reads of every changed test file of every package are two Git
// processes in all (STR-V0-072).
func testGoSlices(ctx context.Context, plan affected.Plan, graph *affected.Graph, root, base string) map[string]sessiontest.GoSlice {
	sliced := make(map[string]sessiontest.GoSlice, len(plan.Selected))
	var candidates []testSliceCandidate
	for _, selection := range plan.Selected {
		pkg, ok := strings.CutPrefix(selection.UnitID, "go:")
		if !ok {
			continue
		}
		candidate, reason := testSliceCandidateOf(plan, graph, selection)
		if reason != "" {
			sliced[pkg] = sessiontest.GoSlice{Reason: reason}
			continue
		}
		candidates = append(candidates, candidate)
	}
	elsewhere := testSlicesReachedElsewhere(plan, graph, candidates)
	var paths []string
	for _, candidate := range candidates {
		if elsewhere[candidate.id] == "" {
			for _, path := range candidate.unit.Tests {
				if candidate.changed[path] {
					paths = append(paths, path)
				}
			}
		}
	}
	atBase := testBaseDeclarations(ctx, root, base, paths)
	for _, candidate := range candidates {
		pkg := strings.TrimPrefix(candidate.id, "go:")
		if reason := elsewhere[candidate.id]; reason != "" {
			sliced[pkg] = sessiontest.GoSlice{Reason: reason}
			continue
		}
		sliced[pkg] = testGoSlice(root, candidate, atBase)
	}
	return sliced
}

// testSliceSelect and testSliceReach are the plan walks test slicing makes,
// held in variables so a test can count them (STR-V0-072).
var (
	testSliceSelect = affected.Select
	testSliceReach  = (*affected.Graph).ReachedBeyondOwnTests
)

// testSliceCandidate is a Go package the plan selects by DIRECT_TEST_CHANGE
// with at least one of its own test files dirty: changed holds those files,
// and rest whether any other dirty path exists.
type testSliceCandidate struct {
	id      string
	unit    affected.Unit
	changed map[string]bool
	rest    bool
}

func testSliceCandidateOf(plan affected.Plan, graph *affected.Graph, selection affected.Selection) (testSliceCandidate, string) {
	witness := selection.Witness
	if witness.Kind != affected.WitnessDirectTest {
		return testSliceCandidate{}, "whole: " + witness.Kind + " via " + witness.DirtyPath
	}
	unit, ok := graph.Unit(selection.UnitID)
	if !ok {
		return testSliceCandidate{}, "whole: the plan's unit is not in the graph"
	}
	candidate := testSliceCandidate{id: selection.UnitID, unit: unit, changed: make(map[string]bool)}
	for _, path := range plan.Dirty {
		if testFileOf(unit, path) {
			candidate.changed[path] = true
		} else {
			candidate.rest = true
		}
	}
	if len(candidate.changed) == 0 {
		return testSliceCandidate{}, "whole: no dirty path is one of the unit's test files"
	}
	return candidate, ""
}

// testSlicesReachedElsewhere names, per candidate, the reason the plan would
// select it without its own changed test files (STR-V0-072). One walk over the
// whole dirty set, leaving out only the test-only units' own selection, finds
// every candidate something else reaches; the dirty set holds each
// candidate's other paths, so a candidate it misses is missed on them too. A
// witness through another path keeps the package whole with that witness. A
// witness through the candidate's own test file is ambiguous: one shared plan
// of the paths that are no candidate's test file decides it, which for a
// single candidate is exactly the plan of its other paths; with several
// candidates a package that plan does not select stays whole (it might be
// reached through another candidate's test file).
func testSlicesReachedElsewhere(plan affected.Plan, graph *affected.Graph, candidates []testSliceCandidate) map[string]string {
	reasons := make(map[string]string)
	tests := make(map[string]bool)
	rest := false
	for _, candidate := range candidates {
		for path := range candidate.changed {
			tests[path] = true
		}
		rest = rest || candidate.rest
	}
	if !rest {
		return reasons
	}
	reached := testSliceReach(graph, plan.Dirty)
	var outside []string
	for _, path := range plan.Dirty {
		if !tests[path] {
			outside = append(outside, path)
		}
	}
	var shared map[string]affected.Witness
	for _, candidate := range candidates {
		witness, hit := reached[candidate.id]
		if !candidate.rest || !hit {
			continue
		}
		if !candidate.changed[witness.DirtyPath] {
			reasons[candidate.id] = "whole: also selected by " + witness.Kind + " via " + witness.DirtyPath
			continue
		}
		if shared == nil {
			shared = make(map[string]affected.Witness)
			if len(outside) != 0 {
				for _, other := range testSliceSelect(graph, outside).Selected {
					shared[other.UnitID] = other.Witness
				}
			}
		}
		if other, hit := shared[candidate.id]; hit {
			reasons[candidate.id] = "whole: also selected by " + other.Kind + " via " + other.DirtyPath
		} else if len(candidates) > 1 {
			reasons[candidate.id] = "whole: " + witness.Kind + " via its own " + witness.DirtyPath + " is not separable from the other packages' changed test files"
		}
	}
	return reasons
}

func testGoSlice(root string, candidate testSliceCandidate, atBase map[string]testBaseFile) sessiontest.GoSlice {
	type parsed struct {
		path string
		golang.TestDeclarations
	}
	var changedFiles, unchangedFiles []parsed
	for _, path := range candidate.unit.Tests {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return sessiontest.GoSlice{Reason: "whole: " + path + " cannot be read"}
		}
		declarations, err := golang.TestFileDeclarations(path, source)
		if err != nil {
			return sessiontest.GoSlice{Reason: "whole: " + path + " does not parse"}
		}
		if candidate.changed[path] {
			changedFiles = append(changedFiles, parsed{path, declarations})
		} else {
			unchangedFiles = append(unchangedFiles, parsed{path, declarations})
		}
	}
	if len(changedFiles) != len(candidate.changed) {
		return sessiontest.GoSlice{Reason: "whole: a changed test file is not a test file of the unit"}
	}
	var names []string
	declared := make(map[string]string)
	importedAtBase := make(map[string]string)
	for _, file := range changedFiles {
		if file.SideEffect != "" {
			return sessiontest.GoSlice{Reason: "whole: " + file.path + " " + file.SideEffect}
		}
		prior, read := atBase[file.path]
		if !read {
			return sessiontest.GoSlice{Reason: "whole: " + file.path + " cannot be read at base"}
		}
		if prior.reason != "" {
			return sessiontest.GoSlice{Reason: "whole: " + file.path + " " + prior.reason}
		}
		names = append(names, file.Tests...)
		for _, name := range append(append([]string(nil), file.Tests...), file.Others...) {
			declared[name] = file.path
		}
		for _, path := range prior.imports {
			if _, seen := importedAtBase[path]; !seen {
				importedAtBase[path] = file.path
			}
		}
	}
	for _, file := range unchangedFiles {
		for _, name := range file.Uses {
			if owner, shared := declared[name]; shared {
				return sessiontest.GoSlice{Reason: "whole: " + file.path + " uses " + name + " declared in " + owner}
			}
		}
	}
	importedNow, importedBefore := make(map[string]bool), make(map[string]bool)
	for _, file := range unchangedFiles {
		for _, path := range file.Imports {
			importedNow[path], importedBefore[path] = true, true
		}
	}
	for path := range importedAtBase {
		importedBefore[path] = true
	}
	for _, file := range changedFiles {
		for _, path := range file.Imports {
			importedNow[path] = true
			if !importedBefore[path] {
				return sessiontest.GoSlice{Reason: "whole: " + file.path + " imports " + path + ", which no test file of the package imported at base"}
			}
		}
	}
	removed := make([]string, 0, len(importedAtBase))
	for path := range importedAtBase {
		if !importedNow[path] {
			removed = append(removed, path)
		}
	}
	if len(removed) != 0 {
		slices.Sort(removed)
		return sessiontest.GoSlice{Reason: "whole: " + importedAtBase[removed[0]] + " imported " + removed[0] + " at base, which no test file of the package imports now"}
	}
	if len(names) == 0 {
		return sessiontest.GoSlice{Reason: "whole: the changed test files declare no test function"}
	}
	slices.Sort(names)
	pattern := "^(" + strings.Join(slices.Compact(names), "|") + ")$"
	if len(pattern) > cishards.MaxPatternBytes {
		return sessiontest.GoSlice{Reason: "whole: the -run pattern exceeds the bound"}
	}
	paths := make([]string, 0, len(changedFiles))
	for _, file := range changedFiles {
		paths = append(paths, file.path)
	}
	return sessiontest.GoSlice{Pattern: pattern, Reason: "selected: the tests of " + strings.Join(paths, ", ")}
}

func testFileOf(unit affected.Unit, path string) bool {
	for _, test := range unit.Tests {
		if test == path {
			return true
		}
	}
	return false
}

// testBaseFile is one changed test file as it was at base: the reason it
// keeps the package whole (an initialization effect it had, or a failed
// read), else the import paths it named; a file absent at base has neither.
type testBaseFile struct {
	reason  string
	imports []string
}

// testBaseDeclarations reads every given test file as it was at base, in two
// Git processes whatever the number of files or packages (STR-V0-072): one
// `ls-tree -l -z` names each blob and its size, and one `cat-file --batch`
// reads them all. Removing an initialization effect changes what every test
// of the package observes, so it is as much a proof gap as adding one. A
// failed listing, read or parse is itself the reason, so the package stays
// whole on a missing proof; the reasons are those of the former per-file read.
func testBaseDeclarations(ctx context.Context, root, base string, paths []string) map[string]testBaseFile {
	files := make(map[string]testBaseFile, len(paths))
	if len(paths) == 0 {
		return files
	}
	limit := 0
	for _, path := range paths {
		limit += len(path) + 160
	}
	listing, err := sourceGit(ctx, root, limit, append([]string{"ls-tree", "-l", "-z", base, "--"}, paths...)...)
	if err != nil {
		for _, path := range paths {
			files[path] = testBaseFile{reason: "cannot be listed at base"}
		}
		return files
	}
	type blob struct {
		oid  string
		size int
	}
	listed := make(map[string]blob, len(paths))
	for _, entry := range bytes.Split(bytes.TrimSuffix(listing, []byte{0}), []byte{0}) {
		head, path, ok := bytes.Cut(entry, []byte{'\t'})
		if !ok {
			continue
		}
		fields := strings.Fields(string(head))
		if len(fields) != 4 || fields[1] != "blob" {
			listed[string(path)] = blob{}
			continue
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			size = -1
		}
		listed[string(path)] = blob{oid: fields[2], size: size}
	}
	var request bytes.Buffer
	requested := make(map[string]bool)
	limit = 0
	for _, path := range paths {
		entry, present := listed[path]
		switch {
		case !present:
			files[path] = testBaseFile{}
		case entry.oid == "":
			files[path] = testBaseFile{reason: "is not a file at base"}
		case entry.size < 0 || entry.size > sourceBlobLimit:
			files[path] = testBaseFile{reason: "cannot be read at base"}
		case !requested[entry.oid]:
			requested[entry.oid] = true
			request.WriteString(entry.oid + "\n")
			limit += entry.size + len(entry.oid) + 32
		}
	}
	contents := make(map[string][]byte, len(requested))
	if len(requested) != 0 {
		output, err := testSliceGitInput(ctx, root, limit, request.Bytes(), "cat-file", "--batch")
		if err == nil {
			contents = testBatchContents(output)
		}
	}
	for _, path := range paths {
		if _, decided := files[path]; decided {
			continue
		}
		source, read := contents[listed[path].oid]
		if !read {
			files[path] = testBaseFile{reason: "cannot be read at base"}
			continue
		}
		declarations, err := golang.TestFileDeclarations(path, source)
		switch {
		case err != nil:
			files[path] = testBaseFile{reason: "does not parse at base"}
		case declarations.SideEffect != "":
			files[path] = testBaseFile{reason: declarations.SideEffect + " at base"}
		default:
			files[path] = testBaseFile{imports: declarations.Imports}
		}
	}
	return files
}

// testBatchContents parses `git cat-file --batch` output, `<oid> blob
// <size>\n<content>\n` per object, into each blob's content. It stops at the
// first record it cannot read, so an object it omits is read as missing.
func testBatchContents(output []byte) map[string][]byte {
	contents := make(map[string][]byte)
	for len(output) != 0 {
		header, rest, ok := bytes.Cut(output, []byte{'\n'})
		if !ok {
			break
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 || fields[1] != "blob" {
			break
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size+1 > len(rest) || rest[size] != '\n' {
			break
		}
		contents[fields[0]] = rest[:size]
		output = rest[size+1:]
	}
	return contents
}

// testSliceGitInput is sourceGit with stdin, for the one batched
// `cat-file --batch` read of every changed test file's base blob (STR-V0-072).
// It shares sourceGit's sanitized options, environment, executable override,
// deadline and process-group shutdown, and lives here so the frozen
// source-view implementation (ESV-V0-005) stays byte-identical.
func testSliceGitInput(ctx context.Context, root string, limit int, stdin []byte, arguments ...string) ([]byte, error) {
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, sourceRefusal("repository-root")
	}
	executable, _ := ctx.Value(sourceGitExecutableKey{}).(string)
	if executable == "" {
		executable = sourceGitPath
	}
	argv := append(append([]string{executable}, sourceGitOptions...), append([]string{"-C", directory}, arguments...)...)
	grace, shutdown := sourceShutdown(ctx)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: directory, Env: sourceGitEnvironment, Stdin: stdin, Timeout: sourceDeadline(ctx), ShutdownTimeout: shutdown, TerminateGrace: grace, OutputLimit: limit, StderrLimit: 8192})
	if observation.StdoutOverflow {
		return nil, errSourceOutputOverflow
	}
	if observation.Err != nil || observation.ExitStatus != 0 {
		return nil, sourceRefusal("git-failed")
	}
	return observation.Stdout, nil
}
