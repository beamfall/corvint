package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

const declaration = `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local", "GOCACHE": "/tmp/c"}, "flags": ["-p", "1", "-timeout", "30m", "-count=1"]}, "languages": {}}`

// fakeGo puts a `go` first on PATH that records its arguments, directory and
// the two declared variables in record, then exits with the status in
// GO_FAKE_EXIT (0 when unset).
func fakeGo(t *testing.T) string {
	t.Helper()
	bin, record := t.TempDir(), filepath.Join(t.TempDir(), "record")
	script := "#!/bin/sh\n{ printf '%s\\n' \"$@\"; pwd; printf 'GOCACHE=%s\\nGOTOOLCHAIN=%s\\nGOFLAGS=%s\\n' \"$GOCACHE\" \"$GOTOOLCHAIN\" \"$GOFLAGS\"; } > " + record + "\nexit ${GO_FAKE_EXIT:-0}\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

// root makes a repository root holding go.mod and the declaration and
// enters it, as the Makefile's go-test recipe does.
func root(t *testing.T, declared string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n\ngo 1.27.2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if declared != "" {
		name := filepath.Join(dir, filepath.FromSlash(sessiontest.DeclarationPath))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(declared), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

// STR-V0-006: the runner hands go the declared flags and `./...`, the
// declared environment over the inherited one, the repository root as its
// directory, and go's exit status back.
func TestSTRV0006RunnerRunsTheDeclaration(t *testing.T) {
	record := fakeGo(t)
	dir := root(t, declaration)
	t.Setenv("GOCACHE", "/elsewhere")
	t.Setenv("GOFLAGS", "-race")
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr.String())
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	want := "test\n-p\n1\n-timeout\n30m\n-count=1\n./...\n" + dir + "\nGOCACHE=/tmp/c\nGOTOOLCHAIN=local\nGOFLAGS=-race\n"
	if string(got) != want {
		t.Errorf("fake go recorded\n%s\nwant\n%s", got, want)
	}
	t.Setenv("GO_FAKE_EXIT", "3")
	if code := run(nil, &stdout, &stderr); code != 3 {
		t.Errorf("go exit 3 gave %d, stderr %q", code, stderr.String())
	}
}

func TestSTRV0006RunnerRefusals(t *testing.T) {
	fakeGo(t)
	for name, c := range map[string]struct {
		declared string
		args     []string
		want     string
	}{
		"arguments":   {declaration, []string{"./internal/..."}, "takes no arguments"},
		"unreadable":  {strings.Replace(declaration, "/0", "/1", 1), nil, sessiontest.DeclarationPath + ": profile is"},
		"absent":      {"", nil, sessiontest.DeclarationPath + ": "},
		"not at root": {declaration, nil, "holds no go.mod"},
	} {
		dir := root(t, c.declared)
		if name == "not at root" {
			t.Chdir(filepath.Dir(dir))
		}
		var stdout, stderr bytes.Buffer
		if code := run(c.args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("%s: code %d, stderr %q, want 2 and %q", name, code, stderr.String(), c.want)
		}
	}
}

func TestSTRV0006Invocation(t *testing.T) {
	declared := sessiontest.Declared{GoEnv: []string{"GOCACHE=/tmp/c", "GOTOOLCHAIN=local"}, GoFlags: []string{"-count=1"}}
	argv, env := invocation(declared, []string{"PATH=/usr/bin", "GOTOOLCHAIN=auto", "HOME=/h", "GOCACHE=/x"})
	if strings.Join(argv, " ") != "go test -count=1 ./..." || strings.Join(env, " ") != "PATH=/usr/bin HOME=/h GOCACHE=/tmp/c GOTOOLCHAIN=local" {
		t.Errorf("argv %q env %q", argv, env)
	}
}

// STR-V0-174 (V1-3155): an inherited CORVINT_GOCACHE replaces the declared
// GOCACHE, as the Makefile's `CORVINT_GOCACHE ?=` does; an empty one does not.
func TestSTRV0174InvocationHonoursCorvintGocache(t *testing.T) {
	declared := sessiontest.Declared{GoEnv: []string{"GOCACHE=/tmp/c", "GOTOOLCHAIN=local"}, GoFlags: []string{"-count=1"}}
	_, env := invocation(declared, []string{"PATH=/usr/bin", "CORVINT_GOCACHE=/lane/cache", "GOCACHE=/x"})
	if strings.Join(env, " ") != "PATH=/usr/bin CORVINT_GOCACHE=/lane/cache GOCACHE=/lane/cache GOTOOLCHAIN=local" {
		t.Errorf("env %q", env)
	}
	_, env = invocation(declared, []string{"CORVINT_GOCACHE="})
	if strings.Join(env, " ") != "CORVINT_GOCACHE= GOCACHE=/tmp/c GOTOOLCHAIN=local" {
		t.Errorf("empty CORVINT_GOCACHE: env %q", env)
	}
}
