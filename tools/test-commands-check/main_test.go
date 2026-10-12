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
	if !strings.Contains(stdout.String(), "equals make go-test (GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./... in the repository root, one plain command line, no make function in the recipe); 1 declared make targets dry-run; no make gate step recipe runs a suite of [dotnet kotlin python ruby rust swift]") {
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
// go call, a recipe that fails after go, a changed directory, a recipe that
// sets a make-introduced name itself, a Makefile that exports MAKEFLAGS or
// sets SHELL, a second go behind an absolute path (chained, on its own recipe
// line, or as an argument), a wrapper before go (a shell running a script, or
// nice), a make function expanded by the recipe (directly, through a variable
// or through a target-specific variable) and a quoted runner in a gate step
// are each a difference.
func TestSTRV0006MakefileDriftReachesGo(t *testing.T) {
	root := requireMake(t)
	declaration := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	makefile := filepath.Join(root, "Makefile")
	otherGo := filepath.Join(t.TempDir(), "go")
	if err := os.WriteFile(otherGo, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "wrapper")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	const command = "GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1"
	const recipe = "go-test:\n\t$(GO_TEST_COMMAND) ./...\n"
	mutations := map[string]struct{ old, new, diagnostic, detail string }{
		"target-specific export":  {"", "\ngo-test: export GOFLAGS := -race\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [GOCACHE=/tmp/corvint-go-build-cache GOFLAGS=-race GOTOOLCHAIN=local]", ""},
		"global export":           {"", "\nexport GOFLAGS := -race\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [GOCACHE=/tmp/corvint-go-build-cache GOFLAGS=-race GOTOOLCHAIN=local]", ""},
		"export all variables":    {"", "\n.EXPORT_ALL_VARIABLES:\nEXTRA := 1\n", "go.env is [GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local], make go-test runs go with [", " EXTRA=1 "},
		"absolute-path go":        {command, "GOTOOLCHAIN=local /usr/bin/true test $(GO_TEST_FLAGS) -count=1", "make go-test never ran go from PATH", ""},
		"two go calls":            {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./...\n\t$(GO_TEST_COMMAND) ./...\n", "make go-test ran go 2 times, want exactly once", ""},
		"make fails after go":     {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./... && false\n", "make -s go-test: exit status 2", ""},
		"changed flags":           {command, "GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) -count=1 -race", `go.flags give "test -p 1 -timeout 30m -count=1 ./...", make go-test runs go with "test -p 1 -timeout 30m -count=1 -race ./..."`, ""},
		"changed directory":       {recipe, "go-test:\n\tcd internal/sessiontest; unset OLDPWD; $(GO_TEST_COMMAND) ./...\n", "make go-test runs go in ", "/internal/sessiontest, want the repository root "},
		"recipe sets MAKEFLAGS":   {recipe, "go-test:\n\tMAKEFLAGS=--just-print $(GO_TEST_COMMAND) ./...\n", `make go-test runs go with MAKEFLAGS="--just-print", which make gives a plain recipe as "`, ""},
		"chained absolute go":     {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./... && " + otherGo + " test -race ./...\n", "make -s -n go-test prints \"GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./... && " + otherGo + " test -race ./...\": the go-test recipe may not use shell operators", ""},
		"second line absolute go": {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./...\n\t" + otherGo + " test -race ./...\n", "make -s -n go-test prints 2 lines, want one plain go command line", otherGo + " test -race ./..."},
		"absolute go word":        {recipe, "go-test:\n\t$(GO_TEST_COMMAND) ./... " + otherGo + "\n", `go.flags give "test -p 1 -timeout 30m -count=1 ./...", make go-test runs go with "test -p 1 -timeout 30m -count=1 ./... ` + otherGo + `"`, ""},
		"shell wrapper":           {command, "GOTOOLCHAIN=local /bin/sh " + wrapper + " go test $(GO_TEST_FLAGS) -count=1", "make -s -n go-test prints \"", `: want NAME=VALUE words then go, found "/bin/sh", a program the go shim on PATH does not observe`},
		"nice wrapper":            {command, "GOTOOLCHAIN=local nice go test $(GO_TEST_FLAGS) -count=1", "make -s -n go-test prints \"", `: want NAME=VALUE words then go, found "nice", a program the go shim on PATH does not observe`},
		"recipe make function":    {recipe, "go-test:\n\t$(shell true)$(GO_TEST_COMMAND) ./...\n", `the go-test recipe contains "$(shell true)$(GO_TEST_COMMAND) ./...": only $(NAME) references`, ""},
		"variable make function":  {"GO_TEST_FLAGS = -p 1 -timeout $(GO_TEST_TIMEOUT)", "GO_TEST_FLAGS = $(shell true)-p 1 -timeout $(GO_TEST_TIMEOUT)", `variable GO_TEST_FLAGS contains "$(shell true)-p 1 -timeout $(GO_TEST_TIMEOUT)": only $(NAME) references`, ""},
		"target make function":    {"", "\ngo-test: GO_TEST_FLAGS = $(shell true)-p 1 -timeout 30m\n", `go-test's own GO_TEST_FLAGS contains "$(shell true)-p 1 -timeout 30m": only $(NAME) references`, ""},
		"global MAKEFLAGS export": {"", "\nexport MAKEFLAGS := --keep-going\n", `make go-test runs go with MAKEFLAGS="sk", which make gives a plain recipe as "s": the Makefile or recipe sets it`, ""},
		"Makefile sets SHELL":     {"", "\nSHELL := /bin/sh\n", "the Makefile sets SHELL (makefile (from ", "the go-test recipe must run through make's default shell"},
		"target SHELL":            {"", "\ngo-test: SHELL := /usr/bin/nice /bin/sh\n", `the Makefile sets SHELL for go-test itself ("/usr/bin/nice /bin/sh")`, "the go-test recipe must run through make's default shell"},
		"pattern SHELLFLAGS":      {"", "\n%: .SHELLFLAGS := -ec\n", `the Makefile sets .SHELLFLAGS for the pattern %, which matches go-test ("-ec")`, "the go-test recipe must run through make's default shell"},
		"quoted gate runner":      {"", "\nGATE_STEPS += quoted-python\nquoted-python:\n\t\"python3\" -m unittest discover\n", `languages.python is none, but make gate step quoted-python runs "\"python3\""`, ""},
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

// The environment comparison sets aside exactly the closed make-introduced
// set, requires every passed name to arrive present and unchanged and
// otherwise demands the declared words in sorted order.
func TestSTRV0006CompareEnv(t *testing.T) {
	passed := []string{"PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}
	declared := []string{"A=1", "B=2"}
	for name, c := range map[string]struct {
		observed []string
		problems int
	}{
		"exact":             {[]string{"B=2", "PATH=/shim:/usr/bin:/bin", "A=1", recordVariable + "=/records"}, 0},
		"make introduced":   {[]string{"A=1", "B=2", "MAKEFLAGS=s", "MFLAGS=-s", "MAKELEVEL=1", "PWD=/r", "SHLVL=1", "_=/shim/go", "MANPATH=/m", "__CF_USER_TEXT_ENCODING=0", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 0},
		"extra name":        {[]string{"A=1", "B=2", "GOFLAGS=-race", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"missing name":      {[]string{"A=1", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"changed value":     {[]string{"A=1", "B=3", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"changed PATH":      {[]string{"A=1", "B=2", "PATH=/usr/local/bin:/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"dropped PATH":      {[]string{"A=1", "B=2", recordVariable + "=/records"}, 1},
		"dropped both":      {[]string{"A=1", "B=2"}, 2},
		"lower-case make":   {[]string{"A=1", "B=2", "makeflags=s", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"entry without =":   {[]string{"A=1", "B=2", "C", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 1},
		"unsorted declared": {[]string{"B=2", "A=1", "PATH=/shim:/usr/bin:/bin", recordVariable + "=/records"}, 0},
	} {
		if got := len(compareEnv(declared, passed, c.observed)); got != c.problems {
			t.Errorf("%s: %d problems, want %d", name, got, c.problems)
		}
	}
}

// The make-introduced names are excluded from the declaration only while
// they hold the values make gives a plain recipe.
func TestSTRV0006CompareIntroduced(t *testing.T) {
	control := map[string]string{"MAKEFLAGS": "s", "MFLAGS": "-s", "MAKELEVEL": "1", "PWD": "/r"}
	for name, c := range map[string]struct {
		observed map[string]string
		problems int
	}{
		"same":            {map[string]string{"MAKEFLAGS": "s", "MFLAGS": "-s", "MAKELEVEL": "1", "PWD": "/r"}, 0},
		"recipe override": {map[string]string{"MAKEFLAGS": "--just-print", "MFLAGS": "-s", "MAKELEVEL": "1", "PWD": "/r"}, 1},
		"recipe adds":     {map[string]string{"MAKEFLAGS": "s", "MFLAGS": "-s", "MAKELEVEL": "1", "PWD": "/r", "MAKE_RESTARTS": "1"}, 1},
		"recipe unsets":   {map[string]string{"MAKEFLAGS": "s", "MFLAGS": "-s", "MAKELEVEL": "1"}, 1},
	} {
		if got := len(compareIntroduced(control, c.observed)); got != c.problems {
			t.Errorf("%s: %d problems, want %d", name, got, c.problems)
		}
	}
	if got := introducedValues([]string{"A=1", "MAKEFLAGS=s", "PATH=/p", "_=/shim/go"}); len(got) != 2 || got["MAKEFLAGS"] != "s" || got["_"] != "/shim/go" {
		t.Errorf("introducedValues %v", got)
	}
}

// The database parse keeps the recipe as written, global, define and
// target-specific definitions and each origin; the plain-recipe check follows
// $(NAME) and ${NAME} references through recursive variables, stops at simple
// ones and refuses functions, automatic variables, $$ and a non-default SHELL.
func TestSTRV0006PlainRecipe(t *testing.T) {
	printed := strings.Join([]string{
		"# default",
		"SHELL := /bin/sh",
		"# makefile (from `Makefile', line 1)",
		"export CMD = ${ENV} go test $(FLAGS)",
		"# makefile (from `Makefile', line 2)",
		"FLAGS := -count=1",
		"# makefile (from `Makefile', line 3)",
		"define ENV",
		"GOTOOLCHAIN=local",
		"endef",
		"# makefile (from `Makefile', line 4)",
		"LOOP = $(LOOP)",
		"# makefile (from `Makefile', line 5)",
		"FN = $(shell true)",
		"# makefile (from `Makefile', line 6)",
		"LATE := $$HOME",
		"",
		"go-test:",
		"#  Phony target (prerequisite of .PHONY).",
		"# automatic",
		"# @ := go-test",
		"# makefile (from `Makefile', line 7)",
		"# OWN = $(FLAGS)",
		"#  commands to execute (from `Makefile', line 8):",
		"\t$(CMD) ./...",
		"\t",
		"",
		"# Not a target:",
		"other:",
		"#  commands to execute (from `Makefile', line 9):",
		"\t$(FN)",
		"",
	}, "\n")
	db := parseDatabase(printed)
	if strings.Join(db.recipe, "|") != "$(CMD) ./..." || db.origin["SHELL"] != "default" || !db.global["CMD"].recursive || db.global["FLAGS"].recursive || db.global["ENV"].value != "GOTOOLCHAIN=local" || db.target["OWN"].value != "$(FLAGS)" || len(db.target) != 1 {
		t.Fatalf("parsed %+v", db)
	}
	if problems := plainRecipe(db); len(problems) != 0 {
		t.Errorf("plain recipe: %v", problems)
	}
	for name, c := range map[string]struct {
		recipe   string
		problems int
	}{
		"own variable":   {"$(OWN) $(CMD)", 0},
		"loop":           {"$(LOOP)", 0},
		"undefined":      {"$(NOPE) go", 0},
		"function":       {"$(FN)", 1},
		"inline":         {"$(shell true) $(CMD)", 1},
		"automatic":      {"$(CMD) $@", 1},
		"escaped dollar": {"$(CMD) $$HOME", 1},
		"simple with $":  {"$(CMD) $(LATE)", 1},
		"substitution":   {"$(CMD:go=nice go)", 1},
		"nested":         {"$($(CMD))", 1},
	} {
		db.recipe = []string{c.recipe}
		if got := len(plainRecipe(db)); got != c.problems {
			t.Errorf("%s: %d problems, want %d: %v", name, got, c.problems, plainRecipe(db))
		}
	}
	db.origin["SHELL"] = "makefile (from `Makefile', line 1)"
	db.recipe = []string{"$(CMD)"}
	if got := len(plainRecipe(db)); got != 1 {
		t.Errorf("non-default SHELL: %d problems, want 1", got)
	}
	if _, err := readDatabase(requireMake(t), "Makefile"); err != nil {
		t.Errorf("repository database: %v", err)
	}
}

// TestSTRV0006DatabaseForms parses the database in both forms the gate meets:
// GNU make 3.81 (macOS) prints `#  commands to execute` and target-specific
// variables inside the target block; GNU make 4.x (Ubuntu CI) prints `#  recipe
// to execute`, `go-test: NAME := value` lines before the target, origins
// quoted with ASCII quotes, and `+=` and `$(subst ,,)` forms (its print_file,
// print_variable and print_commands in src/file.c, src/variable.c and
// src/commands.c of 4.3). Pattern-specific values appear in their own section.
func TestSTRV0006DatabaseForms(t *testing.T) {
	forms := map[string]string{
		"make 3.81": strings.Join([]string{
			"# default",
			"SHELL := /bin/sh",
			"# makefile (from `Makefile', line 1)",
			"GO_TEST_FLAGS := -count=1",
			"",
			"# Pattern-specific Variable Values",
			"",
			"% :",
			"# makefile (from `Makefile', line 2)",
			"# .SHELLFLAGS := -ec",
			"",
			"%.o :",
			"# makefile (from `Makefile', line 3)",
			"# SHELL := /bin/bash",
			"",
			"# 2 pattern-specific variable values",
			"",
			"go-test:",
			"#  Phony target (prerequisite of .PHONY).",
			"#  Command-line target.",
			"# automatic",
			"# @ := go-test",
			"# makefile (from `Makefile', line 4)",
			"# SHELL := /usr/bin/nice /bin/sh",
			"# variable set hash-table stats:",
			"# Load=2/32=6%, Rehash=0, Collisions=0/2=0%",
			"#  commands to execute (from `Makefile', line 5):",
			"\tGOTOOLCHAIN=local go test $(GO_TEST_FLAGS) ./...",
			"\t",
			"",
		}, "\n"),
		"make 4.3": strings.Join([]string{
			"# default",
			"SHELL = /bin/sh",
			"# makefile (from 'Makefile', line 1)",
			"GO_TEST_FLAGS := -count=1",
			"# makefile (from 'Makefile', line 6)",
			"EXTRA += -p 1",
			"# makefile (from 'Makefile', line 7)",
			"SPACE := $(subst ,, )",
			"# makefile (from 'Makefile', line 8)",
			"LATE := $$$$HOME",
			"",
			"# Pattern-specific Variable Values",
			"",
			"% :",
			"# makefile (from 'Makefile', line 2)",
			"# .SHELLFLAGS := -ec",
			"",
			"%.o :",
			"# makefile (from 'Makefile', line 3)",
			"# SHELL := /bin/bash",
			"",
			"# 2 pattern-specific variable values",
			"",
			"# makefile (from 'Makefile', line 4)",
			"go-test: SHELL := /usr/bin/nice /bin/sh",
			"go-test:",
			"#  Phony target (prerequisite of .PHONY).",
			"#  Command line target.",
			"# automatic",
			"# @ := go-test",
			"# makefile (from 'Makefile', line 4)",
			"# SHELL := /usr/bin/nice /bin/sh",
			"# variable set hash-table stats:",
			"# Load=2/32=6%, Rehash=0, Collisions=0/2=0%",
			"#  recipe to execute (from 'Makefile', line 5):",
			"\tGOTOOLCHAIN=local go test $(GO_TEST_FLAGS) ./...",
			"",
		}, "\n"),
	}
	for form, printed := range forms {
		db := parseDatabase(printed)
		if strings.Join(db.recipe, "|") != "GOTOOLCHAIN=local go test $(GO_TEST_FLAGS) ./..." {
			t.Errorf("%s: recipe %q", form, db.recipe)
		}
		if db.origin["SHELL"] != "default" || db.origin["GO_TEST_FLAGS"] == "" || db.global["GO_TEST_FLAGS"].recursive {
			t.Errorf("%s: globals %+v origins %+v", form, db.global, db.origin)
		}
		if db.target["SHELL"].value != "/usr/bin/nice /bin/sh" || len(db.target) != 1 {
			t.Errorf("%s: target-specific %+v", form, db.target)
		}
		if def := db.pattern[".SHELLFLAGS"]; def.value != "-ec" || def.target != "%" || len(db.pattern) != 1 {
			t.Errorf("%s: pattern-specific %+v", form, db.pattern)
		}
		problems := plainRecipe(db)
		if len(problems) != 2 || !strings.Contains(problems[0], "SHELL for go-test itself") || !strings.Contains(problems[1], ".SHELLFLAGS for the pattern %, which matches go-test") {
			t.Errorf("%s: problems %v", form, problems)
		}
	}
	db := parseDatabase(forms["make 4.3"])
	if db.global["EXTRA"] != (definition{value: "-p 1", recursive: true}) || db.global["SPACE"].value != " " || db.global["LATE"].value != "$$HOME" {
		t.Errorf("4.3 definitions %+v", db.global)
	}
	for pattern, want := range map[string]bool{"%": true, "go-%": true, "%-test": true, "go-test": true, "%.o": false, "go-test%": false, "other": false, "go-%-test": false} {
		if got := matchesTarget(pattern, goTarget); got != want {
			t.Errorf("matchesTarget(%q) = %v, want %v", pattern, got, want)
		}
	}
	for line, want := range map[string]bool{"NAME = v": true, "NAME := v": true, "NAME += v": true, "export NAME = v": true, "NAME ?= v": false, "NAME": false, "# NAME = v": false, "go-test: SHELL := x": false, "": false} {
		if _, _, got := parseDefinition(line); got != want {
			t.Errorf("parseDefinition(%q) ok = %v, want %v", line, got, want)
		}
	}
}

func TestSTRV0006ShimRecordRoundTrip(t *testing.T) {
	data := []byte("2\x00/r\x00test\x00./...\x00A=1\x00B=\x00")
	seen, err := parseRecord(data)
	if err != nil || seen.cwd != "/r" || strings.Join(seen.argv, " ") != "test ./..." || strings.Join(seen.env, " ") != "A=1 B=" {
		t.Fatalf("record %+v err %v", seen, err)
	}
	for _, malformed := range []string{"", "x\x00", "3\x00/r\x00a\x00", "1\x00/r\x00a", "1\x00a\x00"} {
		if _, err := parseRecord([]byte(malformed)); err == nil {
			t.Errorf("%q parsed", malformed)
		}
	}
}
