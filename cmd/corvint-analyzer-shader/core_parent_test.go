package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The candidate's invariant is its absence from the core dependency closure.
func TestCandidateIsUnselectedByCore(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	dependencies, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go list -deps: %v: %s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	if strings.Contains(string(dependencies), "internal/analyzershader") || strings.Contains(string(dependencies), "corvint-analyzer-shader") {
		t.Fatalf("Core selected shader candidate: %s", dependencies)
	}
}
