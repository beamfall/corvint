package affected_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestTasksClosureChangeSelectsCompanionRelease_V14050: an edit to any Go file
// of a package in corvint-tasks' import closure selects internal/companionrelease,
// whose TestCurrentTasksSourceArchiveBuildsOffline rebuilds corvint-tasks offline
// from the exported subset. A new import in such a package can pull a package
// outside that subset into the closure (imp-u16 made internal/contextindex import
// the affected planner), and only that test sees it. The closure is read from
// `go list -deps ./cmd/corvint-tasks`, so a package that joins it is covered too.
func TestTasksClosureChangeSelectsCompanionRelease_V14050(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	module, err := golang.ReadModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	list := exec.Command("go", "list", "-deps", "-f", `{{if and (not .Standard) .GoFiles}}{{.Dir}}{{"\t"}}{{index .GoFiles 0}}{{end}}`, "./cmd/corvint-tasks")
	list.Dir = root
	list.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	output, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	want := "go:" + module + "/internal/companionrelease"
	checked := 0
	for _, line := range listedDirectories(string(output)) {
		directory, file, ok := strings.Cut(line, "\t")
		relative, err := filepath.Rel(root, directory)
		if !ok || err != nil || strings.HasPrefix(relative, "..") {
			continue // a module dependency outside the repository is not a repository edit
		}
		dirty := filepath.ToSlash(filepath.Join(relative, file))
		plan := affected.Select(graph, []string{dirty})
		selected := false
		for _, selection := range plan.Selected {
			selected = selected || selection.UnitID == want
		}
		if !selected {
			t.Errorf("%s: internal/companionrelease is not selected; scope %s, unknown %+v", dirty, plan.Scope, plan.Unknown)
		}
		checked++
	}
	// The closure holds the Tasks packages and their Core dependencies; far fewer
	// means go list read the wrong package and the loop above proved nothing.
	if checked < 30 {
		t.Fatalf("checked %d corvint-tasks closure packages, want at least 30:\n%s", checked, output)
	}
}
