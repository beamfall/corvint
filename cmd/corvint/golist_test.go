package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goListOutput runs a prepared `go list` command and returns its stdout. On
// failure the error carries go list's stderr, so a toolchain or module error
// names its cause instead of surfacing as a bare exit status (V1-2257).
func goListOutput(command *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return output, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

// TestGoListOutputCarriesStderr is V1-2257: a deliberately broken module makes
// go list fail, and the returned error contains go list's own error text.
func TestGoListOutputCarriesStderr(t *testing.T) {
	t.Parallel()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/broken\n\ngo 1.21\n\nrequire (\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(goTool, "list", "-deps", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=", "GOWORK=off")
	_, err = goListOutput(command)
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("broken-module go list error = %v, want go list's stderr naming go.mod", err)
	}
}
