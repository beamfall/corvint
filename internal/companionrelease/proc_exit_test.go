//go:build !windows

package companionrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// V1-3176: a tool killed by a signal must name the signal, not only an exit
// code and whatever stderr it managed to write before it died.
func TestRunCapturedReportsKillingSignal(t *testing.T) {
	_, _, err := runCaptured(context.Background(), t.TempDir(), []string{"PATH=/bin:/usr/bin"}, subprocessTimeout,
		"/bin/sh", "-c", "echo '# example.com/pkg' >&2; kill -KILL $$")
	if err == nil {
		t.Fatal("runCaptured succeeded for a killed process")
	}
	for _, want := range []string{"signal=killed", "# example.com/pkg"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}

func TestRunGoBuildReportsCompileErrorStderr(t *testing.T) {
	goPath, err := goBinary()
	if err != nil {
		t.Skipf("go not on PATH: %v", err)
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/broken\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() { undefinedName() }\n")
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "GOCACHE=" + filepath.Join(root, "cache"),
		"GOPATH=" + filepath.Join(root, "gopath"), "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local"}
	err = runGoBuild(context.Background(), goPath, root, ".", filepath.Join(root, "out"), env, nil)
	if err == nil {
		t.Fatal("runGoBuild succeeded on a broken package")
	}
	for _, want := range []string{"undefined: undefinedName", "exit=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "signal=") {
		t.Fatalf("compile error reported as a signal: %q", err)
	}
	if strings.Count(err.Error(), "undefined: undefinedName") != 1 {
		t.Fatalf("compile stderr repeated in %q", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
