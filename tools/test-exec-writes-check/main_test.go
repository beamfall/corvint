package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestASDV0929RefusesExecutableWrites: an executable literal mode is refused with the fix,
// through a conversion and an import alias; a data mode, a variable mode and another
// package's WriteFile pass (V1-2587).
func TestASDV0929RefusesExecutableWrites(t *testing.T) {
	source := `package p

import (
	"io/fs"
	sys "os"

	"github.com/Beamfall/corvint/internal/execfile"
)

func f(perm fs.FileMode) {
	_ = sys.WriteFile("a", nil, 0o755)
	_ = sys.WriteFile("b", nil, fs.FileMode(0700))
	_ = sys.WriteFile("c", nil, 0o644)
	_ = sys.WriteFile("d", nil, perm)
	_ = execfile.WriteFile("e", nil, 0o755)
	_ = sys.WriteFile("f", nil,
		0o500)
}
`
	findings, err := scan("pkg/a_test.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("findings %q", findings)
	}
	for index, line := range []string{":11:", ":12:", ":16:"} {
		if !strings.Contains(findings[index], "FAIL pkg/a_test.go"+line) || !strings.Contains(findings[index], "fix: use execfile.WriteFile") {
			t.Errorf("finding %d = %q", index, findings[index])
		}
	}
	tasks, err := scan("internal/tasks/store/a_test.go", []byte(source))
	if err != nil || len(tasks) != 3 || !strings.Contains(tasks[0], "fix: use fixture.WriteExecutable") {
		t.Fatalf("tasks findings %q %v", tasks, err)
	}
}

func TestASDV0929Skips(t *testing.T) {
	for path, want := range map[string]bool{
		"internal/execfile/execfile_test.go":      true,
		"internal/x/testdata/a_test.go":           true,
		"internal/tasks/cli/live_upgrade_test.go": true,
		"internal/x/a_test.go":                    false,
	} {
		if skipped(path) != want {
			t.Errorf("skipped(%q) = %v", path, !want)
		}
	}
}

// TestASDV0929RepositoryIsClean runs the check over this repository: no tracked test file
// writes an executable with os.WriteFile.
func TestASDV0929RepositoryIsClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run("../..", &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, stdout.String(), stderr.String())
	}
}
