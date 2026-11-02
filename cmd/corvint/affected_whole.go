package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedWhole is one selected Go unit whose every test the change can
// reach because the change adds, removes or edits the package's TestMain,
// which wraps every test of the package (AFP-V0-550, V1-3688).
type affectedWhole struct {
	Unit   string `json:"unit"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// withWholePackages adds a whole-package verdict for each selected Go unit
// whose dirty `_test.go` file adds, removes or changes its TestMain between
// base (or the HEAD revision when base is "") and the tree, so a caller can run the
// package whole and name that reason instead of writing one by hand. A test
// file that cannot be read or parsed on either side keeps its unit whole
// with the reason stated: the verdict never narrows anything.
func withWholePackages(ctx context.Context, advice affectedAdvice, repository, tree, base, revision string, plan affected.Plan) affectedAdvice {
	units := map[string]string{}
	for _, selection := range plan.Selected {
		for _, test := range selection.Tests {
			units[test] = selection.UnitID
		}
	}
	var paths []string
	for _, path := range plan.Dirty {
		if strings.HasSuffix(path, "_test.go") && units[path] != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return advice
	}
	if base != "" {
		revision = base
	}
	before := testBaseBlobs(ctx, repository, revision, paths)
	seen := map[string]bool{}
	for _, path := range paths {
		unit := units[path]
		if seen[unit] {
			continue
		}
		reason := ""
		old, oldErr := testMainSource(before[path])
		data, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(path)))
		current, currentErr := []byte(nil), error(nil)
		if err == nil {
			current, currentErr = testMainOf(path, data)
		} else if !os.IsNotExist(err) {
			currentErr = err
		}
		switch {
		case oldErr != nil || currentErr != nil:
			reason = path + " cannot be read or parsed on both sides, so a TestMain change cannot be ruled out"
		case old == nil && current != nil:
			reason = path + " adds TestMain, which wraps every test of the package"
		case old != nil && current == nil:
			reason = path + " removes TestMain, which wrapped every test of the package"
		case old != nil && !bytes.Equal(old, current):
			reason = path + " changes TestMain, which wraps every test of the package"
		}
		if reason != "" {
			seen[unit] = true
			advice.WholePackages = append(advice.WholePackages, affectedWhole{Unit: unit, Path: path, Reason: reason})
		}
	}
	sort.Slice(advice.WholePackages, func(i, j int) bool { return advice.WholePackages[i].Unit < advice.WholePackages[j].Unit })
	return advice
}

// testMainSource is the TestMain declaration of a base blob, nil when the
// path or the function is absent there.
func testMainSource(blob testBaseBlob) ([]byte, error) {
	switch {
	case blob.absent:
		return nil, nil
	case blob.reason != "":
		return nil, os.ErrNotExist
	}
	return testMainOf("base", blob.content)
}

// testMainOf returns the source bytes of the file's top-level TestMain
// function, nil when it declares none.
func testMainOf(name string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "TestMain" {
			start, end := fset.Position(fn.Pos()).Offset, fset.Position(fn.End()).Offset
			return src[start:end], nil
		}
	}
	return nil, nil
}
