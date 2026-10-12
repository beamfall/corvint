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
	// GoTestAbsent: the Makefile names no go-test target.
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

// Declared is the repository's declared full scope, read from the root
// Makefile independently of the planner (STR-V0-003). Languages names each
// non-Go language with a `<language>-test` target.
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

var (
	makeAssignment = regexp.MustCompile(`^([A-Za-z_.][A-Za-z0-9_.]*)[ \t]*(\?=|::=|:=|=)[ \t]*(.*)$`)
	makeAppend     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[ \t]*(\+=|!=)`)
	makeRule       = regexp.MustCompile(`^([A-Za-z0-9_.%/-]+(?:[ \t]+[A-Za-z0-9_.%/-]+)*)[ \t]*:([^=]|$)`)
	makeReference  = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
	makeName       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envAssignment  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// goEnvironmentName names the variables that pick or configure the go
	// command; a Makefile that assigns one leaves go-test untyped even when
	// the variable is not inherited now.
	goEnvironmentName = regexp.MustCompile(`^(PATH|GO[A-Z0-9]+|CGO_[A-Z0-9_]+)$`)
	// Anything a shell would interpret makes the recipe untyped.
	shellMeta = "'\"\\;|&<>`$(){}[]*?~#!\n\r"
	// The adapter adds these itself; a recipe that sets them is untyped.
	reservedGoFlags = []string{"-json", "-run", "-skip"}
	// makeControlNames change how make reads the Makefile or runs a recipe;
	// a conditional cannot be evaluated against one and a Makefile that
	// assigns one leaves go-test untyped.
	makeControlNames = map[string]bool{
		"SHELL": true, "MAKE": true, "MAKEFLAGS": true, "MFLAGS": true, "MAKEFILES": true, "MAKEOVERRIDES": true,
		"MAKESHELL": true, "MAKELEVEL": true, "MAKECMDGOALS": true, "MAKEFILE_LIST": true, "CURDIR": true, "VPATH": true,
	}
	// exportWords change which Makefile variables reach a recipe's environment.
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

// ReadDeclared reads root/Makefile. languages are the non-Go language names a
// `<language>-test` target may declare; lookup is the invoking environment,
// which make imports as variables. Typing is conservative: anything this
// reader does not reproduce exactly leaves go-test untyped, so it runs only
// through `make go-test`.
func ReadDeclared(root string, languages []string, lookup func(string) (string, bool)) Declared {
	declared := Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestAbsent, Languages: []string{}}
	file, err := os.Open(filepath.Join(root, "Makefile"))
	if errors.Is(err, fs.ErrNotExist) {
		return declared
	}
	if err != nil {
		declared.Unreadable = "Makefile is unreadable"
		return declared
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxMakefileBytes+1))
	if err != nil || len(raw) > maxMakefileBytes {
		declared.Unreadable = fmt.Sprintf("Makefile is unreadable or exceeds %d bytes", maxMakefileBytes)
		return declared
	}
	lines := logicalLines(string(raw))
	variables := map[string]string{}
	wanted := map[string]bool{}
	for _, language := range languages {
		wanted[language+"-test"] = true
	}
	var recipe []string
	untyped, incomplete := "", ""
	setUntyped := func(reason string) {
		if untyped == "" {
			untyped = reason
		}
	}
	setIncomplete := func(reason string) {
		if incomplete == "" {
			incomplete = reason
		}
	}
	if flags, ok := lookup("MAKEFLAGS"); ok && strings.Contains(flags, "=") {
		setUntyped("MAKEFLAGS carries command-line variable assignments, which reach the go-test recipe")
	}
	if files, ok := lookup("MAKEFILES"); ok && strings.TrimSpace(files) != "" {
		setIncomplete("MAKEFILES names extra makefiles")
		setUntyped("MAKEFILES names extra makefiles, which this reader cannot read")
	}
	// refused names variables this reader cannot evaluate: assigned under a
	// conditional, or an immediate assignment whose expansion it cannot type.
	refused := map[string]bool{}
	var stack []int
	inGoTest, inRule, inDefine := false, false, false
	for index, line := range lines {
		number := index + 1
		if inDefine {
			if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "endef" {
				inDefine = false
			}
			continue
		}
		skipping := conditionSkipping(stack)
		if strings.HasPrefix(line, "\t") {
			body := strings.TrimSpace(line)
			switch {
			case skipping || body == "" || strings.HasPrefix(body, "#"):
			case inGoTest:
				recipe = append(recipe, body)
			case !inRule:
				// Outside a rule make reads a tab line as Makefile text.
				setUntyped(fmt.Sprintf("Makefile line %d is a tab line outside a rule", number))
			}
			continue
		}
		// A blank or comment line does not end a recipe; make keeps reading
		// tab lines after it as the same rule's recipe.
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		// make accepts leading blanks before a directive or assignment, and
		// any blank, tab included, after a directive word.
		line = strings.TrimLeft(line, " \t")
		directive := strings.Fields(line)[0]
		if before, _, found := strings.Cut(directive, "("); found && before != "" {
			directive = before
		}
		// A conditional does not end a recipe: make keeps reading the rule's
		// tab lines inside it, so one inside the go-test recipe span, or
		// around a go-test rule, leaves go-test untyped.
		switch directive {
		case "ifeq", "ifneq", "ifdef", "ifndef":
			if inGoTest {
				setUntyped("the go-test recipe contains a conditional")
			}
			if skipping {
				stack = append(stack, condNested)
			} else {
				stack = append(stack, evaluateConditional(line, variables, refused, lookup))
			}
			continue
		case "else", "endif":
			if inGoTest {
				setUntyped("the go-test recipe contains a conditional")
			}
			if len(stack) == 0 {
				setUntyped(fmt.Sprintf("Makefile line %d has an unmatched %s", number, directive))
				continue
			}
			top := &stack[len(stack)-1]
			if directive == "endif" {
				stack = stack[:len(stack)-1]
				continue
			}
			switch rest := strings.TrimSpace(strings.TrimPrefix(line, "else")); *top {
			case condTaken:
				*top = condDone
			case condSkipping:
				*top = condTaken
				if rest != "" {
					*top = evaluateConditional(rest, variables, refused, lookup)
				}
			}
			continue
		case "define":
			// A define body is skipped up to endef; what it holds can reach
			// a recipe through a reference, so go-test is untyped.
			inDefine = true
			setUntyped("the Makefile uses define, which this reader cannot evaluate")
			continue
		}
		if skipping {
			continue
		}
		inGoTest, inRule = false, false
		if strings.Contains(line, "$(eval") || strings.Contains(line, "${eval") {
			// eval can define any rule, a <lang>-test included.
			setIncomplete("the Makefile uses eval")
			setUntyped("the Makefile uses eval, which this reader cannot evaluate")
			continue
		}
		switch directive {
		case "export", "unexport", "override", "include", "-include", "sinclude", ".EXPORT_ALL_VARIABLES:":
			if strings.HasSuffix(directive, "include") {
				// An included file can declare any full-suite target.
				setIncomplete("the Makefile uses " + directive)
			}
			setUntyped("the Makefile uses " + strings.TrimSuffix(directive, ":") + ", which this reader cannot evaluate")
			continue
		}
		// export, unexport, override and .EXPORT_ALL_VARIABLES in any other
		// position, a target-specific one included, change a recipe's
		// environment.
		for _, word := range strings.FieldsFunc(stripMakeComment(line), func(r rune) bool {
			return strings.ContainsRune(" \t:=;,()", r)
		}) {
			if exportWords[word] {
				setUntyped("the Makefile uses " + word + ", which this reader cannot evaluate")
			}
		}
		if makeAppend.MatchString(line) {
			setUntyped("the Makefile uses += or !=, which this reader cannot evaluate")
			continue
		}
		if match := makeAssignment.FindStringSubmatch(line); match != nil {
			name, operator, value := match[1], match[2], strings.TrimSpace(stripMakeComment(match[3]))
			if makeControlNames[name] || (strings.HasPrefix(name, ".") && name != ".DEFAULT_GOAL") {
				setUntyped("the Makefile assigns " + name + ", which changes how make runs a recipe")
			}
			if _, inherited := lookup(name); operator != "?=" && (inherited || goEnvironmentName.MatchString(name)) {
				// make exports a variable that came from the environment
				// with its Makefile value, so the recipe sees that value.
				setUntyped("the Makefile assigns " + name + ", which make passes to the go-test recipe from the environment")
			}
			if len(stack) > 0 {
				refused[name] = true
			}
			if operator == "?=" {
				// ?= assigns only an undefined variable; an earlier Makefile
				// assignment wins over the environment, as in make.
				if _, defined := variables[name]; defined {
					continue
				}
				if environment, ok := lookup(name); ok {
					value = environment
				}
			}
			if (operator == ":=" || operator == "::=") && strings.Contains(value, "$") {
				// An immediate assignment expands now, against the values
				// assigned so far; a later reassignment must not change it.
				expanded, err := expandMake(value, variables, 0)
				if err != nil || strings.Contains(expanded, "$") {
					refused[name] = true
				} else {
					value = expanded
				}
			}
			variables[name] = value
			continue
		}
		if targets, _, found := strings.Cut(line, ":"); found && strings.Contains(targets, "$") {
			// A computed target can name go-test and add prerequisites to it.
			setUntyped(fmt.Sprintf("Makefile line %d has a rule target that uses $, which this reader cannot expand", number))
			continue
		}
		match := makeRule.FindStringSubmatch(line)
		if match == nil {
			setUntyped(fmt.Sprintf("Makefile line %d is not a construct this reader recognises", number))
			continue
		}
		inRule = true
		// A rule may name several targets; each one is declared.
		for _, target := range strings.Fields(match[1]) {
			switch {
			case target == "go-test":
				declared.GoTest = GoTestUnparsed
				inGoTest = true
				if len(stack) > 0 {
					setUntyped("go-test is defined under a conditional")
				}
				// Prerequisites, target-specific variables and a
				// double-colon rule all change what `make go-test` runs.
				if _, after, _ := strings.Cut(line, ":"); strings.TrimSpace(stripMakeComment(after)) != "" {
					setUntyped("the go-test rule line has prerequisites or target-specific variables")
				}
			case wanted[target]:
				declared.Languages = appendUnique(declared.Languages, strings.TrimSuffix(target, "-test"))
			case strings.HasPrefix(target, ".") && target != ".PHONY":
				setUntyped("the Makefile names the special target " + target + ", which this reader cannot evaluate")
			case patternMatches(target, "go-test"):
				setUntyped("the pattern rule " + target + " applies to go-test")
			}
		}
	}
	if inDefine {
		setUntyped("the Makefile has a define without endef")
	}
	if len(stack) != 0 {
		setUntyped("the Makefile has a conditional without endif")
	}
	sort.Strings(declared.Languages)
	if incomplete != "" {
		declared.Unreadable = incomplete + ", so its declared full scope cannot be read completely"
	}
	if declared.GoTest == GoTestUnparsed {
		for name := range refused {
			delete(variables, name) // expandMake then refuses any reference to it
		}
		env, flags, note := typeGoRecipe(recipe, variables)
		if untyped != "" {
			note = untyped
		}
		if note == "" {
			declared.GoTest, declared.GoEnv, declared.GoFlags = GoTestParsed, env, flags
		}
		declared.GoTestNote = note
	}
	return declared
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

// evaluateConditional evaluates an `ifeq (A,B)` or `ifneq (A,B)` line as make
// does for `make go-test`, against the variables assigned so far and the
// environment. Anything it cannot reproduce exactly, ifdef, the quoted form,
// surrounding blanks, a make default variable or an unsupported function,
// is condUnknown, so every branch is read.
func evaluateConditional(line string, variables map[string]string, refused map[string]bool, lookup func(string) (string, bool)) int {
	cut := strings.IndexAny(line, " \t(")
	if cut < 0 || strings.Contains(line, "#") {
		return condUnknown
	}
	word, rest := line[:cut], strings.TrimSpace(line[cut:])
	if (word != "ifeq" && word != "ifneq") || !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return condUnknown
	}
	args, ok := splitMakeArgs(rest[1 : len(rest)-1])
	if !ok || len(args) != 2 {
		return condUnknown
	}
	values := make([]string, 2)
	for i, arg := range args {
		value, err := evaluateMake(arg, variables, refused, lookup, 0)
		if err != nil || arg != strings.TrimSpace(arg) || value != strings.TrimSpace(value) {
			return condUnknown
		}
		values[i] = value
	}
	if (values[0] == values[1]) == (word == "ifeq") {
		return condTaken
	}
	return condSkipping
}

// splitMakeArgs splits at top-level commas; ok is false when the parentheses
// or braces do not balance.
func splitMakeArgs(value string) (args []string, ok bool) {
	depth, start := 0, 0
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '(', '{':
			depth++
		case ')', '}':
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

// evaluateMake expands value for a conditional: Makefile variables, the
// environment, MAKECMDGOALS as go-test, and the filter and or functions.
// Anything else is an error.
func evaluateMake(value string, variables map[string]string, refused map[string]bool, lookup func(string) (string, bool), depth int) (string, error) {
	if depth > maxExpandDepth {
		return "", errors.New("conditional variables nest too deeply")
	}
	var out strings.Builder
	for index := 0; index < len(value); {
		if value[index] != '$' {
			out.WriteByte(value[index])
			index++
			continue
		}
		if strings.HasPrefix(value[index:], "$$") {
			out.WriteString("$")
			index += 2
			continue
		}
		if index+1 >= len(value) || (value[index+1] != '(' && value[index+1] != '{') {
			return "", errors.New("a one-letter variable")
		}
		end, nesting := -1, 0
		for scan := index + 1; scan < len(value) && end < 0; scan++ {
			switch value[scan] {
			case '(', '{':
				nesting++
			case ')', '}':
				nesting--
				if nesting == 0 {
					end = scan
				}
			}
		}
		if end < 0 {
			return "", errors.New("an unterminated reference")
		}
		body := value[index+2 : end]
		expanded, err := evaluateReference(body, variables, refused, lookup, depth)
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		index = end + 1
	}
	return out.String(), nil
}

func evaluateReference(body string, variables map[string]string, refused map[string]bool, lookup func(string) (string, bool), depth int) (string, error) {
	if makeName.MatchString(body) {
		if refused[body] {
			return "", fmt.Errorf("variable %s cannot be evaluated", body)
		}
		if definition, ok := variables[body]; ok {
			return evaluateMake(definition, variables, refused, lookup, depth+1)
		}
		if body == "MAKECMDGOALS" {
			return "go-test", nil
		}
		if makeControlNames[body] {
			return "", fmt.Errorf("make sets %s itself", body)
		}
		if environment, ok := lookup(body); ok {
			return environment, nil
		}
		// make may define it by default, so its value is not known here.
		return "", fmt.Errorf("variable %s is undefined", body)
	}
	function, arguments, found := strings.Cut(body, " ")
	if !found {
		return "", errors.New("an unsupported reference")
	}
	args, ok := splitMakeArgs(strings.TrimLeft(arguments, " \t"))
	if !ok {
		return "", errors.New("unbalanced function arguments")
	}
	values := make([]string, len(args))
	for i, arg := range args {
		value, err := evaluateMake(arg, variables, refused, lookup, depth+1)
		if err != nil {
			return "", err
		}
		values[i] = value
	}
	switch function {
	case "filter":
		if len(values) != 2 {
			return "", errors.New("filter takes two arguments")
		}
		var kept []string
		for _, word := range strings.Fields(values[1]) {
			for _, pattern := range strings.Fields(values[0]) {
				matched, err := filterMatches(pattern, word)
				if err != nil {
					return "", err
				}
				if matched {
					kept = append(kept, word)
					break
				}
			}
		}
		return strings.Join(kept, " "), nil
	case "or":
		for _, value := range values {
			if value != strings.TrimSpace(value) {
				return "", errors.New("an or argument has surrounding blanks")
			}
			if value != "" {
				return value, nil
			}
		}
		return "", nil
	}
	return "", fmt.Errorf("make function %s is not supported", function)
}

// filterMatches matches one filter pattern; % matches any stem, the empty one
// included. A pattern with a backslash or several % is not supported.
func filterMatches(pattern, word string) (bool, error) {
	if strings.Contains(pattern, `\`) || strings.Count(pattern, "%") > 1 {
		return false, errors.New("an unsupported filter pattern")
	}
	prefix, suffix, wildcard := strings.Cut(pattern, "%")
	if !wildcard {
		return pattern == word, nil
	}
	return len(word) >= len(prefix)+len(suffix) && strings.HasPrefix(word, prefix) && strings.HasSuffix(word, suffix), nil
}

// patternMatches reports whether a make target pattern with one % stem
// matches name.
func patternMatches(pattern, name string) bool {
	prefix, suffix, ok := strings.Cut(pattern, "%")
	return ok && !strings.Contains(suffix, "%") && len(name) > len(prefix)+len(suffix) &&
		strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix)
}

// logicalLines joins backslash continuations, as make does for every line.
func logicalLines(body string) []string {
	var lines []string
	current := ""
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasSuffix(line, "\\") {
			current += strings.TrimSuffix(line, "\\") + " "
			continue
		}
		lines = append(lines, current+line)
		current = ""
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
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
func typeGoRecipe(recipe []string, variables map[string]string) (env, flags []string, note string) {
	if len(recipe) != 1 {
		return nil, nil, fmt.Sprintf("go-test has %d recipe lines, want exactly one", len(recipe))
	}
	line := strings.TrimPrefix(recipe[0], "@")
	expanded, err := expandMake(line, variables, 0)
	if err != nil {
		return nil, nil, err.Error()
	}
	if strings.ContainsAny(expanded, shellMeta) {
		return nil, nil, "go-test recipe needs a shell"
	}
	fields := strings.Fields(expanded)
	env = []string{}
	for len(fields) > 0 && envAssignment.MatchString(fields[0]) {
		if strings.HasPrefix(fields[0], "PATH=") {
			// The runner is resolved on Corvint's PATH, so a recipe that
			// picks another go cannot be reproduced by the typed argv.
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

// expandMake expands $(NAME) and ${NAME} from the Makefile's own variables;
// $$ is a literal dollar, which typeGoRecipe then refuses. Anything else, an
// undefined name or a make function, leaves the recipe untyped.
func expandMake(value string, variables map[string]string, depth int) (string, error) {
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
		if strings.HasPrefix(value[index:], "$$") {
			out.WriteString("$")
			index += 2
			continue
		}
		match := makeReference.FindStringSubmatchIndex(value[index:])
		if match == nil || match[0] != 0 {
			return "", errors.New("go-test recipe uses a make function or a one-letter variable")
		}
		name := value[index+match[2] : index+match[3]]
		definition, defined := variables[name]
		if !defined {
			return "", fmt.Errorf("go-test recipe names undefined variable %s", name)
		}
		expanded, err := expandMake(definition, variables, depth+1)
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		index += match[1]
	}
	return out.String(), nil
}
