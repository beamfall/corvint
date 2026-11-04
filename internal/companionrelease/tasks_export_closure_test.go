package companionrelease

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTasksExportCoversCorvintTasksImportClosure (V1-4050, PUB-V0-012) lists the
// non-standard import closure of ./cmd/corvint-tasks and requires every package
// directory in it to fall inside tasksExportFiles/tasksExportPrefixes. A package of
// that closure that gains an import outside the subset otherwise surfaces only as
// a failed offline build in TestCurrentTasksSourceArchiveBuildsOffline; this check
// names the missing directory in seconds.
func TestTasksExportCoversCorvintTasksImportClosure(t *testing.T) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goPath, "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", "./cmd/corvint-tasks")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("go list -deps ./cmd/corvint-tasks: %v\n%s", err, stderr)
	}
	packages := 0
	for _, dir := range goListDirs(string(out)) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			t.Fatalf("corvint-tasks imports %s, outside the module; the offline archive carries no other module", dir)
		}
		packages++
		if !inTasksExport(filepath.ToSlash(rel) + "/") {
			t.Errorf("corvint-tasks imports %s, which the Tasks source export omits: add %q to tasksExportPrefixes or drop the import", rel, filepath.ToSlash(rel)+"/")
		}
	}
	if packages == 0 {
		t.Fatal("go list reported no corvint-tasks packages")
	}
}

// goListDirs returns the non-empty lines of go list output, one package directory each. A checkout
// path may contain spaces, so the output is split on line ends only.
func goListDirs(out string) []string {
	var dirs []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSuffix(line, "\r"); line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs
}

func TestGoListDirsKeepsDirectoriesWithSpaces(t *testing.T) {
	out := "/work/Corvint Review/internal/tasks\n\n/work/Corvint Review/cmd/corvint-tasks\r\n"
	got := goListDirs(out)
	want := []string{"/work/Corvint Review/internal/tasks", "/work/Corvint Review/cmd/corvint-tasks"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("goListDirs = %q, want %q", got, want)
	}
}
