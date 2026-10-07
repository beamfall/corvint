// Package tasks holds only the import-direction test for the in-tree
// corvint-tasks companion (decision 0397). The companion's code lives in the
// subpackages and cmd/corvint-tasks.
package tasks

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePrefix = "github.com/Beamfall/corvint/"
	tasksPrefix  = modulePrefix + "internal/tasks/"
	tasksWire    = modulePrefix + "internal/tasks/wire"
)

// importViolation names the rule a module-internal import breaks for the Go
// file at rel (slash-separated, relative to the module root), or "".
func importViolation(rel, path string) string {
	if !strings.HasPrefix(path, modulePrefix) {
		return ""
	}
	tasksSide := strings.HasPrefix(rel, "internal/tasks/") || strings.HasPrefix(rel, "cmd/corvint-tasks/")
	if strings.HasPrefix(rel, "internal/tasks/wire/") {
		return "the Tasks wire package imports no module package"
	}
	// CAL-V0-022's accepted in-process pack adapter is one production
	// exception to decision 0397 rule 2; its Core dependencies remain explicit.
	if strings.HasPrefix(rel, "internal/tasks/scopes/") && (path == modulePrefix+"internal/contextindex" || path == modulePrefix+"internal/runtimeenv") {
		return ""
	}
	if rel == "internal/tasks/cli/scope_test.go" && path == modulePrefix+"internal/contextindex" {
		return ""
	}
	// The dispatcher's process-group Owner edge (decision 0397's #464 addendum)
	// admits only the standard-library-only internal/groupreap.
	if strings.HasPrefix(rel, "internal/tasks/dispatch/") && path == modulePrefix+"internal/groupreap" {
		return ""
	}
	// The #481 attempt runner's process-group Owner edge (same addendum)
	// admits only its two files.
	if (rel == "internal/tasks/cli/attempt_run.go" || rel == "internal/tasks/cli/attempt_run_test.go") && path == modulePrefix+"internal/groupreap" {
		return ""
	}
	// The SERVICE500-007 helper wrapper's process-group Owner edge (decision
	// 0397's V1-0804 addendum) admits only its Linux descendant files.
	if (rel == "internal/tasks/service/descendants_linux.go" || rel == "internal/tasks/service/descendants_linux_test.go") && path == modulePrefix+"internal/groupreap" {
		return ""
	}
	// The know-how write's secret screen (decision 0397's V1-0955 addendum)
	// admits only the mutation step file and its test.
	if (rel == "internal/tasks/mutation/know_how.go" || rel == "internal/tasks/mutation/know_how_test.go") && path == modulePrefix+"internal/secretscreen" {
		return ""
	}
	if tasksSide && !strings.HasPrefix(path, tasksPrefix) {
		return "Tasks imports no Core package"
	}
	if tasksSide || !strings.HasPrefix(path, tasksPrefix) {
		return ""
	}
	if strings.HasPrefix(rel, "cmd/corvint/") {
		return "cmd/corvint imports no Tasks package"
	}
	if path != tasksWire {
		return "Core imports only the Tasks wire package"
	}
	return ""
}

// skipDir reports directories that are not part of this module's Go source:
// VCS and tool state, test data, dependencies and nested modules.
func skipDir(path string, d fs.DirEntry, root string) bool {
	name := d.Name()
	if path == root {
		return false
	}
	if strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules" || name == "vendor" {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

func TestImportViolationControls(t *testing.T) {
	cases := []struct{ rel, path string }{
		{"internal/tasks/cli/cli.go", modulePrefix + "internal/gitstatus"},
		{"cmd/corvint-tasks/main.go", modulePrefix + "internal/taskman"},
		{"internal/tasks/wire/codes.go", tasksPrefix + "ticket"},
		{"internal/taskman/decode.go", tasksPrefix + "store"},
		{"cmd/corvint/main.go", tasksWire},
		{"internal/tasks/scopes/derive.go", modulePrefix + "internal/gitstatus"},
		{"internal/tasks/cli/cli.go", modulePrefix + "internal/contextindex"},
		{"internal/tasks/cli/scope_test.go", modulePrefix + "internal/runtimeenv"},
		{"internal/tasks/cli/dispatch.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/dispatch/loop.go", modulePrefix + "internal/gokernel"},
		{"internal/tasks/cli/cli.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/cli/attempt_run_helper.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/cli/attempt_run.go", modulePrefix + "internal/gitstatus"},
		{"internal/tasks/cli/attempt_run_test.go", modulePrefix + "internal/contextindex"},
		{"internal/tasks/service/helper.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/service/descendants_linux.go", modulePrefix + "internal/gitstatus"},
		{"internal/tasks/mutation/apply.go", modulePrefix + "internal/secretscreen"},
		{"internal/tasks/cli/know_how.go", modulePrefix + "internal/secretscreen"},
		{"internal/tasks/mutation/know_how.go", modulePrefix + "internal/gitstatus"},
	}
	for _, c := range cases {
		if importViolation(c.rel, c.path) == "" {
			t.Errorf("SPY_INEFFECTIVE %s importing %s", c.rel, c.path)
		}
	}
	if got := importViolation("internal/taskman/decode.go", tasksWire); got != "" {
		t.Errorf("Core import of the wire package refused: %s", got)
	}
	for _, c := range []struct{ rel, path string }{
		{"internal/tasks/scopes/derive.go", modulePrefix + "internal/contextindex"},
		{"internal/tasks/scopes/derive.go", modulePrefix + "internal/runtimeenv"},
		{"internal/tasks/cli/scope_test.go", modulePrefix + "internal/contextindex"},
		{"internal/tasks/dispatch/loop.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/dispatch/reader_test.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/cli/attempt_run.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/cli/attempt_run_test.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/service/descendants_linux.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/service/descendants_linux_test.go", modulePrefix + "internal/groupreap"},
		{"internal/tasks/mutation/know_how.go", modulePrefix + "internal/secretscreen"},
		{"internal/tasks/mutation/know_how_test.go", modulePrefix + "internal/secretscreen"},
	} {
		if got := importViolation(c.rel, c.path); got != "" {
			t.Errorf("accepted Core edge refused: %s", got)
		}
	}
}

// Decision 0397 and its CAL-V0-022 addendum: apart from the pack adapter,
// Tasks imports no Core package, Core imports only the Tasks
// wire package (which imports no module package), and cmd/corvint imports no
// Tasks package, so the corvint binary links no Tasks mutation code.
func TestImportDirection(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skipDir(path, d, root) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			imported, _ := strconv.Unquote(spec.Path.Value)
			if rule := importViolation(filepath.ToSlash(rel), imported); rule != "" {
				t.Errorf("%s imports %s: %s", filepath.ToSlash(rel), imported, rule)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
