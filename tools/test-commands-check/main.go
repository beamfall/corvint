// Command test-commands-check is the STR-V0-006 declaration gate. The Go
// full suite is declared once, in .github/cishards/test-commands.json
// (corvint-test-commands/0); `corvint test` reads that file as plain data and
// `make go-test` runs it through tools/test-commands-run. This gate keeps the
// Makefile from meaning anything else: it reads the Makefile source text, not
// make's output, and fails unless
//
//   - the go-test rule is exactly `go-test:` with the one recipe line
//     `go run ./tools/test-commands-run`: no prerequisites, no second recipe
//     line, no second rule, and no other rule with a recipe whose target could
//     match go-test (the literal name, a `%` pattern it fits, or a target make
//     would expand first; a prerequisite-only line such as the GOC-V0-010
//     order-only clear is allowed);
//   - no line of the Makefile or of a file it `include`s assigns SHELL,
//     .SHELLFLAGS, MAKEFLAGS or .RECIPEPREFIX (globally, exported, or for any
//     target or pattern), exports MAKEFLAGS or every variable (`export` alone,
//     `.EXPORT_ALL_VARIABLES`), or defines a target- or pattern-specific
//     variable whose target could match go-test;
//   - the text is readable without running make: no `$(eval ...)`, no
//     `define`, and only literal include names;
//   - every declared language command that names a make target exists
//     (`make -n`), and no GATE_STEPS step recipe runs a suite of a language
//     declared none (GATE_STEPS is one plain `GATE_STEPS = ...` line).
//
// A line inside a conditional counts as live, so the check is stricter than
// make, never looser. What it does not read: the environment make itself is
// started with, an `export` of another name (a Makefile `export GOFLAGS`
// reaches go through the runner's inherited environment), and the other
// tiers (`ledger/go-test`, `gate-affected`) that still build their own go
// test command from Makefile variables (docs/specs/session-test-runner-v0.md).
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
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

const (
	makeDeadline = time.Minute
	// minimalPath is what make's /bin/sh and the Makefile's $(shell git ...)
	// need for a dry run, and nothing a developer prepends.
	minimalPath = "/usr/bin:/bin"
	// goTarget is the make target that runs the declaration.
	goTarget = "go-test"
	// goRecipe is the only recipe line goTarget may have.
	goRecipe = "go run ./tools/test-commands-run"
	// gateStepsVariable lists the `make gate` steps whose recipes the
	// none-language check reads.
	gateStepsVariable = "GATE_STEPS"
	// includeDepth bounds how deep `include` files are followed.
	includeDepth = 4
	// ledgerTarget is the rule `make gate` runs Go tests through
	// (docs/specs/gate-ledger-v0.md) instead of goTarget; ledgerRecipe is
	// the only recipe line it may have.
	ledgerTarget = "ledger/go-test"
	ledgerRecipe = "@$(GATE_LEDGER) go-test -- go test $(GO_TEST_FLAGS)"
)

// ledgerNames are the variables ledgerRecipe expands. Each must have exactly
// one global assignment and no target-specific one, so the Makefile text
// alone says what `make gate` runs and checkLedger can compare it with the
// declaration.
var ledgerNames = map[string]bool{"GATE_LEDGER": true, "GO_TEST_FLAGS": true, "GO_TEST_TIMEOUT": true, "CORVINT_GOCACHE": true}

// tmpdirDefault is the one make expression a ledger value may hold besides
// ledgerNames: the Makefile's CORVINT_GOCACHE default directory, TMPDIR without
// its trailing slash or else /tmp (ASD-V0-914). checkLedger resolves it as /tmp,
// its value in the declared CI environment, which sets no TMPDIR; a caller's
// TMPDIR only moves the build cache, as a caller's CORVINT_GOCACHE does
// (STR-V0-174, STR-V0-908).
const tmpdirDefault = "$(patsubst %/,%,$(or $(TMPDIR),/tmp))"

// controlNames are the variables whose assignment anywhere, global or for
// any target, changes how make runs every recipe, so the fixed go-test line
// would not mean what it says.
var controlNames = map[string]bool{"SHELL": true, ".SHELLFLAGS": true, "MAKEFLAGS": true, ".RECIPEPREFIX": true}

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

func run(root, makefile, declaration string, stdout, stderr io.Writer) int {
	declared := sessiontest.ReadDeclaredFile(declaration)
	if declared.Unreadable != "" {
		fmt.Fprintln(stderr, "test-commands-check: "+declared.Unreadable)
		return 1
	}
	s, steps := scanMakefile(root, makefile)
	problems := append(s.problems, checkLedger(s, declared)...)
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
	if len(none) != 0 && steps != nil {
		problems = append(problems, checkGateSteps(root, makefile, steps, none)...)
	}
	if steps != nil {
		problems = append(problems, checkGoTestTimeouts(root, makefile, steps)...)
	}
	if len(problems) != 0 {
		for _, problem := range problems {
			fmt.Fprintln(stderr, "test-commands-check: "+problem)
		}
		return 1
	}
	ledger := "no ledger/go-test rule, so make gate reaches go-test"
	if s.ledgerRules == 1 {
		ledger = "ledger/go-test runs the declared go.env and go.flags through gate-ledger"
	}
	fmt.Fprintf(stdout, "test-commands-check: %s go-test is exactly `%s`, which runs %s (%s go test %s ./... in the repository root); %s; no SHELL, .SHELLFLAGS, MAKEFLAGS or go-test variable in the Makefile; %d declared make targets dry-run; no make gate step recipe runs a suite of %v\n",
		makefile, goRecipe, sessiontest.DeclarationPath, strings.Join(declared.GoEnv, " "), strings.Join(declared.GoFlags, " "), ledger, targets, none)
	return 0
}

// checkMakefile reads the Makefile text and its literal includes and returns
// the problems found and, when exactly one plain GATE_STEPS line exists, its
// steps (nil otherwise, with the reason among the problems).
func checkMakefile(root, makefile string) ([]string, []string) {
	s, steps := scanMakefile(root, makefile)
	return s.problems, steps
}

func scanMakefile(root, makefile string) (*scanner, []string) {
	s := &scanner{root: root, seen: map[string]bool{}, ledger: map[string][]assignment{}}
	s.scan(makefile, includeDepth)
	switch {
	case s.rules != 1:
		s.problems = append(s.problems, fmt.Sprintf("%s has %d `%s:` rules, want exactly one with the recipe `%s`", makefile, s.rules, goTarget, goRecipe))
	case s.recipes == 0:
		s.problems = append(s.problems, fmt.Sprintf("%s: `%s:` has no recipe line, want exactly `%s`", s.ruleAt, goTarget, goRecipe))
	}
	var steps []string
	switch {
	case len(s.gateSteps) != 1:
		s.problems = append(s.problems, fmt.Sprintf("%s is assigned %d times; the none-language check needs exactly one plain `%s = ...` line", gateStepsVariable, len(s.gateSteps), gateStepsVariable))
	case strings.Contains(s.gateSteps[0].value, "$"):
		s.problems = append(s.problems, fmt.Sprintf("%s: %s value %q is expanded by make; the none-language check reads only a literal step list", s.gateSteps[0].at, gateStepsVariable, s.gateSteps[0].value))
	default:
		steps = strings.Fields(s.gateSteps[0].value)
	}
	switch {
	case s.ledgerRules > 1:
		s.problems = append(s.problems, fmt.Sprintf("%s has %d `%s:` rules, want at most one with the recipe `%s`", makefile, s.ledgerRules, ledgerTarget, ledgerRecipe))
	case s.ledgerRules == 1 && s.ledgerRecipes == 0:
		s.problems = append(s.problems, fmt.Sprintf("%s: `%s:` has no recipe line, want exactly `%s`", s.ledgerAt, ledgerTarget, ledgerRecipe))
	}
	return s, steps
}

// checkLedger compares what `make gate` runs for Go tests with the
// declaration. Without a `ledger/go-test:` rule, `make gate` reaches go-test
// through the ledger/% sub-make, which runs the declaration itself. With
// one, its fixed recipe expands ledgerNames from their single literal
// assignments: the environment words before `go` must be the declared go.env
// and GO_TEST_FLAGS the declared go.flags except -count=1, which gate-ledger
// adds only for the packages it cannot key (gate-ledger-v0), running the
// others through Go's test cache.
func checkLedger(s *scanner, declared sessiontest.Declared) []string {
	if s.ledgerRules != 1 {
		return nil
	}
	var problems []string
	values := map[string]string{}
	for name := range ledgerNames {
		assigned := s.ledger[name]
		switch {
		case len(assigned) != 1:
			problems = append(problems, fmt.Sprintf("%s is assigned %d times; `%s:` needs exactly one plain assignment of it", name, len(assigned), ledgerTarget))
		case !slices.Contains([]string{"=", ":=", "::=", "?="}, assigned[0].op):
			problems = append(problems, fmt.Sprintf("%s: %s is assigned with %s; `%s:` needs a plain assignment", assigned[0].at, name, assigned[0].op, ledgerTarget))
		default:
			values[name] = strings.ReplaceAll(assigned[0].value, tmpdirDefault, "/tmp")
		}
	}
	if len(problems) != 0 {
		return problems
	}
	expand := func(name string) (string, error) {
		value := values[name]
		for depth := 0; strings.Contains(value, "$"); depth++ {
			replaced := value
			for other := range ledgerNames {
				replaced = strings.ReplaceAll(replaced, "$("+other+")", values[other])
			}
			if replaced == value || depth == len(ledgerNames) {
				return "", fmt.Errorf("%s expands %q, which is not one of %v", name, value, slices.Sorted(maps.Keys(ledgerNames)))
			}
			value = replaced
		}
		return value, nil
	}
	ledger, err := expand("GATE_LEDGER")
	if err != nil {
		return []string{err.Error()}
	}
	flags, err := expand("GO_TEST_FLAGS")
	if err != nil {
		return []string{err.Error()}
	}
	words := strings.Fields(ledger)
	env := []string{}
	for len(words) != 0 && strings.Contains(words[0], "=") {
		env, words = append(env, words[0]), words[1:]
	}
	slices.Sort(env)
	if !slices.Equal(env, declared.GoEnv) {
		problems = append(problems, fmt.Sprintf("`%s:` runs go with %v (GATE_LEDGER), but %s declares go.env %v", ledgerTarget, env, sessiontest.DeclarationPath, declared.GoEnv))
	}
	want := slices.DeleteFunc(slices.Clone(declared.GoFlags), func(flag string) bool { return flag == "-count=1" })
	if got := strings.Fields(flags); !slices.Equal(got, want) {
		problems = append(problems, fmt.Sprintf("`%s:` runs go test %v (GO_TEST_FLAGS), but %s declares go.flags %v (gate-ledger adds -count=1 itself)", ledgerTarget, got, sessiontest.DeclarationPath, declared.GoFlags))
	}
	return problems
}

// logical is one makefile line with its backslash continuations joined,
// located by file and first physical line.
type logical struct {
	text string
	at   string
}

// logicalLines splits data into make's logical lines. A continuation is
// joined with one space, as make does outside recipes; inside the go-test
// recipe a continuation then simply fails the exact comparison.
func logicalLines(name string, data []byte) []logical {
	physical := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	lines := make([]logical, 0, len(physical))
	for i := 0; i < len(physical); i++ {
		start, text := i+1, physical[i]
		for strings.HasSuffix(text, `\`) && i+1 < len(physical) {
			i++
			text = strings.TrimRight(strings.TrimSuffix(text, `\`), " \t") + " " + strings.TrimLeft(physical[i], " \t")
		}
		lines = append(lines, logical{text: text, at: fmt.Sprintf("%s:%d", name, start)})
	}
	return lines
}

type assignment struct {
	value string
	at    string
	op    string
}

type ruleKind int

const (
	otherRule ruleKind = iota
	goTestRule
	matchingRule
	ledgerRule
)

// scanner walks the Makefile text. rule, ruleLine and ruleWord describe the
// rule the following recipe lines belong to.
type scanner struct {
	root      string
	seen      map[string]bool
	problems  []string
	gateSteps []assignment
	rules     int    // `go-test:` rules seen
	recipes   int    // recipe lines of the go-test rule
	ruleAt    string // where the go-test rule is
	rule      ruleKind
	ruleLine  string
	ruleWord  string // the target word that could match go-test
	// ledger holds the global assignments of ledgerNames; ledgerRules and
	// ledgerRecipes count the `ledger/go-test:` rules and their recipe lines.
	ledger        map[string][]assignment
	ledgerRules   int
	ledgerRecipes int
	ledgerAt      string
}

func (s *scanner) problem(at, format string, args ...any) {
	s.problems = append(s.problems, at+": "+fmt.Sprintf(format, args...))
}

func (s *scanner) scan(name string, depth int) {
	full := name
	if !filepath.IsAbs(full) {
		full = filepath.Join(s.root, name)
	}
	if s.seen[full] {
		return
	}
	s.seen[full] = true
	data, err := os.ReadFile(full)
	if err != nil {
		s.problems = append(s.problems, err.Error())
		return
	}
	for _, line := range logicalLines(name, data) {
		if strings.Contains(line.text, "$(eval") || strings.Contains(line.text, "${eval") {
			s.problem(line.at, "$(eval ...) adds makefile text when make expands it, which this check does not read")
		}
		if strings.HasPrefix(line.text, "\t") {
			s.recipe(line)
			continue
		}
		text := line.text
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue // blank and comment lines do not end a recipe
		}
		if conditional(text) {
			// A conditional directive does not end a recipe either: make
			// keeps the tab lines of every branch with the rule above, so
			// they all count against it.
			s.statement(line.at, text, depth)
			continue
		}
		s.rule, s.ruleLine, s.ruleWord = otherRule, "", ""
		s.statement(line.at, text, depth)
	}
}

// conditional reports whether text is a conditional directive.
func conditional(text string) bool {
	switch strings.Fields(text)[0] {
	case "ifeq", "ifneq", "ifdef", "ifndef", "else", "endif":
		return true
	}
	return false
}

// recipe classifies a tab-prefixed line by the rule it belongs to.
func (s *scanner) recipe(line logical) {
	switch s.rule {
	case goTestRule:
		s.recipes++
		switch {
		case s.recipes > 1:
			s.problem(line.at, "`%s:` has a second recipe line %q; its only recipe is `%s`", goTarget, strings.TrimPrefix(line.text, "\t"), goRecipe)
		case line.text != "\t"+goRecipe:
			s.problem(line.at, "`%s:` recipe is %q, want exactly `%s`", goTarget, strings.TrimPrefix(line.text, "\t"), goRecipe)
		}
	case ledgerRule:
		s.ledgerRecipes++
		switch {
		case s.ledgerRecipes > 1:
			s.problem(line.at, "`%s:` has a second recipe line %q; its only recipe is `%s`", ledgerTarget, strings.TrimPrefix(line.text, "\t"), ledgerRecipe)
		case line.text != "\t"+ledgerRecipe:
			s.problem(line.at, "`%s:` recipe is %q, want exactly `%s`", ledgerTarget, strings.TrimPrefix(line.text, "\t"), ledgerRecipe)
		}
	case matchingRule:
		s.problem(line.at, "recipe line %q belongs to the rule %q, whose target %s %s; only the `%s:` rule may give go-test a recipe", strings.TrimPrefix(line.text, "\t"), s.ruleLine, s.ruleWord, matchReason(s.ruleWord), goTarget)
	}
}

// statement classifies one non-recipe logical line.
func (s *scanner) statement(at, text string, depth int) {
	fields := strings.Fields(text)
	switch fields[0] {
	case "ifeq", "ifneq", "ifdef", "ifndef", "else", "endif", "endef", "vpath", "unexport", "undefine":
		return
	case "define":
		s.problem(at, "`define` holds makefile text this check does not read")
		return
	case "include", "-include", "sinclude":
		s.include(at, fields[0], fields[1:], depth)
		return
	case "export":
		if len(fields) == 1 {
			s.problem(at, "`export` alone exports every make variable into go's environment")
			return
		}
		if kind, _, _, _ := classify(text); kind != assignmentLine {
			if slices.Contains(fields[1:], "MAKEFLAGS") {
				s.problem(at, "`export MAKEFLAGS` hands make's flags to every recipe")
			}
			return
		}
	}
	kind, left, op, right := classify(text)
	switch kind {
	case assignmentLine:
		name := variableName(left)
		if controlNames[name] {
			s.problem(at, "%q assigns %s, which changes how make runs every recipe", text, name)
		}
		if ledgerNames[name] {
			s.ledger[name] = append(s.ledger[name], assignment{value: strings.TrimSpace(right), at: at, op: op})
		}
		if name == gateStepsVariable {
			s.gateSteps = append(s.gateSteps, assignment{value: strings.TrimSpace(right), at: at})
		}
	case ruleLine:
		s.ruleStatement(at, text, strings.Fields(left), right)
	}
}

// ruleStatement handles `targets: ...`: a target-specific variable when the
// text after the colon is an assignment, otherwise a rule.
func (s *scanner) ruleStatement(at, text string, targets []string, rest string) {
	if slices.Contains(targets, ".EXPORT_ALL_VARIABLES") {
		s.problem(at, "`.EXPORT_ALL_VARIABLES` exports every make variable into go's environment")
		return
	}
	if slices.Contains(targets, ".IGNORE") {
		s.problem(at, "`.IGNORE` makes make ignore a failing recipe, so a failing go test would pass")
		return
	}
	body, _, hasRecipe := strings.Cut(rest, ";")
	if kind, left, _, _ := classify(body); kind == assignmentLine {
		name := variableName(left)
		if controlNames[name] {
			s.problem(at, "%q sets %s for a target, which changes how make runs its recipe", text, name)
		}
		if ledgerNames[name] {
			s.problem(at, "%q sets %s for a target; `%s:` must read its one global assignment", text, name, ledgerTarget)
		}
		for _, target := range targets {
			if target == ledgerTarget || target == "ledger/%" || strings.Contains(target, "$") {
				s.problem(at, "%q defines a variable for %s, which could reach `%s:`; it takes no target variable", text, target, ledgerTarget)
				break
			}
		}
		for _, target := range targets {
			if reason := matchReason(target); reason != "" {
				s.problem(at, "%q defines a variable for %s, which %s; the `%s:` rule takes no make variable", text, target, reason, goTarget)
				break
			}
		}
		return
	}
	if slices.Contains(targets, goTarget) {
		if text != goTarget+":" {
			s.problem(at, "rule %q names %s; the rule must be exactly `%s:` with no prerequisites and the one recipe `%s`", text, goTarget, goTarget, goRecipe)
			return
		}
		s.rules++
		s.rule, s.recipes, s.ruleAt, s.ruleLine = goTestRule, 0, at, text
		return
	}
	if slices.Contains(targets, ledgerTarget) {
		if text != ledgerTarget+":" {
			s.problem(at, "rule %q names %s; the rule must be exactly `%s:` with the one recipe `%s`", text, ledgerTarget, ledgerTarget, ledgerRecipe)
			return
		}
		s.ledgerRules++
		s.rule, s.ledgerRecipes, s.ledgerAt, s.ruleLine = ledgerRule, 0, at, text
		return
	}
	for _, target := range targets {
		if reason := matchReason(target); reason != "" {
			if hasRecipe {
				s.problem(at, "rule %q has a recipe, and its target %s %s; only the `%s:` rule may give go-test a recipe", text, target, reason, goTarget)
			}
			s.rule, s.ruleLine, s.ruleWord = matchingRule, text, target
			return
		}
	}
}

// include follows literal include names from the repository root.
func (s *scanner) include(at, directive string, names []string, depth int) {
	if depth == 0 {
		s.problem(at, "%s nests deeper than %d files", directive, includeDepth)
		return
	}
	for _, name := range names {
		if strings.ContainsAny(name, "$*?[~") {
			s.problem(at, "%s %s names a file make computes; this check reads only literal include names", directive, name)
			continue
		}
		full := name
		if !filepath.IsAbs(full) {
			full = filepath.Join(s.root, name)
		}
		if _, err := os.Stat(full); err != nil {
			if directive != "include" {
				continue
			}
			s.problem(at, "include %s: %v", name, err)
			continue
		}
		s.scan(name, depth-1)
	}
}

type lineKind int

const (
	otherLine lineKind = iota
	assignmentLine
	ruleLine
)

// classify finds the first assignment operator (=, :=, ::=, +=, ?=, !=) or
// rule colon in a line, as make does: a colon followed by `=` is an
// assignment, any other colon starts a rule.
func classify(text string) (kind lineKind, left, op, right string) {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '=':
			j := i
			for j > 0 && strings.IndexByte(":+?!", text[j-1]) >= 0 {
				j--
			}
			return assignmentLine, text[:j], text[j : i+1], text[i+1:]
		case ':':
			k := i
			for k < len(text) && text[k] == ':' {
				k++
			}
			if k < len(text) && text[k] == '=' {
				return assignmentLine, text[:i], text[i : k+1], text[k+1:]
			}
			return ruleLine, text[:i], text[i:k], text[k:]
		}
	}
	return otherLine, text, "", ""
}

// variableName strips the export, override and private keywords from the
// left side of an assignment.
func variableName(left string) string {
	fields := strings.Fields(left)
	for len(fields) > 1 && (fields[0] == "export" || fields[0] == "override" || fields[0] == "private") {
		fields = fields[1:]
	}
	return strings.Join(fields, " ")
}

// matchReason says why a target word could name go-test, or "" when it
// cannot: the literal name, a `%` pattern go-test fits, or a word make
// expands before matching.
func matchReason(word string) string {
	switch {
	case word == goTarget:
		return "names go-test"
	case strings.Contains(word, "$"):
		return "is expanded by make, so it could name go-test"
	case strings.Contains(word, "%"):
		prefix, suffix, _ := strings.Cut(word, "%")
		if strings.HasPrefix(goTarget, prefix) && strings.HasSuffix(goTarget, suffix) && len(goTarget) > len(prefix)+len(suffix) {
			return "is a pattern go-test matches"
		}
	}
	return ""
}

// noneLanguages returns the languages declared none and a problem for each
// one this gate knows no runners for.
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
func checkGateSteps(root, makefile string, steps, none []string) []string {
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

// checkGoTestTimeouts dry-runs the GATE_STEPS steps and reports each `go test`
// command, split at &&, ;, | and &, that carries no -timeout flag: Go's 600s
// default panics cmd/corvint under load, so every gate-reachable invocation is
// bound by GO_TEST_TIMEOUT (STR-V0-019, decision 0082).
func checkGoTestTimeouts(root, makefile string, steps []string) []string {
	recipe, err := dryRun(root, makefile, steps...)
	if err != nil {
		return nil // reported by the other gate-step checks
	}
	problems := []string{}
	for _, line := range strings.Split(recipe, "\n") {
		for _, command := range strings.FieldsFunc(line, func(r rune) bool { return r == '&' || r == ';' || r == '|' }) {
			words := strings.Fields(command)
			for i := 0; i+1 < len(words); i++ {
				if path.Base(strings.Trim(words[i], "@-()\"'")) != "go" || words[i+1] != "test" {
					continue
				}
				if !slices.ContainsFunc(words[i+2:], func(w string) bool { return w == "-timeout" || strings.HasPrefix(w, "-timeout=") }) {
					problems = append(problems, fmt.Sprintf("make gate runs `go test` without -timeout: %q (pass -timeout $(GO_TEST_TIMEOUT), STR-V0-019)", strings.TrimSpace(command)))
				}
			}
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
