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
// its argv, working directory and whole environment and exits 0. The gate
// fails unless the shim ran exactly once, make exited zero, the argv is
// `test <go.flags> ./...`, the working directory is the repository root, and
// the environment, less the names the gate passed (present and unchanged)
// and the closed set makeIntroduced, is go.env exactly. The makeIntroduced
// names are not taken on trust either: a control recipe run under the same
// make must show the same values, so a recipe that sets MAKEFLAGS itself is
// a difference, and the control makefile is read alone, so a Makefile-level
// MAKEFLAGS or SHELL setting differs from it too. What the shim cannot see, a
// second invocation through an absolute path, a wrapper or a script, is
// excluded by the recipe's shape: `make -n go-test` must print one command
// line of NAME=VALUE words then `go`, with no shell operator or substitution,
// and make's own database (`make -p`) must show the go-test recipe and every
// variable it expands as plain text: only $(NAME) references, no make
// function such as $(shell ...) that runs during expansion, no target-specific
// override of another kind, and SHELL left at make's default. Make functions
// expanded while the Makefile is read ($(shell ...) in a := assignment) run
// before any of this and are not observed.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
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
	// controlTarget is the recipe of the gate's own one-target makefile, run
	// alone under the same make to observe the values make and its default
	// shell give the makeIntroduced names.
	controlTarget = "__corvint_test_commands_control"
	// goTarget is the make target the declaration's go.env and go.flags
	// describe.
	goTarget = "go-test"
	// shellSyntax are the characters a go-test recipe line may not contain:
	// each can chain, substitute or redirect a command the shim never sees.
	shellSyntax = ";|&()<>{}`$\\\"'\r"
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
// process writes its arguments, working directory and environment there,
// NUL-separated, and exits 0. Each invocation writes its own file, so a
// second call is counted.
func shim() (bool, error) {
	dir := os.Getenv(recordVariable)
	if dir == "" {
		return false, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return true, err
	}
	var record bytes.Buffer
	fmt.Fprintf(&record, "%d\x00%s\x00", len(os.Args)-1, cwd)
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

// observation is what one shim record holds.
type observation struct {
	argv, env []string
	cwd       string
}

// parseRecord reads one shim record back: the argument count, the working
// directory, the arguments and then the environment entries, each
// NUL-terminated.
func parseRecord(data []byte) (observation, error) {
	fields := strings.Split(string(data), "\x00")
	if len(fields) < 3 || fields[len(fields)-1] != "" {
		return observation{}, fmt.Errorf("malformed shim record (%d bytes)", len(data))
	}
	fields = fields[:len(fields)-1]
	count, err := strconv.Atoi(fields[0])
	if err != nil || count < 0 || count > len(fields)-2 {
		return observation{}, fmt.Errorf("malformed shim record: argument count %q", fields[0])
	}
	return observation{cwd: fields[1], argv: fields[2 : 2+count], env: fields[2+count:]}, nil
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
	problems = append(problems, recipeShape(root, makefile)...)
	database, err := readDatabase(root, makefile)
	if err != nil {
		problems = append(problems, "make -p go-test: "+err.Error())
	} else {
		problems = append(problems, plainRecipe(database)...)
	}
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
	if len(none) != 0 && err == nil {
		problems = append(problems, checkGateSteps(root, makefile, database, none)...)
	}
	if len(problems) != 0 {
		for _, problem := range problems {
			fmt.Fprintln(stderr, "test-commands-check: "+problem)
		}
		return 1
	}
	fmt.Fprintf(stdout, "test-commands-check: %s equals make go-test (%s go test %s ./... in the repository root, one plain command line, no make function in the recipe); %d declared make targets dry-run; no make gate step recipe runs a suite of %v\n",
		sessiontest.DeclarationPath, strings.Join(declared.GoEnv, " "), strings.Join(declared.GoFlags, " "), targets, none)
	return 0
}

// observeGoTest runs the real `make -s go-test` with the go shim first on
// PATH and compares what the shim recorded with the declaration, and runs
// the control recipe under the same make to learn the values make itself
// gives the makeIntroduced names.
func observeGoTest(root, makefile string, declared sessiontest.Declared) []string {
	work, err := os.MkdirTemp("", "test-commands-check-")
	if err != nil {
		return []string{"cannot create the shim directory: " + err.Error()}
	}
	defer os.RemoveAll(work)
	bin := filepath.Join(work, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		return []string{"cannot create the shim directory: " + err.Error()}
	}
	if err := installShim(filepath.Join(bin, "go")); err != nil {
		return []string{"cannot install the go shim: " + err.Error()}
	}
	// The control recipe has the go-test recipe's shape, environment words
	// before go, so make runs it through the same shell rather than directly.
	// It is read alone: what the Makefile does to MAKEFLAGS or SHELL must show
	// up as a difference, not be inherited by the control.
	control := filepath.Join(work, "control.mk")
	if err := os.WriteFile(control, []byte(".PHONY: "+controlTarget+"\n"+controlTarget+":\n\t@"+strings.Join(append(slices.Clone(declared.GoEnv), "go", controlTarget), " ")+"\n"), 0o600); err != nil {
		return []string{"cannot write the control makefile: " + err.Error()}
	}
	wantRoot, err := filepath.Abs(root)
	if err == nil {
		wantRoot, err = filepath.EvalSymlinks(wantRoot)
	}
	if err != nil {
		return []string{"cannot resolve the repository root: " + err.Error()}
	}
	path := "PATH=" + bin + string(os.PathListSeparator) + minimalPath
	seen, passed, problems := observe(root, filepath.Join(work, "records"), path, "-s", "-f", makefile, "go-test")
	if len(problems) != 0 {
		return problems
	}
	want := append(append([]string{"test"}, declared.GoFlags...), "./...")
	if !slices.Equal(seen.argv, want) {
		problems = append(problems, fmt.Sprintf("go.flags give %q, make go-test runs go with %q", strings.Join(want, " "), strings.Join(seen.argv, " ")))
	}
	if cwd, err := filepath.EvalSymlinks(seen.cwd); err != nil || cwd != wantRoot {
		problems = append(problems, fmt.Sprintf("make go-test runs go in %s, want the repository root %s: a recipe that changes directory does not run the declared full suite", seen.cwd, wantRoot))
	}
	problems = append(problems, compareEnv(declared.GoEnv, passed, seen.env)...)
	baseline, _, controlProblems := observe(root, filepath.Join(work, "control-records"), path, "-s", "-f", control, controlTarget)
	if len(controlProblems) != 0 {
		return append(problems, "cannot observe make's own environment through the control recipe: "+strings.Join(controlProblems, "; "))
	}
	return append(problems, compareIntroduced(introducedValues(baseline.env), introducedValues(seen.env))...)
}

// observe runs make with exactly PATH and the record variable naming a fresh
// directory, and returns the one shim record it must produce. Make failing,
// no record (an absolute-path go, or a recipe that never reaches go) and more
// than one record are each a problem.
func observe(root, records, path string, args ...string) (observation, []string, []string) {
	if err := os.Mkdir(records, 0o700); err != nil {
		return observation{}, nil, []string{"cannot create the shim record directory: " + err.Error()}
	}
	passed := []string{path, recordVariable + "=" + records}
	_, err := runMake(root, passed, args...)
	entries, readErr := os.ReadDir(records)
	if readErr != nil {
		return observation{}, passed, []string{"cannot read the shim records: " + readErr.Error()}
	}
	target := args[len(args)-1]
	problems := []string{}
	if err != nil {
		problems = append(problems, fmt.Sprintf("make -s %s: %v", target, err))
	}
	switch len(entries) {
	case 0:
		return observation{}, passed, append(problems, fmt.Sprintf("make %s never ran go from PATH: an absolute-path go, or a recipe that does not reach go test, is not the declared full suite", target))
	case 1:
	default:
		return observation{}, passed, append(problems, fmt.Sprintf("make %s ran go %d times, want exactly once", target, len(entries)))
	}
	if err != nil {
		return observation{}, passed, problems
	}
	data, err := os.ReadFile(filepath.Join(records, entries[0].Name()))
	if err != nil {
		return observation{}, passed, []string{"cannot read the shim record: " + err.Error()}
	}
	seen, err := parseRecord(data)
	if err != nil {
		return observation{}, passed, []string{err.Error()}
	}
	return seen, passed, nil
}

// compareEnv checks the environment the shim saw: every name the gate passed
// is present and unchanged, every makeIntroduced name is set aside for
// compareIntroduced, and what remains, sorted, is exactly the declared
// go.env words.
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
			delete(passedValues, name)
			continue
		}
		if _, ok := makeIntroduced[name]; ok {
			continue
		}
		extra = append(extra, entry)
	}
	for _, name := range slices.Sorted(maps.Keys(passedValues)) {
		problems = append(problems, fmt.Sprintf("make go-test runs go without %s, which the gate passed", name))
	}
	sort.Strings(extra)
	if !slices.Equal(extra, declared) {
		problems = append(problems, fmt.Sprintf("go.env is %v, make go-test runs go with %v", declared, extra))
	}
	return problems
}

// introducedValues keeps the makeIntroduced entries of an environment.
func introducedValues(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok {
			if _, ok := makeIntroduced[name]; ok {
				values[name] = value
			}
		}
	}
	return values
}

// compareIntroduced requires the makeIntroduced names go-test's go saw to be
// exactly those the control recipe's go saw, with the same values: the set
// is excluded from the declaration because make and the shell set it, so a
// recipe or Makefile setting one of them is a difference.
func compareIntroduced(control, observed map[string]string) []string {
	problems := []string{}
	for _, name := range slices.Sorted(maps.Keys(makeIntroduced)) {
		want, inControl := control[name]
		got, inObserved := observed[name]
		switch {
		case inControl && !inObserved:
			problems = append(problems, fmt.Sprintf("make go-test runs go without %s, which make gives a plain recipe as %q", name, want))
		case inObserved && !inControl:
			problems = append(problems, fmt.Sprintf("make go-test runs go with %s=%q, which make gives a plain recipe no value: the Makefile or recipe sets it", name, got))
		case inObserved && got != want:
			problems = append(problems, fmt.Sprintf("make go-test runs go with %s=%q, which make gives a plain recipe as %q: the Makefile or recipe sets it", name, got, want))
		}
	}
	return problems
}

// recipeShape requires `make -n go-test` to print one plain command line:
// NAME=VALUE words, then the word `go`, and none of shellSyntax. The shim
// observes the go it is; this excludes what runs before or around it (an
// absolute path, a wrapper such as nice or a script, a chained or substituted
// command). The words after go are go's argv, which the shim records.
func recipeShape(root, makefile string) []string {
	printed, err := dryRun(root, makefile, goTarget)
	if err != nil {
		return []string{"make -s -n go-test: " + err.Error()}
	}
	lines := strings.Split(strings.TrimSuffix(printed, "\n"), "\n")
	if len(lines) != 1 {
		return []string{fmt.Sprintf("make -s -n go-test prints %d lines, want one plain go command line:\n%s", len(lines), printed)}
	}
	line := lines[0]
	if strings.ContainsAny(line, shellSyntax) {
		return []string{fmt.Sprintf("make -s -n go-test prints %q: the go-test recipe may not use shell operators, substitutions, redirections or quotes, which can run what the go shim does not see", line)}
	}
	words := strings.Fields(line)
	if len(words) != 0 {
		words[0] = strings.TrimLeft(words[0], "@-+")
	}
	for _, word := range words {
		if word == "go" {
			return nil
		}
		if name, _, ok := strings.Cut(word, "="); !ok || !plainName(name) {
			return []string{fmt.Sprintf("make -s -n go-test prints %q: want NAME=VALUE words then go, found %q, a program the go shim on PATH does not observe", line, word)}
		}
	}
	return []string{fmt.Sprintf("make -s -n go-test prints %q: want NAME=VALUE words then go, found no go word", line)}
}

// plainName reports whether name is a plain environment or make variable
// name: letters, digits and underscores, not starting with a digit.
func plainName(name string) bool {
	for i, r := range name {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i != 0:
		default:
			return false
		}
	}
	return name != ""
}

// definition is one variable in make's database: recursive (`=`, expanded
// when used) or simple (`:=`, already expanded when the Makefile was read).
type definition struct {
	value     string
	recursive bool
}

// database is what `make -p go-test` shows of the Makefile: the go-test recipe
// as written, the global and go-test-specific variable definitions, each
// global variable's origin comment, and the pattern-specific definitions whose
// pattern matches go-test.
type database struct {
	recipe  []string
	global  map[string]definition
	target  map[string]definition
	pattern map[string]patternDefinition
	origin  map[string]string
}

// patternDefinition is a pattern-specific variable and the pattern it is for.
type patternDefinition struct {
	definition
	target string
}

// readDatabase runs `make -s -n -p go-test` under the minimal environment and
// parses it.
func readDatabase(root, makefile string) (database, error) {
	printed, err := runMake(root, []string{"PATH=" + minimalPath}, "-s", "-n", "-p", "-f", makefile, goTarget)
	if err != nil {
		return database{}, err
	}
	db := parseDatabase(printed)
	if db.recipe == nil {
		return database{}, fmt.Errorf("make -p prints no %s: recipe", goTarget)
	}
	return db, nil
}

// parseDatabase reads make's database print, in GNU make 3.81's form and in
// 4.x's (`src/file.c`, `src/variable.c`, `src/commands.c` of 4.3): `NAME =
// value`, `NAME += value` and `NAME := value` lines (after an optional
// export/override/private), each under a `# <origin>` comment; `define NAME`
// ... `endef` blocks; the `# Pattern-specific Variable Values` section
// (`<pattern> :` lines, each followed by `# NAME = value` comments); the
// `go-test: NAME = value` lines 4.x prints before the target; and the
// `go-test:` block itself, whose `# NAME = value` comments are target-specific
// definitions (the `# automatic` ones are not) and whose lines after
// `#  commands to execute` (3.81) or `#  recipe to execute` (4.x), up to the
// blank line, are the recipe, each behind one recipe prefix character.
func parseDatabase(printed string) database {
	db := database{global: map[string]definition{}, target: map[string]definition{}, pattern: map[string]patternDefinition{}, origin: map[string]string{}}
	lines := strings.Split(printed, "\n")
	origin, pattern, inPatterns, inTarget, inRecipe, automatic := "", "", false, false, false, false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case inRecipe:
			if line != "" {
				if body := line[1:]; strings.TrimSpace(body) != "" {
					db.recipe = append(db.recipe, body)
				}
				continue
			}
			inRecipe, inTarget = false, false
		case inTarget:
			if strings.HasPrefix(line, "#  commands to execute") || strings.HasPrefix(line, "#  recipe to execute") {
				db.recipe, inRecipe = []string{}, true
				continue
			}
			if line == "" || !strings.HasPrefix(line, "#") {
				inTarget = false
				continue
			}
			if line == "# automatic" {
				automatic = true
				continue
			}
			if name, def, ok := parseDefinition(strings.TrimPrefix(line, "# ")); ok && !automatic {
				db.target[name] = def
			}
			automatic = false
			continue
		case inPatterns:
			if strings.HasSuffix(line, " :") && !strings.HasPrefix(line, "#") {
				pattern = strings.TrimSuffix(line, " :")
				continue
			}
			if strings.Contains(line, "pattern-specific variable values") {
				inPatterns = false
				continue
			}
			if name, def, ok := parseDefinition(strings.TrimPrefix(line, "# ")); ok && matchesTarget(pattern, goTarget) {
				db.pattern[name] = patternDefinition{def, pattern}
			}
			continue
		}
		if line == "# Pattern-specific Variable Values" {
			inPatterns = true
			continue
		}
		if rest, ok := strings.CutPrefix(line, goTarget+":"); ok {
			if name, def, ok := parseDefinition(strings.TrimPrefix(rest, " ")); ok {
				db.target[name] = def
				continue
			}
			inTarget, automatic = true, false
			continue
		}
		if rest, ok := strings.CutPrefix(line, "# "); ok {
			origin = rest
			continue
		}
		if name, ok := strings.CutPrefix(stripModifiers(line), "define "); ok {
			body := []string{}
			for i++; i < len(lines) && lines[i] != "endef"; i++ {
				body = append(body, lines[i])
			}
			db.global[name], db.origin[name] = definition{value: strings.Join(body, "\n"), recursive: true}, origin
			continue
		}
		if name, def, ok := parseDefinition(line); ok {
			db.global[name], db.origin[name] = def, origin
		}
	}
	return db
}

// matchesTarget reports whether a make pattern (one `%` standing for any
// non-empty string, or a literal name) matches the target.
func matchesTarget(pattern, target string) bool {
	prefix, suffix, ok := strings.Cut(pattern, "%")
	if !ok {
		return pattern == target
	}
	return len(target) > len(prefix)+len(suffix) && strings.HasPrefix(target, prefix) && strings.HasSuffix(target, suffix)
}

// stripModifiers removes the export, override and private words make prints
// before a definition.
func stripModifiers(line string) string {
	for {
		trimmed := line
		for _, modifier := range []string{"export ", "override ", "private "} {
			trimmed = strings.TrimPrefix(trimmed, modifier)
		}
		if trimmed == line {
			return line
		}
		line = trimmed
	}
}

// parseDefinition reads one `NAME = value`, `NAME += value` or `NAME :=
// value` line as make prints it: a whitespace-only value is printed as
// `$(subst ,,<value>)`, and a simple value has its `$` doubled.
func parseDefinition(line string) (string, definition, bool) {
	line = stripModifiers(line)
	name, rest, ok := strings.Cut(line, " ")
	if !ok || name == "" || strings.ContainsAny(name, "#$:=\t") {
		return "", definition{}, false
	}
	operator, value, _ := strings.Cut(rest, " ")
	if strings.HasPrefix(value, "$(subst ,,") && strings.HasSuffix(value, ")") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "$(subst ,,"), ")")
	}
	switch operator {
	case "=", "+=":
		return name, definition{value: value, recursive: true}, true
	case ":=":
		return name, definition{value: strings.ReplaceAll(value, "$$", "$")}, true
	}
	return "", definition{}, false
}

// plainRecipe requires the go-test recipe and every variable it expands to be
// plain text: $(NAME) and ${NAME} references only, each to a variable that is
// itself plain, so nothing runs while make expands the recipe (a make function
// such as $(shell ...) runs during the gate's dry run and observation yet
// prints nothing). SHELL and .SHELLFLAGS must be make's defaults, globally,
// for go-test itself and for any pattern matching it: a Makefile that sets
// them runs the recipe through a program the gate did not observe. (A
// prerequisite of go-test cannot carry a recipe of its own: the dry run would
// print it as a second line.)
func plainRecipe(db database) []string {
	problems := []string{}
	for _, name := range []string{"SHELL", ".SHELLFLAGS"} {
		const want = ": the go-test recipe must run through make's default shell"
		if origin, ok := db.origin[name]; ok && origin != "default" {
			problems = append(problems, fmt.Sprintf("the Makefile sets %s (%s)%s", name, origin, want))
		}
		if def, ok := db.target[name]; ok {
			problems = append(problems, fmt.Sprintf("the Makefile sets %s for go-test itself (%q)%s", name, def.value, want))
		}
		if def, ok := db.pattern[name]; ok {
			problems = append(problems, fmt.Sprintf("the Makefile sets %s for the pattern %s, which matches go-test (%q)%s", name, def.target, def.value, want))
		}
	}
	for _, line := range db.recipe {
		problems = append(problems, expandsPlainly(db, line, "the go-test recipe", map[string]bool{})...)
	}
	return problems
}

// expandsPlainly checks one piece of make text, named by where for the
// diagnostic, and the variables it references.
func expandsPlainly(db database, text, where string, visited map[string]bool) []string {
	problems := []string{}
	for rest := text; ; {
		dollar := strings.IndexByte(rest, '$')
		if dollar < 0 {
			return problems
		}
		after := rest[dollar+1:]
		closer := byte(0)
		switch {
		case strings.HasPrefix(after, "("):
			closer = ')'
		case strings.HasPrefix(after, "{"):
			closer = '}'
		}
		end := -1
		if closer != 0 {
			end = strings.IndexByte(after, closer)
		}
		if end < 0 || !plainName(strings.ReplaceAll(after[1:end], ".", "_")) {
			return append(problems, fmt.Sprintf("%s contains %q: only $(NAME) references to variables defined as plain text may appear in the go-test recipe, not make functions, substitutions or automatic variables", where, "$"+after))
		}
		name := after[1:end]
		rest = after[end+1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		def, isTarget := db.target[name]
		if !isTarget {
			def = db.global[name]
		}
		scope := "variable " + name
		if isTarget {
			scope = "go-test's own " + name
		}
		if def.recursive {
			problems = append(problems, expandsPlainly(db, def.value, scope, visited)...)
		} else if strings.Contains(def.value, "$") {
			problems = append(problems, fmt.Sprintf("%s is %q: a simply expanded value may not hold a $ for the recipe to expand", scope, def.value))
		}
	}
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
func checkGateSteps(root, makefile string, db database, none []string) []string {
	steps, err := gateSteps(db)
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
		program := path.Base(strings.Trim(word, "@-()&|;\"'"))
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
func gateSteps(db database) ([]string, error) {
	steps, ok := db.global["GATE_STEPS"]
	if !ok {
		return nil, fmt.Errorf("make -p prints no GATE_STEPS variable; the none-language check cannot run")
	}
	return strings.Fields(steps.value), nil
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
