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

// Typing is exact by construction: the whole makefile must match the small
// grammar below, which is the grammar the repository Makefile uses, and a
// go-test recipe is typed only when every byte that reaches it has the same
// meaning to make, to /bin/sh and to the typed argv. A line outside the
// grammar leaves the declared scope incomplete and go-test untyped; absence
// is never claimed over one. Nothing the repository Makefile does not need
// is read: define, export, unexport, override, include, vpath, undefine,
// `+=`, `!=`, `::=`, computed targets, target-specific variables, rule-line
// recipes, double-colon rules and special targets other than .PHONY are all
// outside the grammar.
//
//	makefile  = { blank | comment | conditional | assignment | rule | recipe }
//	conditional = ("ifeq" | "ifneq") " (" A "," B ")" | ("ifdef" | "ifndef") NAME
//	            | "else" [conditional] | "endif"      ifdef is not evaluated
//	assignment = NAME blanks ("=" | "?=" | ":=") blanks value
//	NAME      = [A-Za-z_][A-Za-z0-9_]* | ".DEFAULT_GOAL"   not a make control
//	rule      = targets ":" prerequisites              no "=", ";" or ":" after
//	targets   = word { blanks word }                    no "$", no leading "."
//	          | ".PHONY"
//	word      = [A-Za-z0-9_./%-]+                       at most one "%"
//	prerequisites = { word | "|" | ref }
//	recipe    = TAB text                                only after a rule line
//	typed recipe = ["@"] text                           one line, no continuation
//	value     = text                                    verbatim after the operator
//	text      = { byte | blank | ref }                  no "#", "\", "$" outside a ref
//	ref       = "$(" NAME ")" | "${" NAME "}"
//	byte      = ASCII letter | digit | "-" | "_" | "." | "/" | "=" | ","
//	blank     = " " | "\t"
//
// Every makefile line is printable ASCII. A "#" starts a comment anywhere in
// a line: make cuts it before it reads the line and keeps the blank before it
// in a value, so this reader cuts it the same way; "\#" is outside the
// grammar. A line make expands as it reads it (a conditional, a rule line, an
// immediate assignment) may hold only $(NAME) references, whose assigned and
// inherited texts hold the same, and the pure functions in pureFunctions;
// eval, call, shell and the rest leave the scope incomplete. A pattern rule
// is read only when its pattern can serve neither go-test nor a source of it
// nor the makefile; a rule for the makefile or for a go-test.<suffix> source,
// such a source or an RCS or SCCS entry at the repository root, a root entry
// outside ASCII, and an inherited make control (MAKEFILES, VPATH, GPATH, a
// MAKEFLAGS word outside the harmless set) all leave the scope incomplete. A
// root entry whose case-folded name is go-test leaves go-test untyped.
//
// The expanded recipe is split on blank runs, as sh splits an unquoted
// command line of these bytes. A value is kept verbatim (make strips only
// the blanks after the operator), so its trailing blanks are kept.
var (
	makefileNames  = []string{"GNUmakefile", "makefile", "Makefile"}
	makeAssignment = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*|\.DEFAULT_GOAL)[ \t]*(\?=|:=|=)[ \t]*(.*)$`)
	// makeOtherAssignment names the assignment operators outside the grammar.
	makeOtherAssignment = regexp.MustCompile(`^[A-Za-z_.][A-Za-z0-9_.]*[ \t]*(\+=|!=|::=)`)
	makeRule            = regexp.MustCompile(`^([A-Za-z0-9_.%/-]+(?:[ \t]+[A-Za-z0-9_.%/-]+)*)[ \t]*:(.*)$`)
	envAssignment       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// goEnvironmentName names the variables that pick or configure the go
	// command; a makefile that assigns one leaves go-test untyped even when
	// the variable is not inherited now.
	goEnvironmentName = regexp.MustCompile(`^(PATH|GO[A-Z0-9]+|CGO_[A-Z0-9_]+)$`)
	// harmlessMakeflag matches exactly the MAKEFLAGS and GNUMAKEFLAGS words
	// that change neither what make reads nor what a recipe runs: keep-going,
	// silence, directory printing, job slots, a jobserver and output sync.
	// Every other word (-e, -n, -r, -R, -f, -I, --eval, a VAR=VALUE override,
	// a malformed jobserver...) is refused.
	harmlessMakeflag = regexp.MustCompile(`^(-?[ksw]+(j[0-9]*)?|-?j[0-9]*|-O(none|line|target|recurse)?|--jobs(=[0-9]+)?|--jobserver-auth=([0-9]+,[0-9]+|fifo:[A-Za-z0-9_./-]+)|--jobserver-fds=[0-9]+,[0-9]+|--(no-)?print-directory|--keep-going|--silent|--quiet|--output-sync(=(none|line|target|recurse))?)$`)
	// The adapter adds these itself; a recipe that sets them is untyped.
	reservedGoFlags = []string{"-json", "-run", "-skip"}
	// makeControlNames change how make reads the makefile or runs a recipe;
	// an assignment to one is outside the grammar.
	makeControlNames = map[string]bool{
		"SHELL": true, "MAKE": true, "MAKEFLAGS": true, "GNUMAKEFLAGS": true, "MFLAGS": true, "MAKEFILES": true,
		"MAKEOVERRIDES": true, "MAKESHELL": true, "MAKELEVEL": true, "MAKECMDGOALS": true, "MAKEFILE_LIST": true,
		"CURDIR": true, "VPATH": true, "GPATH": true, "SUFFIXES": true, "IFS": true,
	}
	// inheritedControls are the environment variables whose presence, with
	// any value, changes what make reads, where it finds targets, whether a
	// recipe needs a shell, or how that shell starts and finds go: make
	// expands $(IFS) and $(MAKEFILES) itself, an empty .SHELLFLAGS drops -c,
	// and bash (which /bin/sh is here) imports SHELLOPTS, BASHOPTS,
	// EXECIGNORE, ENV, BASH_ENV and exported functions named go. One that is
	// present leaves the scope incomplete. MAKECMDGOALS and the MAKEFLAGS
	// words are read separately; SHELL is never taken from the environment
	// by make.
	inheritedControls = []string{
		"MAKEFILES", "VPATH", "GPATH", "MAKEOVERRIDES", ".RECIPEPREFIX", ".SHELLFLAGS", ".LIBPATTERNS", ".EXTRA_PREREQS",
		"IFS", "SHELLOPTS", "BASHOPTS", "EXECIGNORE", "ENV", "BASH_ENV", "BASH_FUNC_go%%", "BASH_FUNC_go()", "go",
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
	// directiveWords are the directives outside the grammar; each is refused
	// as a line's first word.
	directiveWords = map[string]bool{
		"define": true, "endef": true, "undefine": true, "export": true, "unexport": true, "override": true,
		"private": true, "include": true, "-include": true, "sinclude": true, "vpath": true, "load": true, "-load": true,
	}
	// exportWords change which makefile variables reach a recipe's
	// environment, in any position of a line.
	exportWords = map[string]bool{"export": true, "unexport": true, "override": true, ".EXPORT_ALL_VARIABLES": true}
	// pureFunctions are the make functions that only compute a string from
	// their arguments. eval and call define rules and variables, shell runs
	// a process, error stops make, and foreach, value, origin, flavor, info,
	// warning and the rest are left out rather than reasoned about.
	pureFunctions = map[string]bool{
		"subst": true, "patsubst": true, "strip": true, "findstring": true, "filter": true, "filter-out": true,
		"sort": true, "word": true, "wordlist": true, "words": true, "firstword": true, "lastword": true,
		"dir": true, "notdir": true, "suffix": true, "basename": true, "addsuffix": true, "addprefix": true,
		"join": true, "wildcard": true, "realpath": true, "abspath": true, "if": true, "or": true, "and": true,
	}
	// functionArity is the fewest arguments each pure function takes; make
	// stops reading with a fatal error below it. word and wordlist also
	// stop on a non-numeric or zero index, so those arguments must be
	// literal numbers (functionIndex, functionCount).
	functionArity = map[string]int{
		"subst": 3, "patsubst": 3, "strip": 1, "findstring": 2, "filter": 2, "filter-out": 2,
		"sort": 1, "word": 2, "wordlist": 3, "words": 1, "firstword": 1, "lastword": 1,
		"dir": 1, "notdir": 1, "suffix": 1, "basename": 1, "addsuffix": 2, "addprefix": 2,
		"join": 2, "wildcard": 1, "realpath": 1, "abspath": 1, "if": 2, "or": 1, "and": 1,
	}
	functionIndex = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)
	functionCount = regexp.MustCompile(`^[0-9]{1,9}$`)
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
		texts:     map[string][]string{},
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
	// make answers `make go-test` from its built-in implicit rules even
	// without a makefile, so the root is listed in every case.
	r.name = name
	r.listRoot(root)
	if name != "" {
		for _, line := range logicalLines(string(raw)) {
			r.read(line)
		}
		// make stops reading on either, so it runs nothing.
		if r.inDefine {
			r.uncertain("the " + r.name + " has a define without endef")
		}
		if len(r.stack) != 0 {
			r.uncertain("the " + r.name + " has a conditional without endif")
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
	// texts holds every text the makefile gives a name, whatever the
	// operator or branch, so pure can follow a reference to it.
	texts  map[string][]string
	recipe []logicalLine
	stack  []int
	// elseSeen parallels stack: a plain else was read for that frame, so
	// make stops on any further else.
	elseSeen []bool
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

// uncertain records input from which make may read or build something this
// reader does not see: the scope is incomplete and go-test untyped.
func (r *makefileReader) uncertain(reason string) {
	r.setIncomplete(reason)
	r.setUntyped(reason)
}

// outside records a makefile line outside the grammar.
func (r *makefileReader) outside(number int, what string) {
	r.uncertain(fmt.Sprintf("%s line %d %s, which is outside the grammar this reader reads", r.name, number, what))
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
				r.uncertain(name + " carries " + word + ", which is outside the harmless words this reader admits")
				break
			}
		}
	}
	for _, name := range inheritedControls {
		if _, ok := r.lookup(name); ok {
			r.uncertain(name + " is in the environment, which changes what make reads, where it finds targets or how the recipe shell starts")
		}
	}
}

// read reads one logical line.
func (r *makefileReader) read(line logicalLine) {
	text := line.text
	if r.inDefine {
		// The body up to endef is makefile text to this reader only in
		// that it is not read as lines; the define line already left the
		// scope incomplete.
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
		case r.inRule:
			// Another rule's recipe line, or one make skips.
		case body == "" || strings.HasPrefix(body, "#"):
		case skipping:
			if len(words) > 0 && (conditionalWords[words[0]] || words[0] == "else" || words[0] == "endif") {
				// Whether make nests a tab-prefixed conditional inside a
				// skipped branch depends on the rule context, so the
				// branch structure after it is not known.
				r.outside(line.number, "is a tab-prefixed conditional in a skipped branch")
			}
		default:
			// Outside a rule make reads a tab line as makefile text.
			r.outside(line.number, "is a tab line outside a rule")
		}
		return
	}
	// A blank or comment line does not end a recipe; make keeps reading
	// tab lines after it as the same rule's recipe. make accepts leading
	// blanks before a directive or assignment.
	text = strings.TrimLeft(text, " \t")
	if strings.Contains(text, `\#`) {
		// make reads \# as a literal #; what the rest of the line then
		// means is not read.
		r.outside(line.number, `has \#`)
		return
	}
	// A # starts a comment anywhere in a makefile line; make cuts it before
	// it reads the line, so `else# c` is an else and a value keeps the blank
	// before the #.
	if index := strings.IndexByte(text, '#'); index >= 0 {
		text = text[:index]
	}
	if strings.Trim(text, " \t") == "" {
		return
	}
	for index := 0; index < len(text); index++ {
		if c := text[index]; (c < ' ' || c > '~') && c != '\t' {
			r.outside(line.number, fmt.Sprintf("has %q, which is outside the typed alphabet", c))
			return
		}
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
		r.elseSeen = append(r.elseSeen, false)
		if !conditionalSyntax(directive, text[len(directive):]) {
			// make stops reading on a malformed conditional.
			r.outside(line.number, "has a malformed "+directive)
			r.stack = append(r.stack, condUnknown)
		} else if skipping {
			r.stack = append(r.stack, condNested)
		} else {
			r.expandsNow(line.number, text)
			r.stack = append(r.stack, r.evaluate(text))
		}
		return
	case directive == "else" || directive == "endif":
		r.conditionalEnd(line.number, directive, text)
		return
	case directive == "define":
		// make skips the body up to endef even in a skipped branch.
		r.inDefine = true
		r.outside(line.number, "uses define")
		return
	case directive == "endef":
		r.outside(line.number, "has an endef without define")
		return
	}
	if skipping {
		return
	}
	r.inGoTest, r.inRule = false, false
	if directiveWords[directive] {
		r.outside(line.number, "uses "+directive)
		return
	}
	// export, unexport, override and .EXPORT_ALL_VARIABLES in any other
	// position, a target-specific one included, change a recipe's
	// environment.
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return strings.ContainsRune(" \t:=;,()", r)
	}) {
		if exportWords[word] {
			r.outside(line.number, "uses "+word)
			return
		}
	}
	if match := makeOtherAssignment.FindStringSubmatch(text); match != nil {
		// += expands now when the variable was assigned with :=, != runs a
		// shell now, and ::= is a rule line to GNU make 3.81.
		r.outside(line.number, "uses "+match[1])
		return
	}
	if match := makeAssignment.FindStringSubmatch(text); match != nil {
		r.assign(line.number, match[1], match[2], match[3])
		return
	}
	if targets, _, found := strings.Cut(text, ":"); found && strings.Contains(targets, "$") {
		// A computed target can name go-test or any <lang>-test, and add
		// prerequisites to it.
		r.outside(line.number, "has a rule target that uses $")
		return
	}
	match := makeRule.FindStringSubmatch(text)
	if match == nil {
		r.outside(line.number, "is not a construct this reader reads")
		return
	}
	// make expands a rule line, prerequisites included, as it reads it.
	r.expandsNow(line.number, text)
	r.rule(line.number, match[1], match[2])
}

// expandsNow checks text that make expands as it reads the line: when the
// expansion could do more than yield a string, what make reads afterwards is
// not known, so the scope is incomplete and go-test untyped.
func (r *makefileReader) expandsNow(number int, text string) {
	if err := r.pure(text, 0); err != nil {
		r.setIncomplete(fmt.Sprintf("%s line %d expands %s now", r.name, number, err))
		r.setUntyped(fmt.Sprintf("%s line %d expands %s now, which this reader cannot follow", r.name, number, err))
	}
}

// conditionalEnd reads an else or endif line.
func (r *makefileReader) conditionalEnd(number int, directive, text string) {
	if r.inGoTest {
		r.setUntyped("the go-test recipe contains a conditional")
	}
	if len(r.stack) == 0 {
		// make stops reading on an extraneous else or endif.
		r.outside(number, "has an unmatched "+directive)
		return
	}
	top := &r.stack[len(r.stack)-1]
	if directive == "endif" {
		if strings.Trim(text[len("endif"):], " \t") != "" {
			// make stops reading on text after endif.
			r.outside(number, "has text after endif")
		}
		r.stack = r.stack[:len(r.stack)-1]
		r.elseSeen = r.elseSeen[:len(r.elseSeen)-1]
		return
	}
	seen := &r.elseSeen[len(r.elseSeen)-1]
	if *seen {
		// make allows one plain else per conditional and stops on another.
		r.outside(number, "has a second else")
		return
	}
	rest := strings.TrimLeft(text[len("else"):], " \t")
	words := makeWords(rest)
	conditional := len(words) > 0 && conditionalWords[words[0]]
	if rest != "" && !conditional {
		// make stops reading on text after else that is not a conditional.
		r.outside(number, "has text after else")
		return
	}
	if !conditional {
		*seen = true
	} else if !conditionalSyntax(words[0], rest[len(words[0]):]) {
		r.outside(number, "has a malformed else "+words[0])
		*top = condUnknown
		return
	}
	switch *top {
	case condTaken:
		*top = condDone
	case condSkipping:
		*top = condTaken
		if conditional {
			if !conditionSkipping(r.stack[:len(r.stack)-1]) {
				r.expandsNow(number, rest)
			}
			*top = r.evaluate(rest)
		}
	}
}

// conditionalSyntax reports whether rest, the text after an ifeq, ifneq,
// ifdef or ifndef word, has the one shape of each that this reader reads:
// `ifdef NAME` with exactly one word, or `ifeq (A,B)` with balanced
// parentheses, a top-level comma and nothing after the closing parenthesis.
// make's quoted forms and everything else are outside the grammar.
func conditionalSyntax(directive, rest string) bool {
	rest = strings.Trim(rest, " \t")
	if directive == "ifdef" || directive == "ifndef" {
		return len(makeWords(rest)) == 1
	}
	if !strings.HasPrefix(rest, "(") {
		return false
	}
	depth, end := 0, -1
	for index := 0; index < len(rest) && end < 0; index++ {
		switch rest[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = index
			}
		}
	}
	if end != len(rest)-1 {
		return false
	}
	args, ok := splitMakeArgs(rest[1:end])
	return ok && len(args) >= 2
}

// assign reads one variable assignment.
func (r *makefileReader) assign(number int, name, operator, value string) {
	r.texts[name] = append(r.texts[name], value)
	if makeControlNames[name] {
		// make re-reads options assigned to MAKEFLAGS, VPATH finds targets
		// and sources in other directories, SHELL runs the recipe: what
		// make reads or runs after this line is not known.
		r.outside(number, "assigns "+name)
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
	case ":=":
		// An immediate assignment expands now, against the values
		// assigned so far; a later reassignment must not change it.
		if strings.Contains(value, "$") {
			r.expandsNow(number, value)
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
func (r *makefileReader) rule(number int, targets, rest string) {
	r.inRule = true
	for _, target := range makeWords(targets) {
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
			if strings.Trim(rest, " \t") != "" {
				r.setUntyped("the go-test rule line has prerequisites or target-specific variables")
			}
		case r.wanted[target]:
			r.languages = appendUnique(r.languages, strings.TrimSuffix(target, "-test"))
		case target == ".PHONY":
			for _, prerequisite := range makeWords(rest) {
				// Under a conditional this reader did not evaluate, make
				// may skip the line, so it proves nothing.
				if prerequisite == "go-test" && !conditionUnknown(r.stack) {
					r.phonyGoTest = true
				}
			}
		case strings.HasPrefix(target, "."):
			// .DEFAULT answers every target, .SUFFIXES and a suffix rule
			// change the implicit rules, ./go-test is go-test to make.
			r.outside(number, "names the special target "+target)
		case r.remakeSource(target):
			// make remakes a makefile that has a rule, then reads it again.
			r.uncertain(fmt.Sprintf("%s line %d has a rule for %s, from which make may remake and re-read the %s", r.name, number, target, r.name))
		case implicitSource(target):
			// A rule for go-test.c makes it a source make ought to build.
			r.uncertain(fmt.Sprintf("%s line %d has a rule for %s, from which a built-in implicit rule may build go-test", r.name, number, target))
		case strings.Count(target, "%") > 1:
			r.outside(number, "has a pattern with more than one %")
		case strings.Contains(target, "%"):
			// A pattern rule serves every target it matches, so a match
			// for go-test declares it untyped; one that can serve a
			// go-test.<suffix> source or the makefile is not read.
			if patternMatches(target, "go-test") {
				if r.goTest == GoTestAbsent {
					r.goTest = GoTestUnparsed
				}
				r.setUntyped("the pattern rule " + target + " applies to go-test")
			}
			if patternReaches(target, "go-test") || patternMatches(target, r.name) || patternReaches(target, r.name) {
				r.uncertain(fmt.Sprintf("%s line %d has the pattern rule %s, which can serve a source of go-test or the %s", r.name, number, target, r.name))
			}
			for want := range r.wanted {
				if patternMatches(target, want) {
					r.languages = appendUnique(r.languages, strings.TrimSuffix(want, "-test"))
				}
			}
		}
	}
	// After the colon the grammar has words, | and references only: a
	// target-specific variable, a rule-line recipe and a second colon are
	// outside it.
	literal, ok := stripReferences(rest)
	if !ok {
		return // expandsNow already recorded the malformed reference
	}
	for index := 0; index < len(literal); index++ {
		if c := literal[index]; !literalByte(c) && c != '%' && c != '|' && c != ' ' && c != '\t' {
			r.outside(number, fmt.Sprintf("has %q after the rule's colon", c))
			return
		}
	}
}

// stripReferences removes every $(...) and ${...} reference and $$ from
// text; ok is false when a $ is bare or a reference is unterminated.
func stripReferences(text string) (literal string, ok bool) {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '$' {
			out.WriteByte(text[index])
			continue
		}
		if index+1 >= len(text) {
			return "", false
		}
		opener, closer := text[index+1], byte(')')
		switch opener {
		case '$':
			index++
			continue
		case '{':
			closer = '}'
		case '(':
		default:
			return "", false
		}
		end, nesting := -1, 0
		for scan := index + 1; scan < len(text) && end < 0; scan++ {
			switch text[scan] {
			case opener:
				nesting++
			case closer:
				nesting--
				if nesting == 0 {
					end = scan
				}
			}
		}
		if end < 0 {
			return "", false
		}
		index = end
	}
	return out.String(), true
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

// conditionUnknown reports whether an open conditional frame was not
// evaluated, so make may or may not read the line.
func conditionUnknown(stack []int) bool {
	for _, state := range stack {
		if state == condUnknown {
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

// implicitSource reports whether a rule target or a repository-root entry
// can start one of make's built-in implicit-rule chains to go-test.
func implicitSource(name string) bool { return implicitSourceOf(name, "go-test") }

// remakeSource reports whether a rule target or a repository-root entry is
// the makefile make read, which it remakes and reads again when a rule or a
// built-in implicit rule can, or a source of such a rule. When no makefile
// exists make tries to remake every default name. Names compare without
// case: make finds a file by name on a filesystem that may ignore case.
func (r *makefileReader) remakeSource(name string) bool {
	for _, makefile := range makefileNames {
		if r.name != "" && makefile != r.name {
			continue
		}
		if strings.EqualFold(name, makefile) || implicitSourceOf(name, makefile) {
			return true
		}
	}
	return false
}

// implicitSourceOf reports whether name can start a built-in implicit-rule
// chain to base: a base.<suffix> source (any suffix, since chains such as .y
// to .c to .o pass through them), an RCS or SCCS master, or their
// directories.
func implicitSourceOf(name, base string) bool {
	for _, source := range []string{base + ".", base + ",v", "s." + base, "RCS", "SCCS", "RCS/" + base + ",v", "SCCS/s." + base} {
		if strings.HasSuffix(source, ".") {
			if len(name) > len(source) && strings.EqualFold(name[:len(source)], source) {
				return true
			}
		} else if strings.EqualFold(name, source) {
			return true
		}
	}
	return false
}

// patternReaches reports whether a pattern rule can serve a base.<suffix>
// source, the start of an implicit-rule chain to base.
func patternReaches(target, base string) bool {
	prefix, _, _ := strings.Cut(target, "%")
	return strings.HasPrefix(base+".", prefix) || strings.HasPrefix(prefix, base+".")
}

// listRoot reads the repository root's entries, which make reads as the
// targets that exist: an entry whose case-folded name is go-test may be an
// up-to-date go-test, a go-test.<suffix> source can start an implicit-rule
// chain, and a name outside ASCII may fold to either on this filesystem.
func (r *makefileReader) listRoot(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		r.uncertain("the repository root cannot be listed")
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		for index := 0; index < len(name); index++ {
			if name[index] > '~' || name[index] < ' ' {
				r.uncertain("the repository root holds a name outside ASCII, which the filesystem may fold to go-test or to a source of it")
				break
			}
		}
		switch {
		case strings.EqualFold(name, "go-test"):
			r.setUntyped("the repository root holds " + name + ", which make may take as an up-to-date go-test, so make go-test may run nothing")
		case implicitSource(name):
			r.uncertain("the repository root holds " + name + ", from which a built-in implicit rule may build go-test")
		}
		if r.remakeSource(name) && !strings.EqualFold(name, r.name) {
			r.uncertain("the repository root holds " + name + ", from which make may remake and re-read the makefile")
		}
	}
}

// a function outside pureFunctions, a computed name, or a name whose
// assigned, defined or inherited text does any of these. Every argument is
// checked, even one make would never expand.
func (r *makefileReader) pure(text string, depth int) error {
	if depth > maxExpandDepth {
		return errors.New("references that nest too deeply")
	}
	for index := 0; index < len(text); index++ {
		if text[index] != '$' {
			continue
		}
		if index+1 >= len(text) {
			return errors.New("a $ that is not a reference")
		}
		opener, closer := text[index+1], byte(')')
		switch opener {
		case '$':
			index++
			continue
		case '{':
			closer = '}'
		case '(':
		default:
			return errors.New("a $ that is not a $(...) or ${...} reference")
		}
		end, nesting := -1, 0
		for scan := index + 1; scan < len(text) && end < 0; scan++ {
			switch text[scan] {
			case opener:
				nesting++
			case closer:
				nesting--
				if nesting == 0 {
					end = scan
				}
			}
		}
		if end < 0 {
			return errors.New("an unterminated reference")
		}
		body := text[index+2 : end]
		name, arguments, function := body, "", false
		if cut := strings.IndexAny(body, " \t"); cut >= 0 {
			name, arguments, function = body[:cut], body[cut+1:], true
			if !pureFunctions[name] {
				return errors.New("make function " + name)
			}
			if err := pureArguments(name, arguments); err != nil {
				return err
			}
		} else if before, after, found := strings.Cut(body, ":"); found {
			// A substitution reference $(NAME:a=b) expands NAME.
			name, arguments = before, after
		}
		if err := r.pure(arguments, depth+1); err != nil {
			return err
		}
		if !function {
			for position := 0; position < len(name); position++ {
				if !referenceNameByte(name[position], position == 0) {
					return errors.New("a reference that is not $(NAME) or ${NAME}")
				}
			}
			for _, candidate := range r.texts[name] {
				if err := r.pure(candidate, depth+1); err != nil {
					return err
				}
			}
			if inherited, ok := r.lookup(name); ok {
				if err := r.pure(inherited, depth+1); err != nil {
					return err
				}
			}
		}
		index = end
	}
	return nil
}

// pureArguments checks the arguments of a pure function for the shapes on
// which make stops reading: too few arguments, or a word or wordlist index
// that is not a literal positive number. A brace inside the arguments is
// refused rather than reasoned about, because make splits the arguments
// on the function's own delimiter only.
func pureArguments(name, arguments string) error {
	if strings.ContainsAny(arguments, "{}") {
		return errors.New("make function " + name + " with a brace in its arguments")
	}
	args, ok := splitMakeArgs(arguments)
	if !ok || len(args) < functionArity[name] {
		return fmt.Errorf("make function %s with %d arguments where make needs %d", name, len(args), functionArity[name])
	}
	switch name {
	case "word":
		if !functionIndex.MatchString(strings.Trim(args[0], " \t")) {
			return errors.New("make function word with an index that is not a literal positive number")
		}
	case "wordlist":
		if !functionIndex.MatchString(strings.Trim(args[0], " \t")) || !functionCount.MatchString(strings.Trim(args[1], " \t")) {
			return errors.New("make function wordlist with indexes that are not literal numbers")
		}
	}
	return nil
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
		name, _, _ := strings.Cut(fields[0], "=")
		if name == "PATH" {
			return nil, nil, "go-test recipe sets PATH, which changes which go runs"
		}
		if !goEnvironmentName.MatchString(name) {
			// Another name may be read-only or special to the shell make
			// starts (SHELLOPTS, UID, IFS...), so the line needs that shell.
			return nil, nil, "go-test recipe sets " + name + ", which is not a go environment variable"
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
