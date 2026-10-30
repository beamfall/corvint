package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// windowsVetCommand is one `go vet` invocation of the CI step or of the
// Makefile windows-vet recipe: the module directory it runs in, its argv, and
// the variables that decide which files are type-checked.
type windowsVetCommand struct {
	Dir  string
	Argv string
	Env  map[string]string
}

// windowsVetKeys are the environment variables that select the files go vet
// sees, so local and CI runs must agree on every one (AFP-V0-054).
var windowsVetKeys = []string{"CGO_ENABLED", "GOARCH", "GOFLAGS", "GOOS", "GOTOOLCHAIN", "GOWORK"}

// parseWindowsVetLine reads `[(][cd DIR && ]K=V ... go vet ARGS[)]` over a base
// environment. Assignments whose value is a Make reference are left out.
func parseWindowsVetLine(t *testing.T, line string, base map[string]string) windowsVetCommand {
	t.Helper()
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "("), ")")
	command := windowsVetCommand{Dir: ".", Env: map[string]string{}}
	for key, value := range base {
		command.Env[key] = value
	}
	if rest, ok := strings.CutPrefix(line, "cd "); ok {
		dir, tail, found := strings.Cut(rest, " && ")
		if !found {
			t.Fatalf("cd without && in %q", line)
		}
		command.Dir, line = dir, tail
	}
	fields := strings.Fields(line)
	for len(fields) > 0 {
		key, value, ok := strings.Cut(fields[0], "=")
		if !ok || key == "" || strings.ContainsAny(key, "./") {
			break
		}
		if !strings.Contains(value, "$(") {
			command.Env[key] = value
		}
		fields = fields[1:]
	}
	command.Argv = strings.Join(fields, " ")
	if !strings.HasPrefix(command.Argv, "go vet ") {
		t.Fatalf("not a go vet command: %q", line)
	}
	return command
}

func readRepositoryFile(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(raw), "\n")
}

// ciWindowsVetStep is the name of the go-static step AFP-V0-054 mirrors.
const ciWindowsVetStep = "- name: Windows vet including test files (AFP-V0-042)"

// ciWindowsVetCommands returns the go vet lines of the go-static step
// "Windows vet including test files (AFP-V0-042)" of ci.yml.
func ciWindowsVetCommands(t *testing.T) []windowsVetCommand {
	t.Helper()
	return parseCIWindowsVetCommands(t, readRepositoryFile(t, ".github/workflows/ci.yml"))
}

// parseCIWindowsVetCommands reads the go vet lines of the Windows vet step over
// its effective environment: the go-static job env with the step's own env
// merged over it, as GitHub Actions applies them.
func parseCIWindowsVetCommands(t *testing.T, lines []string) []windowsVetCommand {
	t.Helper()
	start := slices.Index(lines, "  go-static:")
	if start < 0 {
		t.Fatal("ci.yml has no go-static job")
	}
	jobEnv, stepEnv := map[string]string{}, map[string]string{}
	var target map[string]string // the env block being read, if any
	entryIndent := ""            // the indent of that block's entries
	var vetLines []string
	inStep := false
	envEntry := func(line, indent string) bool {
		return strings.HasPrefix(line, indent) && !strings.HasPrefix(line, indent+" ")
	}
	for _, line := range lines[start+1:] {
		if envEntry(line, "  ") && strings.HasSuffix(line, ":") {
			break // the next job
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case line == "    env:":
			target, entryIndent = jobEnv, "      "
			continue
		case inStep && line == "        env:":
			target, entryIndent = stepEnv, "          "
			continue
		case target != nil && envEntry(line, entryIndent):
			key, value, _ := strings.Cut(trimmed, ":")
			target[key] = strings.Trim(strings.TrimSpace(value), `"`)
			continue
		}
		target = nil
		if strings.HasPrefix(trimmed, "- name: ") {
			inStep = trimmed == ciWindowsVetStep
			continue
		}
		if inStep && strings.Contains(trimmed, "go vet") {
			vetLines = append(vetLines, trimmed)
		}
	}
	if len(vetLines) == 0 {
		t.Fatal("ci.yml go-static has no Windows vet step with a go vet line")
	}
	effective := map[string]string{}
	for key, value := range jobEnv {
		effective[key] = value
	}
	for key, value := range stepEnv {
		effective[key] = value
	}
	commands := make([]windowsVetCommand, 0, len(vetLines))
	for _, line := range vetLines {
		commands = append(commands, parseWindowsVetLine(t, line, effective))
	}
	return commands
}

// windowsVetMismatches lists every way the local commands differ from CI's.
func windowsVetMismatches(ci, local []windowsVetCommand) []string {
	if len(ci) != len(local) {
		return []string{fmt.Sprintf("CI runs %d go vet commands, make windows-vet runs %d", len(ci), len(local))}
	}
	var mismatches []string
	for index := range ci {
		if ci[index].Dir != local[index].Dir || ci[index].Argv != local[index].Argv {
			mismatches = append(mismatches, fmt.Sprintf("command %d: CI %q in %q, make %q in %q", index, ci[index].Argv, ci[index].Dir, local[index].Argv, local[index].Dir))
		}
		for _, key := range windowsVetKeys {
			got, inLocal := local[index].Env[key]
			want, inCI := ci[index].Env[key]
			if !inLocal || !inCI || got != want {
				mismatches = append(mismatches, fmt.Sprintf("command %d %s: CI %q (set %v), make %q (set %v)", index, key, want, inCI, got, inLocal))
			}
		}
	}
	return mismatches
}

// makeWindowsVetCommands returns the windows-vet recipe of the Makefile with
// $(WINDOWS_VET_ENV) expanded.
func makeWindowsVetCommands(t *testing.T) []windowsVetCommand {
	t.Helper()
	lines := readRepositoryFile(t, "Makefile")
	environment := ""
	for _, line := range lines {
		if value, ok := strings.CutPrefix(line, "WINDOWS_VET_ENV = "); ok {
			environment = value
		}
	}
	if environment == "" {
		t.Fatal("Makefile defines no WINDOWS_VET_ENV")
	}
	start := slices.IndexFunc(lines, func(line string) bool {
		return line == "windows-vet:" || strings.HasPrefix(line, "windows-vet: ")
	})
	if start < 0 {
		t.Fatal("Makefile has no windows-vet target")
	}
	var commands []windowsVetCommand
	for _, line := range lines[start+1:] {
		recipe, ok := strings.CutPrefix(line, "\t")
		if !ok {
			break
		}
		commands = append(commands, parseWindowsVetLine(t, strings.ReplaceAll(recipe, "$(WINDOWS_VET_ENV)", environment), nil))
	}
	return commands
}

// AFP-V0-054 (1): the Makefile windows-vet target runs exactly the CI step's
// go vet commands, module by module, under the same file-selecting variables,
// with persisted `go env -w` settings off and the go-version check first.
func TestAFPV0054WindowsVetTargetMatchesTheCIStep(t *testing.T) {
	ci, local := ciWindowsVetCommands(t), makeWindowsVetCommands(t)
	for _, mismatch := range windowsVetMismatches(ci, local) {
		t.Error(mismatch)
	}
	if ci[0].Env["GOOS"] != "windows" || ci[0].Env["GOFLAGS"] != "" {
		t.Fatalf("CI step is not an untagged GOOS=windows vet: %+v", ci[0].Env)
	}
	for index, command := range local {
		if command.Env["GOENV"] != "off" {
			t.Errorf("make windows-vet command %d runs with GOENV=%q, want off", index, command.Env["GOENV"])
		}
	}
	lines := readRepositoryFile(t, "Makefile")
	if !slices.Contains(lines, "windows-vet: go-version") {
		t.Error("Makefile windows-vet does not depend on go-version")
	}
	start := slices.Index(lines, "go-version:")
	// Decision 0557: the go-version target admits exactly the one pin.
	if start < 0 || start+1 >= len(lines) || lines[start+1] != "\ttest \"$$(GOTOOLCHAIN=local go env GOVERSION)\" = \"go1.27.3\"" {
		t.Fatal("Makefile go-version does not check exactly go1.27.3")
	}
	ciYAML := readRepositoryFile(t, ".github/workflows/ci.yml")
	if !slices.Contains(ciYAML, `          go-version: "1.27.3"`) {
		t.Error("ci.yml setup-go no longer pins 1.27.3; update go-version and AFP-V0-054")
	}
}

// AFP-V0-054 (1): a step-level env entry overrides the job env, so a tag added
// to the Windows vet step's own env is seen and breaks parity.
func TestAFPV0054StepEnvOverridesJobEnv(t *testing.T) {
	lines := readRepositoryFile(t, ".github/workflows/ci.yml")
	index := slices.Index(lines, "      "+ciWindowsVetStep)
	if index < 0 {
		t.Fatal("ci.yml has no Windows vet step")
	}
	envAt := -1
	for offset, line := range lines[index+1:] {
		if line == "        env:" {
			envAt = index + 1 + offset
			break
		}
		if strings.HasPrefix(strings.TrimSpace(line), "- name: ") {
			break
		}
	}
	if envAt < 0 {
		t.Fatal("Windows vet step has no env block to mutate")
	}
	mutated := slices.Insert(slices.Clone(lines), envAt+1, `          GOFLAGS: "-tags=special"`)
	ci := parseCIWindowsVetCommands(t, mutated)
	if ci[0].Env["GOFLAGS"] != "-tags=special" {
		t.Fatalf("step-level GOFLAGS not merged over the job env: %q", ci[0].Env["GOFLAGS"])
	}
	if len(windowsVetMismatches(ci, makeWindowsVetCommands(t))) == 0 {
		t.Fatal("a step-level GOFLAGS override did not break parity")
	}
}

// AFP-V0-054 (1): GOENV=off keeps a GOFLAGS saved by `go env -w` out of the
// recipe; an empty GOFLAGS alone falls back to the saved value.
func TestAFPV0054WindowsVetIgnoresSavedGoEnv(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go tool: %v", err)
	}
	saved := filepath.Join(t.TempDir(), "go.env")
	if err := os.WriteFile(saved, []byte("GOFLAGS=-tags=local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goflags := func(env map[string]string) string {
		run := exec.Command(goTool, "env", "GOFLAGS")
		run.Env = append(os.Environ(), "GOENV="+saved)
		for key, value := range env {
			run.Env = append(run.Env, key+"="+value)
		}
		output, err := run.CombinedOutput()
		if err != nil {
			t.Fatalf("go env: %v\n%s", err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if got := goflags(map[string]string{"GOFLAGS": "", "GOTOOLCHAIN": "local"}); got != "-tags=local" {
		t.Fatalf("control: empty GOFLAGS with a saved value gave %q, want -tags=local", got)
	}
	for index, command := range makeWindowsVetCommands(t) {
		if got := goflags(command.Env); got != "" {
			t.Errorf("make windows-vet command %d sees saved GOFLAGS %q", index, got)
		}
	}
}

// AFP-V0-054 (2): the repository's AGENTS.md Verify section declares
// `make windows-vet`, so `corvint affected` lists it, conditional on a Go change.
func TestAFPV0054AffectedListsWindowsVet(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checks, unknown, truncated := mandatoryAffectedChecks(root)
	if truncated || len(unknown) != 0 {
		t.Fatalf("declarations truncated=%v unknown=%q", truncated, unknown)
	}
	for _, check := range checks {
		if check.Command != "make windows-vet" {
			continue
		}
		if check.Kind != adviceKindConditional || check.Source != adviceAgentsName || !strings.Contains(check.Reason, "`.go`, `go.mod` or `go.sum`") {
			t.Fatalf("make windows-vet check = %+v", check)
		}
		return
	}
	t.Fatalf("affected advice lists no make windows-vet: %+v", checks)
}

// AFP-V0-054 (3): a test that calls a helper defined only in a unix-constrained
// file (the PR #1089 shape) passes the native vet and fails the windows-vet
// recipe line, run as the Makefile states it, before any push.
func TestAFPV0054WindowsVetFlagsUnixOnlyTestHelper(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go tool: %v", err)
	}
	module := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.com/winvetfixture\n\ngo 1.27\n",
		"audit.go":            "package audit\n\nfunc Audit() int { return 1 }\n",
		"holder_unix_test.go": "//go:build unix\n\npackage audit\n\nfunc waitForHolder() int { return 1 }\n",
		"audit_test.go":       "package audit\n\nimport \"testing\"\n\nfunc TestAudit(t *testing.T) {\n\tif Audit() != waitForHolder() {\n\t\tt.Fatal(\"mismatch\")\n\t}\n}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(module, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	vet := func(command windowsVetCommand) (string, error) {
		argv := strings.Fields(command.Argv)
		run := exec.Command(goTool, argv[1:]...)
		run.Dir = module
		run.Env = os.Environ()
		for key, value := range command.Env {
			run.Env = append(run.Env, key+"="+value)
		}
		var output bytes.Buffer
		run.Stdout, run.Stderr = &output, &output
		err := run.Run()
		return output.String(), err
	}
	recipe := makeWindowsVetCommands(t)[0]
	if recipe.Dir != "." {
		t.Fatalf("first windows-vet line runs in %q, want the root module", recipe.Dir)
	}
	if runtime.GOOS != "windows" {
		native := windowsVetCommand{Argv: recipe.Argv, Env: map[string]string{"GOFLAGS": "", "GOWORK": "off", "GOTOOLCHAIN": "local"}}
		if output, err := vet(native); err != nil {
			t.Fatalf("native go vet should pass on %s: %v\n%s", runtime.GOOS, err, output)
		}
	}
	output, err := vet(recipe)
	if err == nil {
		t.Fatalf("windows-vet passed a test using a unix-only helper:\n%s", output)
	}
	if !strings.Contains(output, "undefined: waitForHolder") {
		t.Fatalf("windows-vet failed for another reason: %v\n%s", err, output)
	}
}
