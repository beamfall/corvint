// Command test-commands-check is the STR-V0-006 declaration gate: the Go
// environment and flags declared in .github/cishards/test-commands.json
// (corvint-test-commands/0) must be exactly what `make go-test` runs, every
// declared language command that names a make target must exist, and no
// `make gate` step recipe may run a suite of a language declared `none`.
// `corvint test` reads the declaration as a plain file and never interprets
// the Makefile; this gate is what keeps the two from drifting
// (docs/specs/session-test-runner-v0.md).
//
// It does not compare recipe text. It runs the real `make -s go-test` under
// an environment of exactly PATH (a temporary directory holding a `go` shim,
// then /usr/bin:/bin) and the shim's record variable, so the Makefile
// defaults apply (a developer GO_TEST_TIMEOUT or CORVINT_GOCACHE override is
// not the declared full suite) and whatever make and the recipe shell hand
// the test process is observed: the shim, a copy of this program, records
// its argv and its whole environment and exits 0. The gate fails unless the
// shim ran exactly once, make exited zero, the argv is `test <go.flags>
// ./...` and the environment, less the names the gate passed (unchanged)
// and the closed set makeIntroduced, is go.env exactly.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

const (
	makeDeadline = time.Minute
	// recordVariable names the directory the go shim records into; the gate
	// passes it to make, so it is part of the passed environment.
	recordVariable = "CORVINT_TEST_COMMANDS_RECORD"
	// minimalPath follows the shim directory: what make's /bin/sh and the
	// Makefile's $(shell git ...) need, and nothing a developer prepends.
	minimalPath = "/usr/bin:/bin"
)

// makeIntroduced is the closed set of environment names that reach the test
// process from GNU make and the recipe shell rather than from the Makefile,
// each with where it was observed or documented (STR-V0-006). Any other name
// the shim sees that the gate did not pass must be declared in go.env.
var makeIntroduced = map[string]string{
	"MAKEFLAGS":               "GNU make 3.81 and 4.x",
	"MFLAGS":                  "GNU make 3.81 and 4.x",
	"MAKELEVEL":               "GNU make 3.81 and 4.x",
	"MAKE_RESTARTS":           "GNU make 4.x after a makefile remake",
	"MAKE_TERMOUT":            "GNU make 4.1+ when stdout is a terminal",
	"MAKE_TERMERR":            "GNU make 4.1+ when stderr is a terminal",
	"PWD":                     "/bin/sh (bash 3.2 and dash)",
	"SHLVL":                   "/bin/sh (bash)",
	"_":                       "/bin/sh (bash)",
	"MANPATH":                 "Apple's /usr/bin/make xcrun shim",
	"__CF_USER_TEXT_ENCODING": "Apple's /usr/bin/make xcrun shim",
}

// languageRunners names the programs that run a language's suite. A direct
// recipe word of a `make gate` step naming one, for a language declared
// none, is a difference: the gate runs that suite, so the declaration must
// name it. Only the words make prints are inspected, not what a script
// those words name goes on to run.
var languageRunners = map[string][]string{
	"dotnet":     {"dotnet"},
	"kotlin":     {"gradle", "gradlew", "kotlin", "kotlinc"},
	"python":     {"nox", "pipenv", "poetry", "py.test", "pytest", "python", "python2", "python3", "tox", "uv"},
	"ruby":       {"bundle", "bundler", "rake", "rspec", "ruby"},
	"rust":       {"cargo", "rustc"},
	"swift":      {"swift", "swiftc", "xcodebuild"},
	"typescript": {"bun", "deno", "jest", "mocha", "node", "nodejs", "npm", "npx", "playwright", "pnpm", "tsc", "vitest", "yarn"},
}

func main() {
	if recorded, err := shim(); recorded {
		if err != nil {
			fmt.Fprintln(os.Stderr, "test-commands-check shim: "+err.Error())
			os.Exit(1)
		}
		return
	}
	root := flag.String("root", ".", "repository root holding the Makefile")
	makefile := flag.String("makefile", "Makefile", "the makefile make reads, relative to --root")
	declaration := flag.String("declaration", "", "declaration file (default <root>/"+sessiontest.DeclarationPath+")")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: test-commands-check [--root DIR] [--makefile FILE] [--declaration FILE]")
		os.Exit(2)
	}
	if *declaration == "" {
		*declaration = filepath.Join(*root, filepath.FromSlash(sessiontest.DeclarationPath))
	}
	os.Exit(run(*root, *makefile, *declaration, os.Stdout, os.Stderr))
}

// shim is the `go` make runs: when recordVariable names a directory, this
// process writes its arguments and environment there, NUL-separated, and
// exits 0. Each invocation writes its own file, so a second call is counted.
func shim() (bool, error) {
	dir := os.Getenv(recordVariable)
	if dir == "" {
		return false, nil
	}
	var record bytes.Buffer
	fmt.Fprintf(&record, "%d\x00", len(os.Args)-1)
	for _, word := range os.Args[1:] {
		record.WriteString(word)
		record.WriteByte(0)
	}
	for _, entry := range os.Environ() {
		record.WriteString(entry)
		record.WriteByte(0)
	}
	file, err := os.OpenFile(filepath.Join(dir, strconv.Itoa(os.Getpid())+".record"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return true, err
	}
	_, err = file.Write(record.Bytes())
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return true, err
}

// parseRecord reads one shim record back: the argument count, the arguments
// and then the environment entries, each NUL-terminated.
func parseRecord(data []byte) (argv, env []string, err error) {
	fields := strings.Split(string(data), "\x00")
	if len(fields) < 2 || fields[len(fields)-1] != "" {
		return nil, nil, fmt.Errorf("malformed shim record (%d bytes)", len(data))
	}
	fields = fields[:len(fields)-1]
	count, err := strconv.Atoi(fields[0])
	if err != nil || count < 0 || count > len(fields)-1 {
		return nil, nil, fmt.Errorf("malformed shim record: argument count %q", fields[0])
	}
	return fields[1 : 1+count], fields[1+count:], nil
}

// run returns 0 when the declaration is what make runs, 1 on any difference
// or unreadable input, naming every difference on stderr.
func run(root, makefile, declaration string, stdout, stderr io.Writer) int {
	declared := sessiontest.ReadDeclaredFile(declaration)
	if declared.Unreadable != "" {
		fmt.Fprintln(stderr, "test-commands-check: "+declared.Unreadable)
		return 1
	}
	problems := observeGoTest(root, makefile, declared)
	targets := 0
	for _, entry := range declared.Languages {
		if len(entry.Argv) < 2 || entry.Argv[0] != "make" {
			continue
		}
		targets++
		if _, err := dryRun(root, makefile, entry.Argv[1:]...); err != nil {
			problems = append(problems, fmt.Sprintf("languages.%s names make %s: %v", entry.Language, strings.Join(entry.Argv[1:], " "), err))
		}
	}
	none, unknown := noneLanguages(declared)
	problems = append(problems, unknown...)
	if len(none) != 0 {
		problems = append(problems, checkGateSteps(root, makefile, none)...)
	}
	if len(problems) != 0 {
		for _, problem := range problems {
			fmt.Fprintln(stderr, "test-commands-check: "+problem)
		}
		return 1
	}
	fmt.Fprintf(stdout, "test-commands-check: %s equals make go-test (%s go test %s ./...); %d declared make targets dry-run; no make gate step recipe runs a suite of %v\n",
		sessiontest.DeclarationPath, strings.Join(declared.GoEnv, " "), strings.Join(declared.GoFlags, " "), targets, none)
	return 0
}

// observeGoTest runs the real `make -s go-test` with the go shim first on
// PATH and compares what the shim recorded with the declaration.
func observeGoTest(root, makefile string, declared sessiontest.Declared) []string {
	work, err := os.MkdirTemp("", "test-commands-check-")
	if err != nil {
		return []string{"cannot create the shim directory: " + err.Error()}
	}
	defer os.RemoveAll(work)
	bin, records := filepath.Join(work, "bin"), filepath.Join(work, "records")
	for _, dir := range []string{bin, records} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return []string{"cannot create the shim directory: " + err.Error()}
		}
	}
	if err := installShim(filepath.Join(bin, "go")); err != nil {
		return []string{"cannot install the go shim: " + err.Error()}
	}
	passed := []string{"PATH=" + bin + string(os.PathListSeparator) + minimalPath, recordVariable + "=" + records}
	_, err = runMake(root, passed, "-s", "-f", makefile, "go-test")
	entries, readErr := os.ReadDir(records)
	if readErr != nil {
		return []string{"cannot read the shim records: " + readErr.Error()}
	}
	problems := []string{}
	if err != nil {
		problems = append(problems, "make -s go-test: "+err.Error())
	}
	switch len(entries) {
	case 0:
		return append(problems, "make go-test never ran go from PATH: an absolute-path go, or a recipe that does not reach go test, is not the declared full suite")
	case 1:
	default:
		return append(problems, fmt.Sprintf("make go-test ran go %d times, want exactly once", len(entries)))
	}
	if err != nil {
		return problems
	}
	data, err := os.ReadFile(filepath.Join(records, entries[0].Name()))
	if err != nil {
		return append(problems, "cannot read the shim record: "+err.Error())
	}
	argv, env, err := parseRecord(data)
	if err != nil {
		return append(problems, err.Error())
	}
	want := append(append([]string{"test"}, declared.GoFlags...), "./...")
	if !slices.Equal(argv, want) {
		problems = append(problems, fmt.Sprintf("go.flags give %q, make go-test runs go with %q", strings.Join(want, " "), strings.Join(argv, " ")))
	}
	return append(problems, compareEnv(declared.GoEnv, passed, env)...)
}

// compareEnv checks the environment the shim saw: every name the gate passed
// is unchanged, every makeIntroduced name is ignored, and what remains,
// sorted, is exactly the declared go.env words.
func compareEnv(declared, passed, observed []string) []string {
	passedValues := map[string]string{}
	for _, entry := range passed {
		name, value, _ := strings.Cut(entry, "=")
		passedValues[name] = value
	}
	problems, extra := []string{}, []string{}
	for _, entry := range observed {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			problems = append(problems, fmt.Sprintf("make go-test runs go with the environment entry %q, which has no value", entry))
			continue
		}
		if want, ok := passedValues[name]; ok {
			if value != want {
				problems = append(problems, fmt.Sprintf("make go-test changes %s from %q to %q", name, want, value))
			}
			continue
		}
		if _, ok := makeIntroduced[name]; ok {
			continue
		}
		extra = append(extra, entry)
	}
	sort.Strings(extra)
	if !slices.Equal(extra, declared) {
		problems = append(problems, fmt.Sprintf("go.env is %v, make go-test runs go with %v", declared, extra))
	}
	return problems
}

// installShim copies this program to dst, where make will find it as `go`.
func installShim(dst string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	source, err := os.Open(self)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	return target.Close()
}

// noneLanguages lists the languages declared none, and a problem for each
// one whose runners this gate does not know: it cannot then check that no
// gate step runs the suite, so the declaration is refused rather than taken
// on trust.
func noneLanguages(declared sessiontest.Declared) ([]string, []string) {
	none, problems := []string{}, []string{}
	for _, entry := range declared.Languages {
		if entry.None == "" {
			continue
		}
		none = append(none, entry.Language)
		if _, ok := languageRunners[entry.Language]; !ok {
			problems = append(problems, fmt.Sprintf("languages.%s is none, but this gate knows no %s runners to look for in make gate: add them to languageRunners", entry.Language, entry.Language))
		}
	}
	return none, problems
}

// checkGateSteps dry-runs the GATE_STEPS steps and reports each direct
// recipe word that names a runner of a language declared none. All steps
// are dry-run together first; only when that finds a word is each step
// dry-run alone, to name the step.
func checkGateSteps(root, makefile string, none []string) []string {
	steps, err := gateSteps(root, makefile)
	if err != nil {
		return []string{"GATE_STEPS: " + err.Error()}
	}
	recipe, err := dryRun(root, makefile, steps...)
	if err != nil {
		return []string{fmt.Sprintf("make -s -n %s: %v", strings.Join(steps, " "), err)}
	}
	if len(runnerWords(recipe, none)) == 0 {
		return nil
	}
	problems := []string{}
	for _, step := range steps {
		recipe, err := dryRun(root, makefile, step)
		if err != nil {
			problems = append(problems, fmt.Sprintf("make -s -n %s: %v", step, err))
			continue
		}
		for _, found := range runnerWords(recipe, none) {
			problems = append(problems, fmt.Sprintf("languages.%s is none, but make gate step %s runs %q: a suite the gate runs must be declared as its command", found[0], step, found[1]))
		}
	}
	return problems
}

// runnerWords returns each (language, word) pair where a recipe word's
// program name is a runner of a language declared none.
func runnerWords(recipe string, none []string) [][2]string {
	found := [][2]string{}
	for _, word := range strings.FieldsFunc(recipe, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }) {
		program := path.Base(strings.Trim(word, "@-()&|;"))
		for _, language := range none {
			if slices.Contains(languageRunners[language], program) {
				found = append(found, [2]string{language, word})
			}
		}
	}
	return found
}

// gateSteps reads GATE_STEPS from make's own database, so the step list is
// the one `make gate` uses rather than a parse of the Makefile text.
func gateSteps(root, makefile string) ([]string, error) {
	database, err := runMake(root, []string{"PATH=" + minimalPath}, "-s", "-n", "-p", "-f", makefile, "go-test")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(database, "\n") {
		if steps, ok := strings.CutPrefix(line, "GATE_STEPS = "); ok {
			return strings.Fields(steps), nil
		}
	}
	return nil, fmt.Errorf("make -p prints no GATE_STEPS variable; the none-language check cannot run")
}

// dryRun prints what make would run for the targets, under an environment
// holding only the minimal PATH so no inherited MAKEFLAGS or variable
// override leaks in.
func dryRun(root, makefile string, targets ...string) (string, error) {
	return runMake(root, []string{"PATH=" + minimalPath}, append([]string{"-s", "-n", "-f", makefile}, targets...)...)
}

// runMake runs make from root with exactly env, returning its stdout and,
// on failure, its stderr in the error.
func runMake(root string, env []string, args ...string) (string, error) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), makeDeadline)
	defer cancel()
	cmd := exec.CommandContext(ctx, makePath, args...)
	cmd.Dir, cmd.Env = root, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
