package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
)

// AFP-V0-9012: Go's own `./...` stays the authority for the FULL universe. It
// skips every directory whose name starts with "." or "_", so FULL appends
// exactly those package directories, found by this narrow walk. Everything
// else (symlinked files, build constraints, packages Go itself discovers at
// run time) is left to Go. A walk error refuses the run.
func hiddenPackages(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == root {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == ".git" || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		// Any non-directory entry counts, including a symlink Go would follow.
		if strings.HasSuffix(name, ".go") || name == "go.mod" {
			relative, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hiddenPackageDirs(paths), nil
}

// hiddenPackageDirs maps repository-relative file paths (a disk walk or a Git
// tree listing) to the sorted ./dir package arguments that `./...` skips:
// a directory below a "." or "_" element holding a Go file Go would read,
// outside testdata, vendor and nested modules (which `./...` also excludes).
func hiddenPackageDirs(paths []string) []string {
	modules := map[string]bool{}
	for _, file := range paths {
		if path.Base(file) == "go.mod" {
			modules[path.Dir(file)] = true
		}
	}
	found := map[string]bool{}
	for _, file := range paths {
		name, dir := path.Base(file), path.Dir(file)
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || dir == "." || found["./"+dir] {
			continue
		}
		hidden, excluded := false, false
		for _, element := range strings.Split(dir, "/") {
			hidden = hidden || strings.HasPrefix(element, ".") || strings.HasPrefix(element, "_")
			excluded = excluded || element == "testdata" || element == "vendor" || element == "." || element == ".."
		}
		for parent := dir; parent != "." && !excluded; parent = path.Dir(parent) {
			excluded = modules[parent]
		}
		if hidden && !excluded {
			found["./"+dir] = true
		}
	}
	packages := make([]string, 0, len(found))
	for dir := range found {
		packages = append(packages, dir)
	}
	sort.Strings(packages)
	return packages
}

// fullArgs is the FULL package argv: Go's ./... plus the hidden packages.
func fullArgs(hidden []string) []string {
	return append([]string{"./..."}, hidden...)
}

// canonicalFullArgs reports ./... followed by sorted, distinct hidden package
// arguments. A historical row's completeness is its separately listed
// expected universe, whose every package must report an outcome.
func canonicalFullArgs(args []string) bool {
	if len(args) == 0 || args[0] != "./..." {
		return false
	}
	paths := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		if !strings.HasPrefix(arg, "./") {
			return false
		}
		paths = append(paths, strings.TrimPrefix(arg, "./")+"/a.go")
	}
	return equal(args, fullArgs(hiddenPackageDirs(paths)))
}

// listArgs is `go list` with the build flags of testArgs, so the universe
// matches the execution's build context: a package whose only Go files are
// `//go:build race` exists for `go test -race` and must be listed too.
func listArgs(hidden []string) []string {
	args := []string{"list"}
	for _, arg := range testArgs {
		if arg == "-race" {
			args = append(args, arg)
		}
	}
	return append(args, fullArgs(hidden)...)
}

// writeSortedPackages prints `go list` output as sorted import paths, one per
// line: the wildcard lists visible packages before the appended hidden ones
// (AFP-V0-9012), so `--mode packages` sorts the combined list.
func writeSortedPackages(w io.Writer, listed []byte) error {
	packages := strings.Fields(string(listed))
	sort.Strings(packages)
	for _, p := range packages {
		if _, err := fmt.Fprintln(w, p); err != nil {
			return err
		}
	}
	return nil
}

// goUniverse is `go list` of the FULL argv in the execution build context:
// the complete package universe for sharded placement, rerun and shadow
// observation.
func goUniverse(ctx context.Context, o options) ([]string, error) {
	hidden, err := hiddenPackages(o.root)
	if err != nil {
		return nil, err
	}
	listed, err := capture(ctx, o, "go", listArgs(hidden)...)
	if err != nil {
		return nil, err
	}
	return cishards.Packages([]byte(listed))
}

// fullReceiptArgs checks a FULL receipt's package argv against the hidden
// packages of the target commit's own tree (AFP-V0-9012), never against the
// receipt. A receipt without hidden packages is the frozen ./... form.
func fullReceiptArgs(args []string, treeHidden func() ([]string, error)) error {
	if treeHidden == nil {
		return errors.New("FULL receipt cannot be checked without the target tree")
	}
	hidden, err := treeHidden()
	if err != nil {
		return err
	}
	if !equal(args, fullArgs(hidden)) {
		return errors.New("FULL receipt package argv differs from ./... plus the target's hidden packages")
	}
	return nil
}
