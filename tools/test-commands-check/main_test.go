package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// STR-V0-006: the repository declaration equals `make -s -n go-test`, and a
// one-flag mutation, an extra environment variable or a missing make target
// each fail the gate.
func TestSTRV0006DeclarationMatchesMakefile(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not on PATH")
	}
	root := filepath.Join("..", "..")
	original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath)))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath)), &stdout, &stderr); code != 0 {
		t.Fatalf("repository declaration: code %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "equals make go-test (GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./...)") {
		t.Fatalf("stdout %q", stdout.String())
	}
	mutations := map[string][2]string{
		"flag value":          {`"-count=1"`, `"-count=2"`},
		"flag order":          {`"-p", "1", "-timeout"`, `"-timeout", "-p", "1"`},
		"dropped flag":        {`"-p", "1", `, ``},
		"extra env":           {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "local", "GOFLAGS": ""`},
		"env value":           {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "go1.27"`},
		"missing make target": {`"languages": {`, `"languages": {"rust": ["make", "no-such-target"], `},
	}
	for name, mutation := range mutations {
		if !strings.Contains(string(original), mutation[0]) {
			t.Fatalf("%s: the declaration no longer contains %q", name, mutation[0])
		}
		mutated := filepath.Join(t.TempDir(), "test-commands.json")
		if err := os.WriteFile(mutated, []byte(strings.Replace(string(original), mutation[0], mutation[1], 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errs bytes.Buffer
		if code := run(root, mutated, &out, &errs); code != 1 || !strings.Contains(errs.String(), "test-commands-check: ") {
			t.Errorf("%s: code %d stderr %q", name, code, errs.String())
		}
	}
	var out, errs bytes.Buffer
	if code := run(root, filepath.Join(t.TempDir(), "absent.json"), &out, &errs); code != 1 || !strings.Contains(errs.String(), sessiontest.DeclarationPath) {
		t.Errorf("absent declaration: code %d stderr %q", code, errs.String())
	}
}

// A one-command recipe is required: a go-test target with two commands or
// a different package pattern is a difference, not a partial match.
func TestSTRV0006CompareGo(t *testing.T) {
	declared := sessiontest.Declared{GoEnv: []string{"A=1", "B=2"}, GoFlags: []string{"-p", "1"}}
	for recipe, want := range map[string]int{
		"B=2 A=1 go test -p 1 ./...\n":          0,
		"A=1 B=2 go test -p 1 ./...":            0,
		"A=1 B=2 go test -p 1 ./... \n\n":       0,
		"A=1 go test -p 1 ./...\n":              1,
		"A=1 B=2 go test -p 1 ./pkg/...\n":      1,
		"A=1 B=2 go test -p 1 -count=1 ./...\n": 1,
		"A=1 B=2 go test -p 1 ./...\ngo vet\n":  1,
		"":                                      1,
		"A=1 B=2 env go test -p 1 ./...\n":      1,
	} {
		if got := len(compareGo(declared, recipe)); (got != 0) != (want != 0) {
			t.Errorf("%q: %d problems, want %d", recipe, got, want)
		}
	}
}
