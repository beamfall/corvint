package main

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

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

// STR-V0-006: the repository Makefile's go-test is the fixed runner line and
// the declaration's make targets exist; a missing make target, a none
// language the gate runs or one it has no runners for each fail the gate
// with the diagnostic of the comparison the mutation targets.
func TestSTRV0006DeclarationMatchesMakefile(t *testing.T) {
	root := requireMake(t)
	declaration := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	var stdout, stderr bytes.Buffer
	if code := run(root, "Makefile", declaration, &stdout, &stderr); code != 0 {
		t.Fatalf("repository declaration: code %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Makefile go-test is exactly `go run ./tools/test-commands-run`, which runs .github/cishards/test-commands.json (GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -p 1 -timeout 30m -count=1 ./... in the repository root); ledger/go-test runs the declared go.env and go.flags through gate-ledger; no SHELL, .SHELLFLAGS, MAKEFLAGS or go-test variable in the Makefile; 2 declared make targets dry-run; no make gate step recipe runs a suite of [dotnet kotlin python ruby rust swift]") {
		t.Fatalf("stdout %q", stdout.String())
	}
	// Every mutation is a readable declaration (the reader is not what fails)
	// and names the diagnostic the comparison it targets must print.
	mutations := map[string][3]string{
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

// STR-V0-006: each Makefile text rule fires on a one-line mutation of the
// repository Makefile, and a line that cannot reach go-test passes.
func TestSTRV0006MakefileTextRules(t *testing.T) {
	root := requireMake(t)
	const recipe = "go-test:\n\tgo run ./tools/test-commands-run\n"
	refusals := map[string][3]string{
		"changed recipe":          {recipe, "go-test:\n\tGOFLAGS=-race go run ./tools/test-commands-run\n", "recipe is \"GOFLAGS=-race go run ./tools/test-commands-run\", want exactly"},
		"continued recipe":        {recipe, "go-test:\n\tgo run ./tools/test-commands-run \\\n\t\t-x\n", "recipe is \"go run ./tools/test-commands-run -x\""},
		"second recipe line":      {recipe, recipe + "\techo done\n", "has a second recipe line \"echo done\""},
		"conditional recipe":      {recipe, recipe + "ifneq ($(CI),)\n\tgo test -race ./...\nendif\n", "has a second recipe line \"go test -race ./...\""},
		".IGNORE":                 {"", ".IGNORE: go-test\n", "`.IGNORE` makes make ignore a failing recipe"},
		"global .IGNORE":          {"", ".IGNORE:\n", "`.IGNORE` makes make ignore a failing recipe"},
		"recipe after a comment":  {recipe, recipe + "# still the same recipe\n\techo done\n", "has a second recipe line \"echo done\""},
		"no recipe":               {recipe, "go-test:\n", "`go-test:` has no recipe line"},
		"prerequisite":            {recipe, "go-test: go-version\n\tgo run ./tools/test-commands-run\n", "must be exactly `go-test:` with no prerequisites"},
		"double colon":            {recipe, "go-test::\n\tgo run ./tools/test-commands-run\n", "must be exactly `go-test:`"},
		"second rule":             {"", recipe, "has 2 `go-test:` rules"},
		"SHELL":                   {"", "SHELL = /bin/bash\n", "assigns SHELL"},
		".SHELLFLAGS":             {"", ".SHELLFLAGS := -ec\n", "assigns .SHELLFLAGS"},
		"MAKEFLAGS":               {"", "MAKEFLAGS += -e\n", "assigns MAKEFLAGS"},
		"exported SHELL":          {"", "export SHELL = /bin/bash\n", "assigns SHELL"},
		"export MAKEFLAGS":        {"", "export MAKEFLAGS\n", "`export MAKEFLAGS`"},
		"bare export":             {"", "export\n", "`export` alone"},
		".EXPORT_ALL_VARIABLES":   {"", ".EXPORT_ALL_VARIABLES:\n", "`.EXPORT_ALL_VARIABLES`"},
		".RECIPEPREFIX":           {"", ".RECIPEPREFIX = >\n", "assigns .RECIPEPREFIX"},
		"target variable":         {"", "go-test: GOFLAGS = -race\n", "defines a variable for go-test, which names go-test"},
		"target export":           {"", "go-test: export GOFLAGS := -race\n", "defines a variable for go-test"},
		"target SHELL":            {"", "go-test: SHELL = /bin/bash\n", "sets SHELL for a target"},
		"pattern variable":        {"", "go-%: GOFLAGS = -race\n", "defines a variable for go-%, which is a pattern go-test matches"},
		"match-anything variable": {"", "%: GOFLAGS = -race\n", "defines a variable for %, which is a pattern go-test matches"},
		"pattern .SHELLFLAGS":     {"", "%.o: .SHELLFLAGS = -ec\n", "sets .SHELLFLAGS for a target"},
		"expanded target":         {"", "$(GATE_STEPS): GOFLAGS = -race\n", "defines a variable for $(GATE_STEPS), which is expanded by make"},
		"pattern recipe":          {"", "go-%:\n\techo $@\n", "recipe line \"echo $@\" belongs to the rule \"go-%:\", whose target go-% is a pattern go-test matches"},
		"expanded recipe":         {"", "$(GATE_STEPS):\n\techo $@\n", "belongs to the rule \"$(GATE_STEPS):\", whose target $(GATE_STEPS) is expanded by make"},
		"inline recipe":           {"", "go-%: ; echo $@\n", "has a recipe, and its target go-% is a pattern go-test matches"},
		"eval":                    {"", "$(eval go-test: GOFLAGS = -race)\n", "$(eval ...) adds makefile text"},
		"define":                  {"", "define EXTRA\ngo-test: GOFLAGS = -race\nendef\n", "`define` holds makefile text"},
		"computed include":        {"", "include $(wildcard *.mk)\n", "include $(wildcard names a file make computes"},
		"missing include":         {"", "include no-such.mk\n", "include no-such.mk: "},
		"GATE_STEPS twice":        {"", "GATE_STEPS += extra\n", "GATE_STEPS is assigned 2 times"},
		"GATE_STEPS expanded":     {"GATE_STEPS = \\\n  host-adapter-test", "GATE_STEPS = \\\n  $(EXTRA) host-adapter-test", "is expanded by make; the none-language check reads only a literal step list"},
	}
	for name, mutation := range refusals {
		problems, _ := checkMakefile(root, mutate(t, filepath.Join(root, "Makefile"), mutation[0], mutation[1]))
		if !strings.Contains(strings.Join(problems, "\n"), mutation[2]) {
			t.Errorf("%s: problems %q, want %q", name, problems, mutation[2])
		}
	}
	passes := map[string]string{
		"unrelated pattern variable": "%.o: CFLAGS = -O2\n",
		"unrelated rule":             "go-test-report: go-test\n\techo done\n",
		"go-test as prerequisite":    "all: go-test\n",
		"conditional around nothing": "ifeq ($(X),y)\nelse\nendif\n",
		"missing optional include":   "-include no-such.mk\n",
	}
	for name, appended := range passes {
		problems, steps := checkMakefile(root, mutate(t, filepath.Join(root, "Makefile"), "", appended))
		if len(problems) != 0 || len(steps) == 0 {
			t.Errorf("%s: problems %q, steps %d", name, problems, len(steps))
		}
	}
}

// STR-V0-006: `make gate` runs Go tests through ledger/go-test, so its fixed
// recipe and the single literal assignments it expands must give the
// declared go.env and go.flags; a plausible edit to any of them fails, and
// removing the rule (make gate then reaches go-test itself) passes.
func TestSTRV0006LedgerMatchesDeclaration(t *testing.T) {
	root := requireMake(t)
	declaration := filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath))
	makefile := filepath.Join(root, "Makefile")
	const recipe = "ledger/go-test:\n\t@$(GATE_LEDGER) go-test -- go test $(GO_TEST_FLAGS)\n"
	refusals := map[string][3]string{
		"flags":              {"GO_TEST_FLAGS = -p 1 -timeout $(GO_TEST_TIMEOUT)", "GO_TEST_FLAGS = -p 4 -timeout $(GO_TEST_TIMEOUT)", "runs go test [-p 4 -timeout 30m] (GO_TEST_FLAGS)"},
		"timeout":            {"GO_TEST_TIMEOUT ?= 30m", "GO_TEST_TIMEOUT ?= 10m", "runs go test [-p 1 -timeout 10m] (GO_TEST_FLAGS)"},
		"race":               {"GO_TEST_FLAGS = -p 1", "GO_TEST_FLAGS = -race -p 1", "runs go test [-race -p 1 -timeout 30m]"},
		"cache":              {"CORVINT_GOCACHE ?= /tmp/corvint-go-build-cache", "CORVINT_GOCACHE ?= /tmp/other", "runs go with [GOCACHE=/tmp/other GOTOOLCHAIN=local] (GATE_LEDGER)"},
		"ledger env":         {"GATE_LEDGER = GOCACHE", "GATE_LEDGER = GOFLAGS=-race GOCACHE", "runs go with [GOCACHE=/tmp/corvint-go-build-cache GOFLAGS=-race GOTOOLCHAIN=local]"},
		"appended flags":     {"", "GO_TEST_FLAGS += -race\n", "GO_TEST_FLAGS is assigned 2 times"},
		"unknown variable":   {"GO_TEST_FLAGS = -p 1", "GO_TEST_FLAGS = $(EXTRA) -p 1", "GO_TEST_FLAGS expands"},
		"recipe":             {recipe, "ledger/go-test:\n\t@$(GATE_LEDGER) go-test -- go test -race $(GO_TEST_FLAGS)\n", "`ledger/go-test:` recipe is"},
		"second recipe":      {recipe, recipe + "\techo done\n", "`ledger/go-test:` has a second recipe line"},
		"target flags":       {"", "ledger/go-test: GO_TEST_FLAGS = -race\n", "sets GO_TEST_FLAGS for a target"},
		"ledger variable":    {"", "ledger/go-test: export GOFLAGS = -race\n", "defines a variable for ledger/go-test, which could reach `ledger/go-test:`"},
		"ledger pattern var": {"", "ledger/%: export GOFLAGS = -race\n", "defines a variable for ledger/%"},
	}
	for name, mutation := range refusals {
		var out, errs bytes.Buffer
		if code := run(root, mutate(t, makefile, mutation[0], mutation[1]), declaration, &out, &errs); code != 1 || !strings.Contains(errs.String(), mutation[2]) {
			t.Errorf("%s: code %d stderr %q, want %q", name, code, errs.String(), mutation[2])
		}
	}
	var out, errs bytes.Buffer
	if code := run(root, mutate(t, makefile, recipe, ""), declaration, &out, &errs); code != 0 || !strings.Contains(out.String(), "no ledger/go-test rule, so make gate reaches go-test") {
		t.Errorf("no ledger rule: code %d stdout %q stderr %q", code, out.String(), errs.String())
	}
}

// STR-V0-006: an included file is read under the same rules, and the
// GOC-V0-010 prerequisite-only line for a `$` target list passes.
func TestSTRV0006IncludedFile(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Makefile", "GATE_STEPS = go-test\ninclude rules.mk\n$(GATE_STEPS): | clear\ngo-test:\n\tgo run ./tools/test-commands-run\n")
	write("rules.mk", "include deeper.mk\n")
	write("deeper.mk", "all: go-test\n")
	if problems, steps := checkMakefile(root, "Makefile"); len(problems) != 0 || strings.Join(steps, " ") != "go-test" {
		t.Fatalf("clean include: problems %q, steps %q", problems, steps)
	}
	write("deeper.mk", "%: SHELL = /bin/bash\n")
	problems, _ := checkMakefile(root, "Makefile")
	if joined := strings.Join(problems, "\n"); !strings.Contains(joined, "deeper.mk:1: \"%: SHELL = /bin/bash\" sets SHELL for a target") || !strings.Contains(joined, "defines a variable for %, which is a pattern go-test matches") {
		t.Errorf("included SHELL: problems %q", problems)
	}
}

func TestSTRV0006Classify(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"A := $(or $(B),c)", "assignment|A |:=| $(or $(B),c)"},
		{"A = b:c", "assignment|A |=| b:c"},
		{"A != git describe", "assignment|A |!=| git describe"},
		{"A ::= b", "assignment|A |::=| b"},
		{"a: b c", "rule|a|:| b c"},
		{"a:: b", "rule|a|::| b"},
		{"$(X) $(Y): | z", "rule|$(X) $(Y)|:| | z"},
		{"a: X += y", "rule|a|:| X += y"},
		{"$(info hi)", "other|$(info hi)||"},
	} {
		kind, left, op, right := classify(c.text)
		got := []string{"other", "assignment", "rule"}[kind] + "|" + left + "|" + op + "|" + right
		if got != c.want {
			t.Errorf("classify(%q) = %q, want %q", c.text, got, c.want)
		}
	}
	for word, want := range map[string]string{"go-test": "names go-test", "go-%": "is a pattern go-test matches", "%": "is a pattern go-test matches", "%-test": "is a pattern go-test matches", "go-test%": "", "%.o": "", "ledger/go-test": "", "$(X)": "is expanded by make, so it could name go-test"} {
		if got := matchReason(word); got != want {
			t.Errorf("matchReason(%q) = %q, want %q", word, got, want)
		}
	}
}

// STR-V0-004: shell declares `make shell-test`, whose prerequisites are
// exactly the GATE_STEPS steps whose recipe runs a script/*_test.sh and which
// adds no recipe of its own, so the declared shell full command runs the
// shell suites make gate runs and no others (V1-1373).
func TestSTRV0004ShellTestIsTheGateShellSteps(t *testing.T) {
	root := requireMake(t)
	declared := sessiontest.ReadDeclaredFile(filepath.Join(root, filepath.FromSlash(sessiontest.DeclarationPath)))
	if entry, ok := declared.Command("shell"); !ok || strings.Join(entry.Argv, " ") != "make shell-test" {
		t.Fatalf("languages.shell is %+v (declared %v), want [make shell-test]", entry, ok)
	}
	text, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	// Logical lines, as make reads them: GATE_STEPS and the shell-test
	// prerequisites are one entry per backslash-continued line (STR-V0-920).
	var lines []string
	for _, line := range logicalLines("Makefile", text) {
		if !strings.HasPrefix(line.text, "\t") {
			line.text, _, _ = strings.Cut(line.text, "#")
		}
		lines = append(lines, line.text)
	}
	rules := map[string][]int{}
	var gateSteps []string
	for index, line := range lines {
		if value, ok := strings.CutPrefix(line, "GATE_STEPS = "); ok {
			gateSteps = strings.Fields(value)
		}
		if target, _, ok := strings.Cut(line, ":"); ok && line != "" && !strings.ContainsAny(line[:1], "\t #") && !strings.ContainsAny(target, " =$%") {
			rules[target] = append(rules[target], index)
		}
	}
	if len(gateSteps) == 0 {
		t.Fatal("no literal GATE_STEPS line")
	}
	recipe := func(index int) []string {
		var recipeLines []string
		for _, line := range lines[index+1:] {
			if !strings.HasPrefix(line, "\t") {
				break
			}
			recipeLines = append(recipeLines, line)
		}
		return recipeLines
	}
	// The recipe must start the script itself (after make's @, + or -
	// prefixes, or through bash for a script without the executable bit,
	// ASD-V0-942), so a recipe that only names it, such as `@echo
	// script/x_test.sh`, does not count as running it.
	shellTest := regexp.MustCompile(`^\t[@+-]*(?:bash )?script/[^\s/]+_test\.sh(\s|$)`)
	var want []string
	for _, step := range gateSteps {
		if len(rules[step]) != 1 {
			t.Fatalf("gate step %s has %d rules, want 1", step, len(rules[step]))
		}
		for _, line := range recipe(rules[step][0]) {
			if shellTest.MatchString(line) {
				want = append(want, step)
				break
			}
		}
	}
	if len(rules["shell-test"]) != 1 {
		t.Fatalf("Makefile has %d shell-test rules, want 1", len(rules["shell-test"]))
	}
	index := rules["shell-test"][0]
	if extra := recipe(index); len(extra) != 0 {
		t.Errorf("shell-test has recipe lines %q, want only prerequisites", extra)
	}
	_, prerequisites, _ := strings.Cut(lines[index], ":")
	got := strings.Fields(prerequisites)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("shell-test prerequisites %q, want the GATE_STEPS steps that run a script/*_test.sh %q (registration steps: the SHELL-TEST-REGISTRATION note under docs)", got, want)
	}
}

// STR-V0-019: every go test a gate step runs carries -timeout; the repository
// passes, an unbounded host-adapter-test is refused, and -timeout= is accepted.
func TestSTRV0019GateGoTestHasTimeout(t *testing.T) {
	root := requireMake(t)
	_, steps := checkMakefile(root, "Makefile")
	if got := checkGoTestTimeouts(root, "Makefile", steps); len(got) != 0 {
		t.Fatalf("repository Makefile: %q", got)
	}
	bad := mutate(t, filepath.Join(root, "Makefile"), "go test -count=1 -timeout $(GO_TEST_TIMEOUT) ./cmd/corvint", "go test -count=1 ./cmd/corvint")
	got := checkGoTestTimeouts(filepath.Dir(bad), filepath.Base(bad), steps)
	if len(got) != 1 || !strings.Contains(got[0], "without -timeout") {
		t.Errorf("unbounded host-adapter-test: %q, want one refusal", got)
	}
	eq := mutate(t, filepath.Join(root, "Makefile"), "-timeout $(GO_TEST_TIMEOUT) ./cmd/corvint", "-timeout=30m ./cmd/corvint")
	if got := checkGoTestTimeouts(filepath.Dir(eq), filepath.Base(eq), steps); len(got) != 0 {
		t.Errorf("-timeout=30m: %q, want pass", got)
	}
}

// unstartedScriptTests returns, sorted, each script/*_test.sh under root that
// no recipe line of the Makefile at makefile starts (ASD-V0-942), read as
// TestSTRV0004ShellTestIsTheGateShellSteps reads a recipe.
func unstartedScriptTests(t *testing.T, root, makefile string) []string {
	t.Helper()
	tests, err := filepath.Glob(filepath.Join(root, "script", "*_test.sh"))
	if err != nil || len(tests) == 0 {
		t.Fatalf("no script/*_test.sh under %s (%v)", root, err)
	}
	text, err := os.ReadFile(makefile)
	if err != nil {
		t.Fatal(err)
	}
	started := map[string]bool{}
	recipe := regexp.MustCompile(`(?m)^\t[@+-]*(?:bash )?script/([^\s/]+_test\.sh)(\s|$)`)
	for _, match := range recipe.FindAllStringSubmatch(string(text), -1) {
		started[match[1]] = true
	}
	var missing []string
	for _, test := range tests {
		if name := filepath.Base(test); !started[name] {
			missing = append(missing, "script/"+name)
		}
	}
	slices.Sort(missing)
	return missing
}

// ASD-V0-942 (V1-2393): every script/*_test.sh is started by a Makefile
// recipe; removing one target's recipe names its script.
func TestASDV0942EveryScriptTestHasAMakeTarget(t *testing.T) {
	root := requireMake(t)
	if missing := unstartedScriptTests(t, root, filepath.Join(root, "Makefile")); len(missing) != 0 {
		t.Fatalf("script tests no Makefile recipe starts: %q; add a <name>-test target (the SHELL-TEST-REGISTRATION note under docs)", missing)
	}
	bad := mutate(t, filepath.Join(root, "Makefile"), "\t@script/agent-preflight_test.sh\n", "\t@true\n")
	if got := unstartedScriptTests(t, root, bad); !slices.Equal(got, []string{"script/agent-preflight_test.sh"}) {
		t.Errorf("a removed recipe: %q, want [script/agent-preflight_test.sh]", got)
	}
}

// nonExecutableScriptTests lists the script/*_test.sh files the Git index at
// root records without the executable bit (mode other than 100755).
func nonExecutableScriptTests(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-s", "--", "script/*_test.sh")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}
	var bad []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		mode, rest, ok := strings.Cut(line, " ")
		_, path, _ := strings.Cut(rest, "\t")
		if ok && mode != "100755" {
			bad = append(bad, path+" ("+mode+")")
		}
	}
	return bad
}

// ASD-V0-921 (V1-3243): every script/*_test.sh is committed executable, so
// a selected shell test runs directly; a 100644 one in a fixture index is
// named. make shell-test-registration-check runs this in doc-gates.
func TestASDV0921ScriptTestsAreExecutable(t *testing.T) {
	root := requireMake(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	// pendingChmod lists the script tests still committed 100644 only because cem/1.0 cannot bind a
	// mode change (metadata-only and old/new mode patches are refused), so a lane cannot carry the
	// chmod; the integrator makes it outside a bound change and removes the entry. An entry that is
	// already executable fails, so the list only shrinks.
	pendingChmod := map[string]bool{
		"script/release-env-compat_test.sh (100644)":  true,
		"script/runtime-environment_test.sh (100644)": true,
	}
	var bad []string
	for _, path := range nonExecutableScriptTests(t, root) {
		if !pendingChmod[path] {
			bad = append(bad, path)
		}
		delete(pendingChmod, path)
	}
	if len(bad) != 0 {
		t.Fatalf("script tests committed without the executable bit: %q; run git update-index --chmod=+x on each", bad)
	}
	if len(pendingChmod) != 0 {
		t.Fatalf("pendingChmod entries are executable now: %q; remove them", slices.Sorted(maps.Keys(pendingChmod)))
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{"ok_test.sh": 0o755, "plain_test.sh": 0o644} {
		if err := os.WriteFile(filepath.Join(dir, "script", name), []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "script"}, {"update-index", "--chmod=-x", "script/plain_test.sh"}, {"update-index", "--chmod=+x", "script/ok_test.sh"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := nonExecutableScriptTests(t, dir); !slices.Equal(got, []string{"script/plain_test.sh (100644)"}) {
		t.Errorf("fixture index: %q, want [script/plain_test.sh (100644)]", got)
	}
}

// listEntries returns the one-per-line entries of the backslash-continued
// list that starts at the line equal to head, with their line indexes.
func listEntries(t *testing.T, lines []string, head string) ([]string, []int) {
	t.Helper()
	start := slices.Index(lines, head)
	if start < 0 {
		t.Fatalf("no %q line", head)
	}
	var names []string
	var at []int
	for i := start + 1; i < len(lines); i++ {
		entry, ok := strings.CutSuffix(lines[i], " \\")
		if !ok {
			break
		}
		names = append(names, strings.TrimSpace(entry))
		at = append(at, i)
	}
	return names, at
}

// STR-V0-920 (V1-3470): GATE_STEPS and the shell-test prerequisites are one
// entry per line, sorted after GATE_STEPS's fixed head, so two lanes that
// each add a different gate shell step (a GATE_STEPS line and a shell-test
// line) merge with no conflict, and the merged Makefile lists both.
func TestGateStepsTwoLaneMergeIsClean_STR_V0_920(t *testing.T) {
	root := requireMake(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	text, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(text), "\n")
	const gateHead, shellHead, fixed = "GATE_STEPS = \\", "shell-test: \\", 5
	for _, list := range []struct {
		head string
		skip int
	}{{gateHead, fixed}, {shellHead, 0}} {
		names, _ := listEntries(t, lines, list.head)
		if len(names) <= list.skip || !slices.IsSorted(names[list.skip:]) {
			t.Fatalf("%q entries after the first %d are not sorted: %q", list.head, list.skip, names)
		}
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	write := func(lines []string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// lane inserts step into both lists at its sorted place.
	lane := func(step string) []string {
		out := slices.Clone(lines)
		for _, list := range []struct {
			head string
			skip int
		}{{gateHead, fixed}, {shellHead, 0}} {
			names, at := listEntries(t, out, list.head)
			i := list.skip + sortedIndex(names[list.skip:], step)
			pos := at[len(at)-1] + 1
			if i < len(at) {
				pos = at[i]
			}
			out = slices.Insert(out, pos, "  "+step+" \\")
		}
		return out
	}
	git("init", "-q")
	write(lines)
	git("add", "Makefile")
	git("commit", "-qm", "base")
	git("checkout", "-qb", "a")
	write(lane("aaa-lane-a-test"))
	git("commit", "-qam", "lane a")
	git("checkout", "-q", "main")
	git("checkout", "-qb", "b")
	write(lane("rg-lane-b-test"))
	git("commit", "-qam", "lane b")
	git("merge", "-q", "--no-edit", "a")
	merged, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	s, steps := scanMakefile(dir, "Makefile")
	if steps == nil {
		t.Fatalf("merged Makefile: no literal GATE_STEPS: %q", s.problems)
	}
	mergedLines := strings.Split(string(merged), "\n")
	shell, _ := listEntries(t, mergedLines, shellHead)
	for _, step := range []string{"aaa-lane-a-test", "rg-lane-b-test"} {
		if !slices.Contains(steps, step) || !slices.Contains(shell, step) {
			t.Errorf("merged Makefile lacks %s: GATE_STEPS %q, shell-test %q", step, steps, shell)
		}
	}
}

// sortedIndex returns the index at which name keeps sorted names sorted.
func sortedIndex(names []string, name string) int {
	i, _ := slices.BinarySearch(names, name)
	return i
}
