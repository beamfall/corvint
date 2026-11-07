package golang

import (
	"bytes"
	"context"
	"errors"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// ImportSetChangesAt compares the import set of each Go package directory
// holding a dirty .go path at commit, read from the repository under root,
// with the tree under head (AFP-V0-2202). It returns nil, which the selector
// treats as unknown, when the base cannot be read; an empty comparison
// without a Git read when no dirty path can change an import set or the
// graph has no import-graph reader.
func ImportSetChangesAt(ctx context.Context, root, head, commit string, graph *affected.Graph, dirty []string) *affected.ImportSetChanges {
	if !graph.HasImportGraphReaders() || len(dirty) == 0 {
		return &affected.ImportSetChanges{Changed: map[string]string{}}
	}
	if commit == "" {
		return nil
	}
	repository, err := gitauth.Open(root, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return nil
	}
	defer repository.BeginObjectSession()()
	base, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return nil
	}
	return ImportSetRecords(base, os.DirFS(head), graph, dirty)
}

// importSetPath reports a dirty path that can change an import set: a module
// file, or a .go file the go tool may build.
func importSetPath(relative string) bool {
	return affected.ModuleFile(relative) || strings.HasSuffix(relative, ".go") && !affected.GoIgnoredPath(relative)
}

// ImportSetRecords is ImportSetChangesAt over two source trees: base, the
// tree the change is measured from, and head, the tree the graph was built
// from. A directory's import set is its package header: for every .go file,
// test files included, its name, its build-constraint lines, its
// `//go:embed` lines, its package clause and its sorted import paths. A
// dirty data file present on one side only, below a Go package that embeds,
// changes that package's import set too: `go list` fails on an embed
// pattern that matches nothing. A directory whose header differs, or
// cannot be read or parsed on either side, changed; a changed directory that
// holds no unit of the graph changes the module's import graph, as does a
// dirty module file. A directory whose folded header (AFP-V0-9010) is kept
// but whose header of every file's own line differs changed only its file
// set: its unit goes to Files, which only an unproven reader observes.
func ImportSetRecords(base, head fs.FS, graph *affected.Graph, dirty []string) *affected.ImportSetChanges {
	changes := &affected.ImportSetChanges{Changed: map[string]string{}}
	byDirectory := map[string][]string{}
	var directories []string
	for _, relative := range dirty {
		switch {
		case affected.ModuleFile(relative):
			if changes.Module == "" {
				changes.Module = relative
			}
		case importSetPath(relative):
			directory := path.Dir(relative)
			if _, seen := byDirectory[directory]; !seen {
				directories = append(directories, directory)
			}
			byDirectory[directory] = append(byDirectory[directory], relative)
		case !strings.HasSuffix(relative, ".go") && existsOnOneSide(base, head, relative):
			for _, unit := range embeddingUnits(head, graph, relative) {
				if _, seen := changes.Changed[unit]; !seen {
					changes.Changed[unit] = relative
				}
			}
		}
	}
	slices.Sort(directories)
	for _, directory := range directories {
		before, beforeFiles, beforeErr := packageHeader(base, directory)
		after, afterFiles, afterErr := packageHeader(head, directory)
		filesOnly := false
		if beforeErr == nil && afterErr == nil && before == after {
			if beforeFiles == afterFiles {
				continue
			}
			filesOnly = true
		}
		paths := byDirectory[directory]
		slices.Sort(paths)
		unit, found := directoryUnit(head, graph, directory, paths)
		if filesOnly && found {
			if changes.Files == nil {
				changes.Files = map[string]string{}
			}
			if _, seen := changes.Files[unit]; !seen {
				changes.Files[unit] = paths[0]
			}
			continue
		}
		if !found {
			if changes.Module == "" {
				changes.Module = paths[0]
			}
			continue
		}
		if _, seen := changes.Changed[unit]; !seen {
			changes.Changed[unit] = paths[0]
		}
	}
	return changes
}

// existsOnOneSide reports a path present in exactly one of the two trees, or
// one whose presence cannot be read on either side.
func existsOnOneSide(base, head fs.FS, relative string) bool {
	_, beforeErr := fs.Stat(base, relative)
	_, afterErr := fs.Stat(head, relative)
	before, after := beforeErr == nil, afterErr == nil
	if beforeErr != nil && !errors.Is(beforeErr, fs.ErrNotExist) || afterErr != nil && !errors.Is(afterErr, fs.ErrNotExist) {
		return true
	}
	return before != after
}

// embeddingUnits lists the Go units in the directories enclosing relative
// whose package embeds files.
func embeddingUnits(head fs.FS, graph *affected.Graph, relative string) []string {
	var units []string
	for directory := path.Dir(relative); ; directory = path.Dir(directory) {
		if id, found := directoryUnit(head, graph, directory, nil); found {
			if unit, known := graph.Unit(id); known && unit.Embeds {
				units = append(units, id)
			}
		}
		if directory == "." || directory == "/" {
			return units
		}
	}
}

// directoryUnit returns the Go unit that owns a .go file of directory in the
// tree: one of its dirty paths, else any other .go file listed there.
func directoryUnit(head fs.FS, graph *affected.Graph, directory string, paths []string) (string, bool) {
	candidates := slices.Clone(paths)
	if entries, err := fs.ReadDir(head, directory); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				candidates = append(candidates, path.Join(directory, entry.Name()))
			}
		}
	}
	for _, candidate := range candidates {
		if id, owned := graph.OwnerOf(candidate); owned && strings.HasPrefix(id, "go:") {
			return id, true
		}
	}
	return "", false
}

// packageHeader is the import-set signature of one directory in fsys: one
// line per .go file, sorted, of its name, build-constraint lines, package
// clause and sorted import paths. An absent directory has the empty header.
// It returns the folded header below and files, the header with every
// file's own line, which a reader not proven to observe the import graph
// alone compares (AFP-V0-9010).
//
// A test file adds an import edge to no other package unless its imports
// leave its package's test import set (AFP-V0-9010). So when the directory's
// non-test files share one package clause N, and every unconstrained test
// file (no build-constraint line, no GOOS or GOARCH in its name, no import of
// "C") declares N or N_test, those test files contribute no line of their
// own: the header instead holds the sorted set of import paths of the N test
// files, that of the N_test files, and the sorted `//go:embed` lines of both.
// A test file that keeps those sets keeps the folded header, though an
// unproven reader still sees it through files; one that adds an import path
// to its role's set, adds an embed line, declares another package clause or
// carries a constraint still changes it.
func packageHeader(fsys fs.FS, directory string) (string, string, error) {
	entries, err := fs.ReadDir(fsys, directory)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", nil
		}
		return "", "", err
	}
	var lines, files []string
	var tests []headerFile
	clauses := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		relative := path.Join(directory, entry.Name())
		body, err := fs.ReadFile(fsys, relative)
		if err != nil {
			return "", "", err
		}
		file, err := readHeaderFile(entry.Name(), relative, body)
		if err != nil {
			return "", "", err
		}
		files = append(files, file.line())
		if strings.HasSuffix(entry.Name(), "_test.go") && file.unconstrained {
			tests = append(tests, file)
			continue
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			clauses[file.clause] = true
		}
		lines = append(lines, file.line())
	}
	if len(tests) != 0 {
		if summary, ok := testSummary(clauses, tests); ok {
			lines = append(lines, summary)
		} else {
			for _, file := range tests {
				lines = append(lines, file.line())
			}
		}
	}
	slices.Sort(lines)
	slices.Sort(files)
	return strings.Join(lines, "\n"), strings.Join(files, "\n"), nil
}

// headerFile is the header of one .go file: its name, its build-constraint
// lines, its `//go:embed` lines, its package clause and its sorted import
// paths. Unconstrained reports a file the go tool builds on every platform:
// no build-constraint line, no GOOS or GOARCH in its name and no import of
// "C".
type headerFile struct {
	name          string
	constraints   []string
	embeds        []string
	clause        string
	imports       []string
	unconstrained bool
}

// line is the file's own header line.
func (file headerFile) line() string {
	return file.name + "\x00" + strings.Join(append(slices.Clone(file.constraints), file.embeds...), "\x01") + "\x00" + file.clause + "\x00" + strings.Join(file.imports, "\x01")
}

// readHeaderFile parses the header of one .go file named name.
func readHeaderFile(name, relative string, body []byte) (headerFile, error) {
	file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return headerFile{}, err
	}
	header := headerFile{name: name, clause: file.Name.Name}
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:build") || strings.HasPrefix(comment.Text, "// +build") {
				header.constraints = append(header.constraints, comment.Text)
			}
		}
	}
	// ImportsOnly stops before the declarations, so the embed lines are
	// read from the bytes.
	for _, line := range strings.Split(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//go:embed") {
			header.embeds = append(header.embeds, trimmed)
		}
	}
	header.imports = make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return headerFile{}, err
		}
		header.imports = append(header.imports, value)
	}
	slices.Sort(header.imports)
	header.unconstrained = len(header.constraints) == 0 && !slices.Contains(header.imports, "C") && platformFree(name, body)
	return header, nil
}

// platformFree reports a file name that names no GOOS or GOARCH: the go
// tool's own name rule, asked for a platform no name can name.
func platformFree(name string, body []byte) bool {
	context := build.Context{GOOS: "corvint-none", GOARCH: "corvint-none", Compiler: "gc"}
	context.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	match, err := context.MatchFile(".", name)
	return err == nil && match
}

// testSummary folds the unconstrained test files of a directory whose
// non-test files declare exactly the clauses into one header line, when
// there is one clause N that does not itself end in _test and every such
// test file declares N or N_test (AFP-V0-9010).
func testSummary(clauses map[string]bool, tests []headerFile) (string, bool) {
	if len(clauses) != 1 {
		return "", false
	}
	var name string
	for clause := range clauses {
		name = clause
	}
	if strings.HasSuffix(name, "_test") || name == "documentation" {
		return "", false
	}
	internal, external := map[string]bool{}, map[string]bool{}
	var embeds []string
	for _, file := range tests {
		var role map[string]bool
		switch file.clause {
		case name:
			role = internal
		case name + "_test":
			role = external
		default:
			return "", false
		}
		for _, value := range file.imports {
			role[value] = true
		}
		embeds = append(embeds, file.embeds...)
	}
	slices.Sort(embeds)
	return "\x00tests\x00" + strings.Join(sortedKeys(internal), "\x01") + "\x00" + strings.Join(sortedKeys(external), "\x01") + "\x00" + strings.Join(embeds, "\x01"), true
}
