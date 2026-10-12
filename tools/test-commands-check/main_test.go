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
	// Every mutation is a readable declaration (the reader is not what fails)
	// and names the diagnostic the comparison it targets must print.
	mutations := map[string][3]string{
		"flag value":          {`"-count=1"`, `"-count=2"`, "go.flags give"},
		"flag order":          {`"-p", "1", "-timeout"`, `"-timeout", "-p", "1"`, "go.flags give"},
		"dropped flag":        {`"-p", "1", `, ``, "go.flags give"},
		"extra env":           {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "local", "GOFLAGS": "-mod=mod"`, "go.env is"},
		"env value":           {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "go1.27"`, "go.env is"},
		"missing make target": {`"languages": {`, `"languages": {"zig": ["make", "no-such-target"], `, "languages.zig names make no-such-target"},
	}
	for name, mutation := range mutations {
		if !strings.Contains(string(original), mutation[0]) {
			t.Fatalf("%s: the declaration no longer contains %q", name, mutation[0])
		}
		mutated := filepath.Join(t.TempDir(), "test-commands.json")
		body := strings.Replace(string(original), mutation[0], mutation[1], 1)
		if err := os.WriteFile(mutated, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if declared := sessiontest.ReadDeclaredFile(mutated); declared.Unreadable != "" {
			t.Fatalf("%s: the mutation is unreadable (%s), so it would not reach the comparison", name, declared.Unreadable)
		}
		var out, errs bytes.Buffer
		if code := run(root, mutated, &out, &errs); code != 1 || !strings.Contains(errs.String(), "test-commands-check: "+mutation[2]) {
			t.Errorf("%s: code %d stderr %q, want %q", name, code, errs.String(), mutation[2])
		}
	}
	var out, errs bytes.Buffer
	if code := run(root, filepath.Join(t.TempDir(), "absent.json"), &out, &errs); code != 1 || !strings.Contains(errs.String(), sessiontest.DeclarationPath) {
		t.Errorf("absent declaration: code %d stderr %q", code, errs.String())
	}
}

// A one-command recipe is required: a go-test target with two commands or
// a different package pattern is a difference, not a partial match, and a
// word the shell would rewrite (quotes, redirections) is a difference even
// when the declaration spells it the same way.
func TestSTRV0006CompareGo(t *testing.T) {
	quoted := sessiontest.Declared{GoEnv: []string{"A=1", "B=2"}, GoFlags: []string{"-run='TestA|TestB'"}}
	if got := compareGo(quoted, "A=1 B=2 go test -run='TestA|TestB' ./...\n"); len(got) == 0 {
		t.Errorf("quoted flag spelled identically on both sides passed")
	}
	dashed := sessiontest.Declared{GoEnv: []string{"A-B=1"}, GoFlags: []string{"-p", "1"}}
	if got := compareGo(dashed, "A-B=1 go test -p 1 ./...\n"); len(got) == 0 {
		t.Errorf("A-B=1 spelled identically on both sides passed; the shell would run it as a command")
	}
	declared := sessiontest.Declared{GoEnv: []string{"A=1", "B=2"}, GoFlags: []string{"-p", "1"}}
	for recipe, want := range map[string]int{
		"B=2 A=1 go test -p 1 ./...\n":           0,
		"A=1 B=2 go test -p 1 ./...":             0,
		"A=1 B=2 go test -p 1 ./... \n\n":        0,
		"A=1 go test -p 1 ./...\n":               1,
		"A=1 B=2 go test -p 1 ./pkg/...\n":       1,
		"A=1 B=2 go test -p 1 -count=1 ./...\n":  1,
		"A=1 B=2 go test -p 1 ./...\ngo vet\n":   1,
		"":                                       1,
		"A=1 B=2 env go test -p 1 ./...\n":       1,
		"A=1 B=2 go test -p '1' ./...\n":         1,
		"A=1 B='2' go test -p 1 ./...\n":         1,
		"A=1 B=2 go test -p 1 ./... 2>/dev/null": 1,
		"A=1 B=2 go test -p\u00a01 ./...\n":      1,
		"A=1 B=2 go test -p 1 ./...\r\n":         1,
		"A=1\tB=2\tgo test -p 1 ./...\n":         0,
		"\u00a0\nA=1 B=2 go test -p 1 ./...\n":   1,
		" \t\nA=1 B=2 go test -p 1 ./...\n\t\n":  0,
	} {
		if got := len(compareGo(declared, recipe)); (got != 0) != (want != 0) {
			t.Errorf("%q: %d problems, want %d", recipe, got, want)
		}
	}
}
