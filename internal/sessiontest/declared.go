package sessiontest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Declared go-test recipe states (STR-V0-003, STR-V0-006).
const (
	// GoTestAbsent: `make go-test` runs no recipe, because the makefile
	// make would read gives go-test no rule, or names it only as a .PHONY
	// prerequisite; the planner reports Go INCOMPLETE rather than run it.
	GoTestAbsent = "absent"
	// GoTestParsed: one typed `[NAME=VALUE...] go test FLAGS ./...` recipe line.
	GoTestParsed = "parsed"
	// GoTestUnparsed: a go-test target this reader cannot type; only
	// `make go-test` reproduces it, so Go runs FULL through it.
	GoTestUnparsed = "unparsed"
)

const (
	maxMakefileBytes = 1 << 20
	maxExpandDepth   = 16
)

// Declared is the repository's declared full scope, read from the makefile
// make would read independently of the planner (STR-V0-003). Languages
// names each non-Go language with a `<language>-test` target.
type Declared struct {
	GoEnv      []string `json:"goEnv"`
	GoFlags    []string `json:"goFlags"`
	GoTest     string   `json:"goTest"`
	GoTestNote string   `json:"goTestNote"`
	Languages  []string `json:"languages"`
	Unreadable string   `json:"unreadable"`
}

// Names reports whether anything full-suite is declared.
func (d Declared) Names() bool { return d.GoTest != GoTestAbsent || len(d.Languages) != 0 }

// Typing is exact by construction: a go-test recipe is typed only when
// every byte that reaches it is inside the grammar below, whose meaning is
// the same to make, to /bin/sh and to the typed argv. Anything else leaves
// go-test untyped, and anything that could define a rule this reader does
// not see leaves the declared scope incomplete.
//
//	recipe   = ["@"] text                 one tab line, no continuation
//	value    = text                       after the operator and its blanks
//	text     = { byte | blank | ref }     no "#", "\", "$" outside a ref
//	ref      = "$(" NAME ")" | "${" NAME "}"
//	byte     = ASCII letter | digit | "-" | "_" | "." | "/" | "=" | ","
//	blank    = " " | "\t"
//
// The expanded recipe is split on blank runs, as sh splits an unquoted
// command line of these bytes. A value is kept verbatim (make strips only
// the blanks after the operator), so its trailing blanks are kept and a
// `#` in it is checked, not stripped.
var (
	makefileNames  = []string{"GNUmakefile", "makefile", "Makefile"}
	makeAssignment = regexp.MustCompile(`^([A-Za-z_.][A-Za-z0-9_.]*)[ \t]*(\?=|::=|:=|=)[ \t]*(.*)$`)
	makeAppend     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[ \t]*(\+=|!=)`)
	makeRule       = regexp.MustCompile(`^([A-Za-z0-9_.%/-]+(?:[ \t]+[A-Za-z0-9_.%/-]+)*)[ \t]*:([^=]|$)`)
	envAssignment  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// goEnvironmentName names the variables that pick or configure the go
	// command; a makefile that assigns one leaves go-test untyped even when
	// the variable is not inherited now.
	goEnvironmentName = regexp.MustCompile(`^(PATH|GO[A-Z0-9]+|CGO_[A-Z0-9_]+)$`)
	// harmlessMakeflag matches the MAKEFLAGS and GNUMAKEFLAGS words that
	// change neither what make reads nor what a recipe runs: silence,
	// keep-going, directory printing, job slots and output sync. Any other
	// word (-e, -n, -R, -f, --eval, a VAR=VALUE override...) is refused.
	harmlessMakeflag = regexp.MustCompile(`^(-?[swk]*(j[0-9]*)?|-?O[a-z]*|--jobserver-(auth|fds)=[^ \t]*|--(no-)?print-directory|--keep-going|--silent|--quiet|--output-sync(=[a-z]+)?)$`)
	// The adapter adds these itself; a recipe that sets them is untyped.
	reservedGoFlags = []string{"-json", "-run", "-skip"}
	// makeControlNames change how make reads the makefile or runs a recipe;
	// a makefile that assigns one leaves go-test untyped.
	makeControlNames = map[string]bool{
		"SHELL": true, "MAKE": true, "MAKEFLAGS": true, "GNUMAKEFLAGS": true, "MFLAGS": true, "MAKEFILES": true,
		"MAKEOVERRIDES": true, "MAKESHELL": true, "MAKELEVEL": true, "MAKECMDGOALS": true, "MAKEFILE_LIST": true,
		"CURDIR": true, "VPATH": true,
	}
	// makeBuiltinNames are GNU make's default variables (default.c, 4.4.1)
	// without a dot, a comma or the MAKE prefix; makeBuiltinVariable covers
	// the rest by shape. A `?=` on one keeps make's default, whose value
	// this reader does not know.
	makeBuiltinNames = map[string]bool{
		"AR": true, "ARFLAGS": true, "AS": true, "CC": true, "CXX": true, "OBJC": true, "CO": true, "COFLAGS": true,
		"CPP": true, "FC": true, "F77": true, "F77FLAGS": true, "GET": true, "LD": true, "LEX": true, "LINT": true,
		"M2C": true, "PC": true, "YACC": true, "MAKEINFO": true, "TEX": true, "TEXI2DVI": true, "WEAVE": true,
		"CWEAVE": true, "TANGLE": true, "CTANGLE": true, "RM": true, "OUTPUT_OPTION": true, "SHELL": true,
		"CURDIR": true, "SUFFIXES": true, "MFLAGS": true, "GNUMAKEFLAGS": true,
	}
	conditionalWords = map[string]bool{"ifeq": true, "ifneq": true, "ifdef": true, "ifndef": true}
	// exportWords change which makefile variables reach a recipe's environment.
	exportWords = map[string]bool{"export": true, "unexport": true, "override": true, ".EXPORT_ALL_VARIABLES": true}
)

// Conditional frame states. Lines are read only while no open frame skips.
const (
	condTaken    = iota // the condition is known true
	condSkipping        // known false so far; a later else may be taken
	condDone            // a branch was taken; the rest is skipped
	condUnknown         // not evaluated; every branch is read conservatively
	condNested          // inside a skipped branch; ignored entirely
)

// ReadDeclared reads the makefile make would read in root: GNUmakefile,
// then makefile, then Makefile, as make tries them. languages are the
// non-Go language names a `<language>-test` target may declare; lookup is
// the invoking environment, which make imports as variables. Typing is
// conservative: anything this reader does not reproduce exactly leaves
// go-test untyped, so it runs only through `make go-test`, and anything
// that could define a rule it does not see leaves the scope incomplete.
func ReadDeclared(root string, languages []string, lookup func(string) (string, bool)) Declared {
	r := &makefileReader{
		lookup:    lookup,
		wanted:    map[string]bool{},
		variables: map[string]string{},
		refused:   map[string]bool{},
		goTest:    GoTestAbsent,
		languages: []string{},
	}
	for _, language := range languages {
		r.wanted[language+"-test"] = true
	}
	r.environment()
	name, raw, problem := readMakefile(root)
	if problem != "" {
		return Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestAbsent, Languages: []string{}, Unreadable: problem}
	}
	if name != "" {
		r.name = name
		for _, line := range logicalLines(string(raw)) {
			r.read(line)
		}
		if r.inDefine {
			r.setUntyped("the " + r.name + " has a define without endef")
		}
		if len(r.stack) != 0 {
			r.setUntyped("the " + r.name + " has a conditional without endif")
		}
	}
	return r.declared()
}

// readMakefile opens the first of make's default makefile names that
// exists, as make does. name is empty when none exists.
func readMakefile(root string) (name string, raw []byte, problem string) {
	for _, candidate := range makefileNames {
		path := filepath.Join(root, candidate)
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return candidate, nil, candidate + " is unreadable"
		}
		file, err := os.Open(path)
		if err != nil {
			return candidate, nil, candidate + " is unreadable"
		}
		defer file.Close()
		raw, err = io.ReadAll(io.LimitReader(file, maxMakefileBytes+1))
		if err != nil || len(raw) > maxMakefileBytes {
			return candidate, nil, fmt.Sprintf("%s is unreadable or exceeds %d bytes", candidate, maxMakefileBytes)
		}
		return candidate, raw, ""
	}
	return "", nil, ""
}

// makefileReader holds one read of a makefile.
type makefileReader struct {
	name      string
	lookup    func(string) (string, bool)
	wanted    map[string]bool
	variables map[string]string
	// refused names variables this reader cannot evaluate: assigned under
	// a conditional, an immediate assignment it cannot expand, or a ?= on a
	// make built-in. A reference to one leaves the recipe untyped.
	refused map[string]bool
	recipe  []logicalLine
	stack   []int
	// goalsUnknown: MAKECMDGOALS came from the environment or a makefile
	// line touched it, so from here on a conditional on it is not
	// evaluated and both branches are read.
	goalsUnknown               bool
	goTestRules                int
	phonyGoTest                bool
	inGoTest, inRule, inDefine bool
	goTest                     string
	languages                  []string
	untyped, incomplete        string
}

func (r *makefileReader) setUntyped(reason string) {
	if r.untyped == "" {
		r.untyped = reason
	}
}

func (r *makefileReader) setIncomplete(reason string) {
	if r.incomplete == "" {
		r.incomplete = reason
	}
}

// environment reads the invoking environment's make controls.
func (r *makefileReader) environment() {
	if _, ok := r.lookup("MAKECMDGOALS"); ok {
		// make then uses the inherited value, so no conditional on it is
		// evaluated for go-test: both branches are read.
		r.goalsUnknown = true
		r.setUntyped("MAKECMDGOALS is in the environment, which make uses instead of the goals")
	}
	for _, name := range []string{"MAKEFLAGS", "GNUMAKEFLAGS"} {
		flags, ok := r.lookup(name)
		if !ok {
			continue
		}
		for _, word := range makeWords(flags) {
			if !harmlessMakeflag.MatchString(word) {
				// An option or override can change which makefile is read
				// and how it runs; -f, -e, -R and -- VAR=VALUE among them.
				r.setIncomplete(name + " carries " + word)
				r.setUntyped(name + " carries " + word + ", which can change what make reads or runs")
				break
			}
		}
	}
	if files, ok := r.lookup("MAKEFILES"); ok && strings.Trim(files, " \t") != "" {
		r.setIncomplete("MAKEFILES names extra makefiles")
		r.setUntyped("MAKEFILES names extra makefiles, which this reader cannot read")
	}
}

// read reads one logical line.
func (r *makefileReader) read(line logicalLine) {
	text := line.text
	if r.inDefine {
		if words := makeWords(text); len(words) > 0 && words[0] == "endef" {
			r.inDefine = false
		}
		return
	}
	skipping := conditionSkipping(r.stack)
	if strings.HasPrefix(text, "\t") {
		body := strings.Trim(text, " \t")
		words := makeWords(body)
		switch {
		case r.inGoTest && !skipping:
			// Every tab line is a recipe line to make, an empty one and a
			// shell comment included.
			r.recipe = append(r.recipe, line)
		case body == "" || strings.HasPrefix(body, "#"):
		case skipping:
			if len(words) > 0 && (conditionalWords[words[0]] || words[0] == "else" || words[0] == "endif") {
				// Whether make nests a tab-prefixed conditional inside a
				// skipped branch depends on the rule context, so the
				// branch structure after it is not known.
				r.setIncomplete(fmt.Sprintf("%s line %d is a tab-prefixed conditional in a skipped branch", r.name, line.number))
				r.setUntyped(fmt.Sprintf("%s line %d is a tab-prefixed conditional in a skipped branch", r.name, line.number))
			}
		case !r.inRule:
			// Outside a rule make reads a tab line as makefile text: a
			// directive or an assignment is read as such (a rule is a
			// make error), so the line is read again without the tab.
			r.setUntyped(fmt.Sprintf("%s line %d is a tab line outside a rule", r.name, line.number))
			r.read(logicalLine{text: body, number: line.number, continued: line.continued})
		}
		return
	}
	// A blank or comment line does not end a recipe; make keeps reading
	// tab lines after it as the same rule's recipe. make accepts leading
	// blanks before a directive or assignment.
	text = strings.TrimLeft(text, " \t")
	if text == "" || text[0] == '#' {
		return
	}
	directive := makeWords(text)[0]
	if strings.Contains(text, "MAKECMDGOALS") && !conditionalWords[directive] && directive != "else" {
		// Any other line that names MAKECMDGOALS can assign it (an
		// assignment, export, override, define or eval), and make
		// evaluates a later conditional against that value.
		r.goalsUnknown = true
	}
	// A conditional does not end a recipe: make keeps reading the rule's
	// tab lines inside it, so one inside the go-test recipe span, or
	// around a go-test rule, leaves go-test untyped.
	switch {
	case conditionalWords[directive]:
		if r.inGoTest {
			r.setUntyped("the go-test recipe contains a conditional")
		}
		if skipping {
			r.stack = append(r.stack, condNested)
		} else {
			r.stack = append(r.stack, r.evaluate(text))
		}
		return
	case directive == "else" || directive == "endif":
		r.conditionalEnd(line.number, directive, text)
		return
	case directive == "define":
		// A define body is skipped up to endef; what it holds can reach
		// a recipe through a reference, so go-test is untyped.
		r.inDefine = true
		r.setUntyped("the " + r.name + " uses define, which this reader cannot evaluate")
		return
	case directive == "endef":
		r.setUntyped(fmt.Sprintf("%s line %d has an endef without define", r.name, line.number))
		return
	}
	if skipping {
		return
	}
	r.inGoTest, r.inRule = false, false
	if strings.Contains(text, "$(eval") || strings.Contains(text, "${eval") {
		// eval can define any rule, a <lang>-test included.
		r.setIncomplete("the " + r.name + " uses eval")
		r.setUntyped("the " + r.name + " uses eval, which this reader cannot evaluate")
		return
	}
	switch directive {
	case "export", "unexport", "override", "include", "-include", "sinclude", ".EXPORT_ALL_VARIABLES:":
		if strings.HasSuffix(directive, "include") {
			// An included file can declare any full-suite target.
			r.setIncomplete("the " + r.name + " uses " + directive)
		}
		r.setUntyped("the " + r.name + " uses " + strings.TrimSuffix(directive, ":") + ", which this reader cannot evaluate")
		return
	}
	// export, unexport, override and .EXPORT_ALL_VARIABLES in any other
	// position, a target-specific one included, change a recipe's
	// environment.
	for _, word := range strings.FieldsFunc(stripMakeComment(text), func(r rune) bool {
		return strings.ContainsRune(" \t:=;,()", r)
	}) {
		if exportWords[word] {
			r.setUntyped("the " + r.name + " uses " + word + ", which this reader cannot evaluate")
		}
	}
	if makeAppend.MatchString(text) {
		r.setUntyped("the " + r.name + " uses += or !=, which this reader cannot evaluate")
		return
	}
	if match := makeAssignment.FindStringSubmatch(text); match != nil {
		r.assign(match[1], match[2], match[3])
		return
	}
	if targets, _, found := strings.Cut(text, ":"); found && strings.Contains(targets, "$") {
		// A computed target can name go-test or any <lang>-test, and add
		// prerequisites to it.
		r.setIncomplete(fmt.Sprintf("%s line %d has a rule target that uses $", r.name, line.number))
		r.setUntyped(fmt.Sprintf("%s line %d has a rule target that uses $, which this reader cannot expand", r.name, line.number))
		return
	}
	match := makeRule.FindStringSubmatch(text)
	if match == nil {
		if strings.ContainsAny(text, "$:") {
			// A function call at top level, $(call ...) included, can
			// expand to a rule, and a line with a colon can be a rule
			// whose target names are outside this reader's alphabet.
			r.setIncomplete(fmt.Sprintf("%s line %d is not a construct this reader recognises", r.name, line.number))
		}
		r.setUntyped(fmt.Sprintf("%s line %d is not a construct this reader recognises", r.name, line.number))
		return
	}
	r.rule(text, match[1])
}

// conditionalEnd reads an else or endif line.
func (r *makefileReader) conditionalEnd(number int, directive, text string) {
	if r.inGoTest {
		r.setUntyped("the go-test recipe contains a conditional")
	}
	if len(r.stack) == 0 {
		r.setUntyped(fmt.Sprintf("%s line %d has an unmatched %s", r.name, number, directive))
		return
	}
	top := &r.stack[len(r.stack)-1]
	if directive == "endif" {
		r.stack = r.stack[:len(r.stack)-1]
		return
	}
	rest := strings.TrimLeft(text[len("else"):], " \t")
	words := makeWords(rest)
	conditional := len(words) > 0 && conditionalWords[words[0]]
	if rest != "" && rest[0] != '#' && !conditional {
		// make refuses text after else that is not another conditional.
		r.setUntyped(fmt.Sprintf("%s line %d has text after else", r.name, number))
		return
	}
	switch *top {
	case condTaken:
		*top = condDone
	case condSkipping:
		*top = condTaken
		if conditional {
			*top = r.evaluate(rest)
		}
	}
}

// assign reads one variable assignment.
func (r *makefileReader) assign(name, operator, value string) {
	switch name {
	case "MAKEFLAGS", "GNUMAKEFLAGS", "MAKEFILES", ".RECIPEPREFIX":
		// make re-reads options assigned to MAKEFLAGS, and the recipe
		// prefix decides which lines are makefile text, so what make
		// reads after this line is not known.
		r.setIncomplete("the " + r.name + " assigns " + name)
	}
	if makeControlNames[name] || (strings.HasPrefix(name, ".") && name != ".DEFAULT_GOAL") {
		r.setUntyped("the " + r.name + " assigns " + name + ", which changes how make runs a recipe")
	}
	if _, inherited := r.lookup(name); operator != "?=" && (inherited || goEnvironmentName.MatchString(name)) {
		// make exports a variable that came from the environment with
		// its makefile value, so the recipe sees that value.
		r.setUntyped("the " + r.name + " assigns " + name + ", which make passes to the go-test recipe from the environment")
	}
	if len(r.stack) > 0 {
		r.refused[name] = true
	}
	switch operator {
	case "?=":
		// ?= assigns only an undefined variable: an earlier makefile
		// assignment wins, then the environment, then make's built-in
		// default, whose value this reader does not know.
		if _, defined := r.variables[name]; defined {
			return
		}
		if environment, ok := r.lookup(name); ok {
			value = environment
		} else if makeBuiltinVariable(name) {
			r.refused[name] = true
		}
	case ":=", "::=":
		// An immediate assignment expands now, against the values
		// assigned so far; a later reassignment must not change it.
		if strings.Contains(value, "$") {
			expanded, err := expandMake(value, r.resolve, 0)
			if err != nil {
				r.refused[name] = true
			} else {
				value = expanded
			}
		}
	}
	r.variables[name] = value
}

// rule reads one rule line; a rule may name several targets.
func (r *makefileReader) rule(text, targets string) {
	r.inRule = true
	for _, target := range makeWords(targets) {
		// make reads ./go-test as go-test.
		for strings.HasPrefix(target, "./") && len(target) > 2 {
			target = target[2:]
		}
		switch {
		case target == "go-test":
			r.goTest = GoTestUnparsed
			r.inGoTest = true
			r.goTestRules++
			if r.goTestRules > 1 {
				// A second rule with a recipe replaces the first recipe.
				r.setUntyped("go-test has more than one rule")
			}
			if len(r.stack) > 0 {
				r.setUntyped("go-test is defined under a conditional")
			}
			// Prerequisites, target-specific variables and a
			// double-colon rule all change what `make go-test` runs.
			if _, after, _ := strings.Cut(text, ":"); strings.Trim(stripMakeComment(after), " \t") != "" {
				r.setUntyped("the go-test rule line has prerequisites or target-specific variables")
			}
		case r.wanted[target]:
			r.languages = appendUnique(r.languages, strings.TrimSuffix(target, "-test"))
		case target == ".DEFAULT":
			// .DEFAULT answers every target make has no rule for.
			r.setIncomplete("the " + r.name + " has a .DEFAULT rule")
			r.setUntyped("the " + r.name + " has a .DEFAULT rule, which answers any target")
		case target == ".PHONY":
			if _, after, _ := strings.Cut(text, ":"); r.goTestRules == 0 {
				for _, prerequisite := range makeWords(stripMakeComment(after)) {
					if prerequisite == "go-test" {
						r.phonyGoTest = true
					}
				}
			}
		case strings.HasPrefix(target, "."):
			r.setUntyped("the " + r.name + " names the special target " + target + ", which this reader cannot evaluate")
		case strings.Contains(target, "%"):
			// A pattern rule serves every target it matches, so each one
			// is declared and go-test is untyped.
			if patternMatches(target, "go-test") {
				if r.goTest == GoTestAbsent {
					r.goTest = GoTestUnparsed
				}
				r.setUntyped("the pattern rule " + target + " applies to go-test")
			}
			for want := range r.wanted {
				if patternMatches(target, want) {
					r.languages = appendUnique(r.languages, strings.TrimSuffix(want, "-test"))
				}
			}
		}
	}
}

// resolve answers a reference from the makefile's own variables; a refused
// name is undefined, so a reference to it leaves the recipe untyped.
func (r *makefileReader) resolve(name string) (string, bool) {
	if r.refused[name] {
		return "", false
	}
	value, ok := r.variables[name]
	return value, ok
}

// evaluate evaluates a conditional line within the exact subset.
func (r *makefileReader) evaluate(line string) int {
	return conditionEvaluator{goalsUnknown: r.goalsUnknown}.conditional(line)
}

func (r *makefileReader) declared() Declared {
	declared := Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: r.goTest, Languages: r.languages}
	sort.Strings(declared.Languages)
	if r.incomplete != "" {
		declared.Unreadable = r.incomplete + ", so its declared full scope cannot be read completely"
	}
	if declared.GoTest == GoTestUnparsed {
		env, flags, note := typeGoRecipe(r.recipe, r.resolve)
		if r.untyped != "" {
			note = r.untyped
		}
		if note == "" {
			declared.GoTest, declared.GoEnv, declared.GoFlags = GoTestParsed, env, flags
		}
		declared.GoTestNote = note
	}
	if declared.GoTest == GoTestAbsent && r.phonyGoTest {
		declared.GoTestNote = ".PHONY names go-test but no rule gives it a recipe, so make go-test runs nothing"
	}
	return declared
}

// makeBuiltinVariable reports whether make defines name itself: a listed
// default, a MAKE* or dotted special variable, or any name with a dot or a
// comma, the shape of the implicit-rule defaults such as LINK.o.
func makeBuiltinVariable(name string) bool {
	return makeBuiltinNames[name] || strings.HasPrefix(name, "MAKE") || strings.ContainsAny(name, ".,")
}

// conditionSkipping reports whether an open conditional frame skips lines.
func conditionSkipping(stack []int) bool {
	for _, state := range stack {
		if state == condSkipping || state == condDone || state == condNested {
			return true
		}
	}
	return false
}

// makeWords splits on ASCII blanks only, as make does in the C locale.
func makeWords(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == '\t' })
}

// conditionEvaluator evaluates an `ifeq (A,B)` or `ifneq (A,B)` line only
// within a subset that is exact by construction for `make go-test`: literal
// words of ASCII letters, digits and -_./ separated by single spaces,
// $(MAKECMDGOALS) as go-test unless it is unknown, $(filter LITERALS,X)
// without %, and $(or X,...). It reads no makefile or environment variable.
// Anything else, another reference, %, a tab, doubled or surrounding
// blanks, a comment or a non-ASCII byte, is condUnknown, so every branch is
// read.
type conditionEvaluator struct {
	goalsUnknown bool
}

func (e conditionEvaluator) conditional(line string) int {
	word, rest := "", ""
	switch {
	case strings.HasPrefix(line, "ifeq"):
		word, rest = "ifeq", line[len("ifeq"):]
	case strings.HasPrefix(line, "ifneq"):
		word, rest = "ifneq", line[len("ifneq"):]
	default:
		return condUnknown
	}
	rest = strings.TrimPrefix(rest, " ")
	if !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return condUnknown
	}
	if !subsetText(rest) {
		return condUnknown
	}
	args, ok := splitMakeArgs(rest[1 : len(rest)-1])
	if !ok || len(args) != 2 {
		return condUnknown
	}
	values := make([]string, 2)
	for index, arg := range args {
		value, err := e.operand(arg)
		if err != nil {
			return condUnknown
		}
		values[index] = value
	}
	if (values[0] == values[1]) == (word == "ifeq") {
		return condTaken
	}
	return condSkipping
}

// subsetText checks the whole conditional text before anything is evaluated:
// only literal bytes, single spaces, commas, `$(` and balanced parentheses.
// Braces, `$$`, a `$` not opening a reference and anything else are outside
// the subset, wherever they appear.
func subsetText(value string) bool {
	depth := 0
	for index := 0; index < len(value); index++ {
		switch c := value[index]; {
		case literalByte(c) || c == ' ' || c == ',':
		case c == '$':
			if index+1 >= len(value) || value[index+1] != '(' {
				return false
			}
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return false
			}
		default:
			return false
		}
	}
	return depth == 0
}

// splitMakeArgs splits at top-level commas; ok is false when the parentheses
// do not balance. Braces never reach it: subsetText refuses them.
func splitMakeArgs(value string) (args []string, ok bool) {
	depth, start := 0, 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, false
			}
		case ',':
			if depth == 0 {
				args = append(args, value[start:index])
				start = index + 1
			}
		}
	}
	return append(args, value[start:]), depth == 0
}

// cleanWords reports whether value is words separated by single spaces, with
// no surrounding blank; the empty string is clean.
func cleanWords(value string) bool {
	return value == "" || (!strings.HasPrefix(value, " ") && !strings.HasSuffix(value, " ") && !strings.Contains(value, "  "))
}

// literalByte is the literal alphabet of the evaluated conditional subset.
func literalByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-_./", c) >= 0
}

// recipeByte is the literal alphabet of a typed recipe or value: a byte that
// make, sh and go all read as itself.
func recipeByte(c byte) bool {
	return literalByte(c) || c == '=' || c == ','
}

// operand evaluates one operand or function argument of the subset; both
// the source and its value must be clean words.
func (e conditionEvaluator) operand(value string) (string, error) {
	if !cleanWords(value) {
		return "", errors.New("blanks outside single separators")
	}
	var out strings.Builder
	for index := 0; index < len(value); {
		switch c := value[index]; {
		case literalByte(c) || c == ' ':
			out.WriteByte(c)
			index++
			continue
		case c != '$' || index+1 >= len(value) || value[index+1] != '(':
			return "", errors.New("outside the evaluated subset")
		}
		end, nesting := -1, 0
		for scan := index + 1; scan < len(value) && end < 0; scan++ {
			switch value[scan] {
			case '(':
				nesting++
			case ')':
				nesting--
				if nesting == 0 {
					end = scan
				}
			}
		}
		if end < 0 {
			return "", errors.New("an unterminated reference")
		}
		expanded, err := e.function(value[index+2 : end])
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		index = end + 1
	}
	if result := out.String(); cleanWords(result) {
		return result, nil
	}
	return "", errors.New("a value with blanks outside single separators")
}

func (e conditionEvaluator) function(body string) (string, error) {
	if body == "MAKECMDGOALS" {
		if e.goalsUnknown {
			return "", errors.New("MAKECMDGOALS is inherited or assigned")
		}
		return "go-test", nil
	}
	function, arguments, found := strings.Cut(body, " ")
	if !found {
		return "", errors.New("a reference outside the evaluated subset")
	}
	args, ok := splitMakeArgs(arguments)
	if !ok {
		return "", errors.New("unbalanced function arguments")
	}
	switch function {
	case "filter":
		if len(args) != 2 || !cleanWords(args[0]) {
			return "", errors.New("filter takes literal patterns and one text")
		}
		patterns := map[string]bool{}
		for _, pattern := range strings.Split(args[0], " ") {
			for index := 0; index < len(pattern); index++ {
				if !literalByte(pattern[index]) {
					return "", errors.New("a filter pattern that is not a literal word")
				}
			}
			patterns[pattern] = pattern != ""
		}
		text, err := e.operand(args[1])
		if err != nil {
			return "", err
		}
		var kept []string
		for _, word := range makeWords(text) {
			if patterns[word] {
				kept = append(kept, word)
			}
		}
		return strings.Join(kept, " "), nil
	case "or":
		// Every argument is checked, even one make would never expand,
		// so the result never rests on text outside the subset.
		result := ""
		for _, arg := range args {
			value, err := e.operand(arg)
			if err != nil {
				return "", err
			}
			if result == "" {
				result = value
			}
		}
		return result, nil
	}
	return "", fmt.Errorf("make function %s is outside the evaluated subset", function)
}

// patternMatches reports whether a make target pattern with one % stem
// matches name.
func patternMatches(pattern, name string) bool {
	prefix, suffix, ok := strings.Cut(pattern, "%")
	return ok && !strings.Contains(suffix, "%") && len(name) > len(prefix)+len(suffix) &&
		strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix)
}

// logicalLine is one line as make reads it, with its first physical line.
type logicalLine struct {
	text      string
	number    int
	continued bool // joined from a backslash-newline continuation
}

// logicalLines joins backslash-newline continuations. A line continues when
// it ends in an odd number of backslashes, as in make. Outside a recipe
// make condenses the backslash-newline and the blanks around it into one
// space; in a recipe it keeps them for the shell, which joins the pieces
// without a blank, so a continued go-test recipe line is untyped. A CR is
// kept: it is a byte outside the grammar, not a line end.
func logicalLines(body string) []logicalLine {
	var lines []logicalLine
	current, open := logicalLine{}, false
	for index, line := range strings.Split(body, "\n") {
		if !open {
			current, open = logicalLine{number: index + 1}, true
		}
		if current.continued {
			line = strings.TrimLeft(line, " \t")
		}
		if trailingBackslashes(line)%2 == 1 {
			current.text += strings.TrimRight(line[:len(line)-1], " \t") + " "
			current.continued = true
			continue
		}
		current.text += line
		lines = append(lines, current)
		open = false
	}
	if open {
		lines = append(lines, current)
	}
	return lines
}

func trailingBackslashes(line string) int {
	count := 0
	for count < len(line) && line[len(line)-1-count] == '\\' {
		count++
	}
	return count
}

func stripMakeComment(value string) string {
	if index := strings.Index(value, "#"); index >= 0 {
		return value[:index]
	}
	return value
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// typeGoRecipe types the one go-test recipe line, or names why it cannot.
func typeGoRecipe(recipe []logicalLine, resolve func(string) (string, bool)) (env, flags []string, note string) {
	if len(recipe) != 1 {
		return nil, nil, fmt.Sprintf("go-test has %d recipe lines, want exactly one", len(recipe))
	}
	line := recipe[0]
	if line.continued {
		return nil, nil, "go-test recipe continues over a backslash-newline, which the shell joins"
	}
	// After the tab make skips blanks; one @ silences the echo, while - and
	// + change how make runs the line and do not fit the shape below.
	body := strings.TrimLeft(line.text, " \t")
	if strings.HasPrefix(body, "@") {
		body = strings.TrimLeft(body[1:], " \t")
	}
	expanded, err := expandMake(body, resolve, 0)
	if err != nil {
		return nil, nil, err.Error()
	}
	if strings.HasPrefix(strings.TrimLeft(expanded, " \t"), "PATH=") {
		// The runner is resolved on Corvint's PATH, so a recipe that picks
		// another go cannot be reproduced by the typed argv.
		return nil, nil, "go-test recipe sets PATH, which changes which go runs"
	}
	for index := 0; index < len(expanded); index++ {
		if c := expanded[index]; !recipeByte(c) && c != ' ' && c != '\t' {
			// Outside the alphabet a byte can mean something to the shell,
			// so the line needs a shell to reproduce.
			return nil, nil, fmt.Sprintf("go-test recipe has %q, which is outside the typed alphabet and needs a shell", c)
		}
	}
	fields := makeWords(expanded)
	env = []string{}
	for len(fields) > 0 && envAssignment.MatchString(fields[0]) {
		if strings.HasPrefix(fields[0], "PATH=") {
			return nil, nil, "go-test recipe sets PATH, which changes which go runs"
		}
		env = append(env, fields[0])
		fields = fields[1:]
	}
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "test" || fields[len(fields)-1] != "./..." {
		return nil, nil, "go-test recipe is not `[NAME=VALUE...] go test FLAGS ./...`"
	}
	flags = append([]string{}, fields[2:len(fields)-1]...)
	for _, flag := range flags {
		if flag == "./..." {
			return nil, nil, "go-test recipe names ./... twice"
		}
		for _, reserved := range reservedGoFlags {
			if flag == reserved || strings.HasPrefix(flag, reserved+"=") {
				return nil, nil, "go-test recipe sets " + reserved + ", which the adapter owns"
			}
		}
	}
	if len(flags) != 0 && !strings.HasPrefix(flags[0], "-") {
		return nil, nil, "go-test recipe names a target before its flags"
	}
	return env, flags, ""
}

// expandMake expands `$(NAME)` and `${NAME}` references, with the closer
// matching the opener, through resolve. Every other `$`, `$$`, an automatic
// variable, a function, a substitution reference or a mismatched delimiter,
// is outside the grammar, as is an undefined name.
func expandMake(value string, resolve func(string) (string, bool), depth int) (string, error) {
	if depth > maxExpandDepth {
		return "", errors.New("go-test recipe variables nest too deeply")
	}
	var out strings.Builder
	for index := 0; index < len(value); {
		if value[index] != '$' {
			out.WriteByte(value[index])
			index++
			continue
		}
		var closer byte
		switch {
		case index+1 < len(value) && value[index+1] == '(':
			closer = ')'
		case index+1 < len(value) && value[index+1] == '{':
			closer = '}'
		default:
			return "", errors.New("go-test recipe uses a $ that is not a $(NAME) or ${NAME} reference")
		}
		start, end := index+2, index+2
		for end < len(value) && referenceNameByte(value[end], end == start) {
			end++
		}
		if end == start || end >= len(value) || value[end] != closer {
			return "", errors.New("go-test recipe uses a reference that is not $(NAME) or ${NAME}")
		}
		name := value[start:end]
		definition, defined := resolve(name)
		if !defined {
			return "", fmt.Errorf("go-test recipe names undefined variable %s", name)
		}
		expanded, err := expandMake(definition, resolve, depth+1)
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		index = end + 1
	}
	return out.String(), nil
}

func referenceNameByte(c byte, first bool) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || (!first && c >= '0' && c <= '9')
}
