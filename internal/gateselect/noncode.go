// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/noncodemap"
)

// Plan members the non-code decision reads (NCM-V1-001, NCM-V1-006).
const (
	unownedDirtyPath        = "UNOWNED_DIRTY_PATH"
	declaredDocumentation   = "DECLARED_DOCUMENTATION"
	e2eFlowsInvalid         = "E2E_FLOWS_INVALID"
	flowsDeclarationInvalid = "FLOWS_DECLARATION_INVALID"
)

// maxTestSourceBytes bounds one _test.go file read to confirm a map entry's
// cited Go test (NCM-V1-008).
const maxTestSourceBytes = 4 << 20

// nonCodeDecision applies docs/specs/non-code-map-v1.md to a plan: it reads
// the repository's map (a missing map is empty; an invalid one is returned as
// the error, never read as empty) and decides each dirty path from the plan's
// UNOWNED_DIRTY_PATH entries, its DECLARED_DOCUMENTATION group, its flow join
// and this index. It writes nothing (NCM-V1-009).
func nonCodeDecision(plan Receipt, index *repositoryIndex, root string) (noncodemap.Decision, error) {
	read, err := noncodemap.Read(root)
	if err != nil {
		return noncodemap.Decision{}, err
	}
	input := noncodemap.Input{
		Dirty:    plan.Plan.Dirty,
		Map:      read,
		Embedded: index.embedded,
		Enclosed: index.enclosed,
		Env: noncodemap.Env{
			TestExists: func(test string) bool { return index.testExists(root, test) },
			IsAncestor: func(commit string) (bool, error) { return ancestorOfHead(root, commit) },
		},
	}
	for _, entry := range plan.Plan.Unknown {
		switch entry.Reason {
		case unownedDirtyPath:
			input.Unowned = append(input.Unowned, entry.Detail)
		case e2eFlowsInvalid:
			input.FlowsInvalid = entry.Reason + " " + entry.Detail
		}
	}
	for _, group := range plan.Plan.NonSource {
		if group.Class == declaredDocumentation {
			input.Documentation = append(input.Documentation, group.Paths...)
		}
	}
	if plan.Plan.E2E != nil {
		input.FlowsJoined = true
		input.FlowsDirectory = plan.Plan.E2E.Flows
		for _, entry := range plan.Plan.E2E.Unknown {
			if entry.Reason == flowsDeclarationInvalid {
				input.FlowsInvalid = entry.Reason + " " + entry.Detail
			}
		}
	}
	deleted := map[string]bool{}
	for _, p := range plan.Plan.Deleted {
		deleted[p] = true
	}
	for _, p := range plan.Plan.Dirty {
		if category, never := noncodemap.NeverNarrowed(p); !never || category != noncodemap.CategoryRunnerConfig {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p))); deleted[p] || errors.Is(err, fs.ErrNotExist) {
			input.Deleted = append(input.Deleted, p)
		}
	}
	return noncodemap.Decide(input), nil
}

// enclosingPackage returns the directory of the nearest indexed Go package
// at or above p's directory.
func (index *repositoryIndex) enclosingPackage(p string) (string, bool) {
	for directory := path.Dir(p); ; directory = path.Dir(directory) {
		key := directory
		if key == "." {
			key = ""
		}
		if index.packages[key] != nil {
			return key, true
		}
		if directory == "." {
			return "", false
		}
	}
}

// enclosed reports a path a Go package directory encloses: the Go runner
// owns the tree that holds it (NCM-V1-001).
func (index *repositoryIndex) enclosed(p string) bool {
	_, ok := index.enclosingPackage(p)
	return ok
}

// embedded reports a non-Go file whose nearest enclosing Go package embeds
// files: it may be a go:embed input, which is never narrowed (NCM-V1-005).
// The index records that a package embeds, not which files, so every file
// below it counts, which can only widen.
func (index *repositoryIndex) embedded(p string) bool {
	if strings.HasSuffix(p, ".go") {
		return false
	}
	directory, ok := index.enclosingPackage(p)
	return ok && index.packages[directory].embeds
}

// testExists confirms a map entry's cited test (NCM-V1-008): a Go test
// `go:<dir>` or `go:<dir>::<Name>[::<subtest>...]` needs an indexed package
// at dir and, with a name, a _test.go file there declaring `func <Name>(`;
// a test of another runner, `<runner>:<file>[::...]`, needs its file to be a
// regular file. Any other form is not confirmed.
func (index *repositoryIndex) testExists(root, test string) bool {
	runner, rest, ok := strings.Cut(test, ":")
	if !ok || runner == "" {
		return false
	}
	file, name, _ := strings.Cut(rest, "::")
	if name != "" {
		name, _, _ = strings.Cut(name, "::")
	}
	if runner != "go" {
		if !noncodemap.CleanPath(file) {
			return false
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(file)))
		return err == nil && info.Mode().IsRegular()
	}
	directory := file
	if directory == "." {
		directory = ""
	} else if !noncodemap.CleanPath(directory) {
		return false
	}
	if index.packages[directory] == nil {
		return false
	}
	if name == "" {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), "_test.go") && declaresFunc(filepath.Join(root, filepath.FromSlash(file), entry.Name()), name) {
			return true
		}
	}
	return false
}

// declaresFunc reports whether the file holds a line starting `func <name>(`.
func declaresFunc(name, function string) bool {
	file, err := os.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, maxTestSourceBytes))
	scanner.Buffer(make([]byte, 0, 64<<10), maxTestSourceBytes)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "func "+function+"(") {
			return true
		}
	}
	return false
}

// ancestorOfHead reports whether commit is an ancestor of root's HEAD
// (NCM-V1-008), with the read-only hermetic Git of baseGit.
func ancestorOfHead(root, commit string) (bool, error) {
	_, err := baseGit(root, 0, "merge-base", "--is-ancestor", commit, "HEAD")
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, err
}
