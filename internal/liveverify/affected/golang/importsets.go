package golang

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
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
// dirty module file.
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
		before, beforeErr := packageHeader(base, directory)
		after, afterErr := packageHeader(head, directory)
		if beforeErr == nil && afterErr == nil && before == after {
			continue
		}
		paths := byDirectory[directory]
		slices.Sort(paths)
		unit, found := directoryUnit(head, graph, directory, paths)
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
func packageHeader(fsys fs.FS, directory string) (string, error) {
	entries, err := fs.ReadDir(fsys, directory)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	var lines []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		relative := path.Join(directory, entry.Name())
		body, err := fs.ReadFile(fsys, relative)
		if err != nil {
			return "", err
		}
		file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return "", err
		}
		var constraints []string
		for _, group := range file.Comments {
			if group.Pos() >= file.Package {
				break
			}
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:build") || strings.HasPrefix(comment.Text, "// +build") {
					constraints = append(constraints, comment.Text)
				}
			}
		}
		// ImportsOnly stops before the declarations, so the embed lines
		// are read from the bytes.
		for _, line := range strings.Split(string(body), "\n") {
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//go:embed") {
				constraints = append(constraints, trimmed)
			}
		}
		imports := make([]string, 0, len(file.Imports))
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return "", err
			}
			imports = append(imports, value)
		}
		slices.Sort(imports)
		lines = append(lines, entry.Name()+"\x00"+strings.Join(constraints, "\x01")+"\x00"+file.Name.Name+"\x00"+strings.Join(imports, "\x01"))
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n"), nil
}
