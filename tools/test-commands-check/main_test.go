package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// The test binary is the go shim when the gate under test copies it onto
// make's PATH: record and exit before any test runs.
func TestMain(m *testing.M) {
	if recorded, err := shim(); recorded {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func requireMake(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not on PATH")
	}
	return filepath.Join("..", "..")
}

// mutate writes a copy of the file at path with the first occurrence of old
// replaced by new (appended when old is empty) and returns the copy's path.
func mutate(t *testing.T, path, old, new string) string {
	t.Helper()
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(original) + new
	if old != "" {
		if !strings.Contains(body, old) {
			t.Fatalf("%s no longer contains %q", path, old)
		}
		body = strings.Replace(string(original), old, new, 1)
	}
	copied := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(copied, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return copied
}

// STR-V0-006: the repository declaration is what `make go-test` runs, and a
// one-flag mutation, an extra environment variable, a missing make target,
// a none language the gate runs or one it has no runners for each fail the
// gate with the diagnostic of the comparison the mutation targets.
func TestSTRV0006DeclarationMatchesMakefile(t *testing.T) {
	root := requireMake(t)
	declaration := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	var stdout, stderr bytes.Buffer
	if code := run(root, "Makefile", declaration, &stdout, &stderr); code != 0 {
		t.Fatalf("repository declaration: code %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "equals make go-test (GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./...); 1 declared make targets dry-run; no make gate step recipe runs a suite of [dotnet kotlin python ruby rust swift]") {
		t.Fatalf("stdout %q", stdout.String())
	}
	// Every mutation is a readable declaration (the reader is not what fails)
	// and names the diagnostic the comparison it targets must print.
	mutations := map[string][3]string{
		"flag value":           {`"-count=1"`, `"-count=2"`, "go.flags give"},
		"flag order":           {`"-p", "1", "-timeout"`, `"-timeout", "-p", "1"`, "go.flags give"},
		"dropped flag":         {`"-p", "1", `, ``, "go.flags give"},
		"extra env":            {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "local", "GOFLAGS": "-mod=mod"`, "go.env is"},
		"env value":            {`"GOTOOLCHAIN": "local"`, `"GOTOOLCHAIN": "go1.27"`, "go.env is"},
		"missing make target":  {`"languages": {`, `"languages": {"zig": ["make", "no-such-target"], `, "languages.zig names make no-such-target"},
		"none the gate runs":   {`"typescript": ["make", "host-adapter-test"]`, `"typescript": {"none": "claimed"}`, `languages.typescript is none, but make gate step host-adapter-test runs "node"`},
		"none without runners": {`"languages": {`, `"languages": {"zig": {"none": "claimed"}, `, "languages.zig is none, but this gate knows no zig runners"},
	}
	for name, mutation := range mutations {
		mutated := mutate(t, declaration, mutation[0], mutation[1])
		if declared := sessiontest.ReadDeclaredFile(mutated); declared.Unreadable != "" {
			t.Fatalf("%s: the mutation is unreadable (%s), so it would not reach the comparison", name, declared.Unreadable)
		}
		var out, errs bytes.Buffer
		if code := run(root, "Makefile", mutated, &out, &errs); code != 1 || !strings.Contains(errs.String(), "test-commands-check: "+mutation[2]) {
			t.Errorf("%s: code %d stderr %q, want %q", name, code, errs.String(), mutation[2])
		}
	}
	var out, errs bytes.Buffer
	if code := run(root, "Makefile", filepath.Join(t.TempDir(), "absent.json"), &out, &errs); code != 1 || !strings.Contains(errs.String(), sessiontest.DeclarationPath) {
		t.Errorf("absent declaration: code %d stderr %q", code, errs.String())
	}
}

// STR-V0-006: drift in make's effective environment or invocation that
// leaves the printed recipe identical still fails, because the gate observes
// what go receives: a target-specific export, a global export,
// .EXPORT_ALL_VARIABLES, an absolute-path go the shim never sees, a second
// go call and a recipe that fails after go are each a difference.
func TestSTRV0006MakefileDriftReachesGo(t *testing.T) {
	root := requireMake(t)
	declaration := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	makefile := filepath.Join(root, "Makefile")
	const command = "GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1"
	const recipe = "go-test:\n\t$(GO_TEST_COMMAND) ./...\n"
	mutations := map[string]struct{ old, new, diagnostic, detail string }{
		"target-specific export": {"", "\ngo-test: export GOFLAGS := -race\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [GOCACHE=/tmp/corvint-go-build-cache GOFLAGS=-race GOTOOLCHAIN=local]", ""},
		"global export":          {"", "\nexport GOFLAGS := -race\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [GOCACHE=/tmp/corvint-go-build-cache GOFLAGS=-race GOTOOLCHAIN=local]", ""},
		"export all variables":   {"", "\n.EXPORT_ALL_VARIABLES:\nEXTRA := 1\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [", " EXTRA=1 "},
		"absolute-path go":       {command, "GOTOOLCHAIN=local /usr/bin/true test $(GO_TEST_FLAGS) -count=1", "make go-test never ran go from PATH", ""},
		"two go calls":           {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./...\n\t$(GO_TEST_COMMAND) ./...\n", "make go-test ran go 2 times, want exactly once", ""},
		"make fails after go":    {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./... && false\n", "make -s go-test: exit status 2", ""},
		"changed flags":          {command, "GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1 -race", `go.flags give "test -p 1 -timeout 30m -count=1 ./...", make go-test runs go with "test -p 1 -timeout 30m -count=1 -race ./..."`, ""},
	}
	for name, mutation := range mutations {
		mutated := mutate(t, makefile, mutation.old, mutation.new)
		var out, errs bytes.Buffer
		if code := run(root, mutated, declaration, &out, &errs); code != 1 || !strings.Contains(errs.String(), "test-commands-check: "+mutation.diagnostic) || !strings.Contains(errs.String(), mutation.detail) {
			t.Errorf("%s: code %d stderr %q, want %q and %q", name, code, errs.String(), mutation.diagnostic, mutation.detail)
		}
	}
	// The unmodified Makefile copied elsewhere still passes: the mutations,
	// not the copy, are what fail.
	var out, errs bytes.Buffer
	if code := run(root, mutate(t, makefile, "", ""), declaration, &out, &errs); code != 0 {
		t.Errorf("copied Makefile: code %d stderr %q", code, errs.String())
	}
}

// The environment comparison ignores exactly the closed make-introduced set,
// requires every passed name to arrive unchanged and otherwise demands the
// declared words in sorted order.
func TestSTRV0006CompareEnv(t *testing.T) {
	passed := []string{"PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}
	declared := []string{"A=1", "B=2"}
	for name, c := range map[string]struct {
		observed []string
		problems int
	}{
		"exact":             {[]string{"B=2", "PATH=/shim:/usr/bin:/bin", "A=1", recordVariable + "=/records"}, 0},
		"make introduced":   {[]string{"A=1", "B=2", "MAKEFLAGS=s", "MFLAGS=-s", "MAKELEVEL=1", "PWD=/r", "SHLVL=1", "_=/shim/go", "MANPATH=/m", "__CF_USER_TEXT_ENCODING=0", "PATH=/shim:/usr/bin:/bin"}, 0},
		"extra name":        {[]string{"A=1", "B=2", "GOFLAGS=-race", "PATH=/shim:/usr/bin:/bin"}, 1},
		"missing name":      {[]string{"A=1", "PATH=/shim:/usr/bin:/bin"}, 1},
		"changed value":     {[]string{"A=1", "B=3", "PATH=/shim:/usr/bin:/bin"}, 1},
		"changed PATH":      {[]string{"A=1", "B=2", "PATH=/usr/local/bin:/shim:/usr/bin:/bin"}, 1},
		"lower-case make":   {[]string{"A=1", "B=2", "makeflags=s"}, 1},
		"entry without =":   {[]string{"A=1", "B=2", "C"}, 1},
		"unsorted declared": {[]string{"B=2", "A=1"}, 0},
	} {
		if got := len(compareEnv(declared, passed, c.observed)); got != c.problems {
			t.Errorf("%s: %d problems, want %d", name, got, c.problems)
		}
	}
}

func TestSTRV0006ShimRecordRoundTrip(t *testing.T) {
	data := []byte("2\x00test\x00./...\x00A=1\x00B=\x00")
	argv, env, err := parseRecord(data)
	if err != nil || strings.Join(argv, " ") != "test ./..." || strings.Join(env, " ") != "A=1 B=" {
		t.Fatalf("argv %q env %q err %v", argv, env, err)
	}
	for _, malformed := range []string{"", "x\x00", "3\x00a\x00", "1\x00a"} {
		if _, _, err := parseRecord([]byte(malformed)); err == nil {
			t.Errorf("%q parsed", malformed)
		}
	}
}
