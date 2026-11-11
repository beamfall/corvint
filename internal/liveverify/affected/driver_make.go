package affected

import (
	"io/fs"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// driverMakeFile is one parsed make file (AZU-V0-001). Each rule's block is
// the text attributed to that target; remainder is every other significant
// line (variables, includes, conditionals, special targets, rules whose
// targets are computed), so a change there selects every target of the file.
type driverMakeFile struct {
	source           *Source
	path             string
	dir              string
	vars             map[string][]string
	rules            map[string]*driverMakeRule
	order            []string
	remainder        []string
	remainderRecipes []string
	includes         []string
	phony            []string
	defaultGoal      string
	computedTargets  bool
	unparsable       bool
	unmodeledExports bool
	exportedNames    []string
	// events are the assignments and conditional lines in file order, from
	// which a variable's alternative values are derived (AFP-V0-3121).
	events       []driverMakeEvent
	alternatives map[driverAlternativesKey]driverAlternatives
	// statements are the lines that are a bare reference, whose expansion
	// make reads again as make file text. statementAt and includeAt hold,
	// for each statement and include, the number of events before it: make
	// expands them at once, against the values assigned so far.
	statements  []string
	statementAt []int
	includeAt   []int
}

// driverMakeEvent is one assignment or conditional line of a make file.
// Conditions are never evaluated: every branch is an alternative.
type driverMakeEvent struct {
	kind byte
	// bare marks an `else` with no condition of its own, which makes the
	// chain exhaustive.
	bare bool
	// scoped marks a target-specific assignment, in force for some targets
	// only, so it is an alternative beside every global value.
	scoped bool
	// immediate marks `:=` and `::=`, which expand their value where
	// make reads them (AFP-V0-3125).
	immediate       bool
	name, op, value string
	// text is a conditional's own line and command a `!=` assignment's shell
	// command: make expands and runs both while it reads the file.
	text, command string
}

const (
	driverEventAssign byte = 'a'
	driverEventIf     byte = 'i'
	driverEventElse   byte = 'e'
	driverEventEndif  byte = 'f'
)

type driverMakeRule struct {
	name    string
	pattern bool
	prereqs []string
	// prereqAt is, for each prerequisite line, the number of events before
	// it: make expands prerequisites where it reads them.
	prereqAt []int
	recipe   []string
	block    []string
}

// driverMakeName reports a base name GNU make reads as a make file.
func driverMakeName(base string) bool {
	return base == "Makefile" || base == "makefile" || base == "GNUmakefile" || strings.HasSuffix(base, ".mk")
}

var driverMakeConditionals = map[string]bool{"ifeq": true, "ifneq": true, "ifdef": true, "ifndef": true}

// driverMakeLines joins backslash continuations into logical lines.
func driverMakeLines(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	physical := strings.Split(body, "\n")
	lines := make([]string, 0, len(physical))
	var current strings.Builder
	joining := false
	for _, line := range physical {
		if joining {
			line = strings.TrimLeft(line, " \t")
			current.WriteByte(' ')
		}
		trailing := len(line) - len(strings.TrimRight(line, "\\"))
		if trailing%2 == 1 {
			current.WriteString(line[:len(line)-1])
			joining = true
			continue
		}
		current.WriteString(line)
		lines = append(lines, current.String())
		current.Reset()
		joining = false
	}
	if joining {
		lines = append(lines, current.String())
	}
	return lines
}

// driverMakeStrip removes a comment that starts outside every reference.
func driverMakeStrip(line string) string {
	depth := 0
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case '\\':
			index++
		case '(', '{':
			if index > 0 && line[index-1] == '$' || depth > 0 {
				depth++
			}
		case ')', '}':
			if depth > 0 {
				depth--
			}
		case '#':
			if depth == 0 {
				return strings.TrimRight(line[:index], " \t")
			}
		}
	}
	return line
}

// driverMakeOperator finds the first `=` or `:` outside every reference and
// returns its index, or -1.
func driverMakeOperator(line string) int {
	depth := 0
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case '$':
			if index+1 < len(line) && (line[index+1] == '(' || line[index+1] == '{') {
				depth++
				index++
			}
		case '(', '{':
			if depth > 0 {
				depth++
			}
		case ')', '}':
			if depth > 0 {
				depth--
			}
		case '=', ':':
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// driverMakeSplitTop splits text at separator outside every reference.
func driverMakeSplitTop(text string, separator byte, limit int) []string {
	parts := make([]string, 0, 2)
	depth := 0
	start := 0
	for index := 0; index < len(text); index++ {
		switch char := text[index]; {
		case char == '(' || char == '{':
			depth++
		case (char == ')' || char == '}') && depth > 0:
			depth--
		case char == separator && depth == 0 && (limit <= 0 || len(parts) < limit-1):
			parts = append(parts, text[start:index])
			start = index + 1
		}
	}
	return append(parts, text[start:])
}

func parseDriverMake(file string, body []byte) *driverMakeFile {
	parsed := &driverMakeFile{
		path:  file,
		dir:   path.Dir(file),
		vars:  map[string][]string{},
		rules: map[string]*driverMakeRule{},
	}
	var current []*driverMakeRule
	inRemainderRule := false
	conditional := 0
	defineName := ""
	defineOp := "="
	defineImmediate := false
	defineDepth := 0
	var defineBody []string
	endRule := func() { current, inRemainderRule = nil, false }
	for _, line := range driverMakeLines(string(body)) {
		if defineDepth > 0 {
			parsed.remainder = append(parsed.remainder, line)
			trimmed := strings.TrimSpace(line)
			first, _, _ := strings.Cut(trimmed, " ")
			switch first {
			case "endef":
				defineDepth--
				if defineDepth == 0 {
					parsed.vars[defineName] = append(parsed.vars[defineName], strings.Join(defineBody, "\n"))
					event := driverMakeEvent{kind: driverEventAssign, name: defineName, op: defineOp, immediate: defineImmediate, value: strings.Join(defineBody, "\n")}
					if defineOp == "!=" {
						event.command, event.value = event.value, driverComputed
					}
					parsed.vars[defineName][len(parsed.vars[defineName])-1] = event.value
					parsed.events = append(parsed.events, event)
					continue
				}
			case "define":
				defineDepth++
			}
			defineBody = append(defineBody, line)
			continue
		}
		if strings.HasPrefix(line, "\t") && (current != nil || inRemainderRule) {
			recipe := line[1:]
			if strings.TrimSpace(recipe) == "" {
				continue
			}
			if inRemainderRule {
				parsed.remainder = append(parsed.remainder, line)
				parsed.remainderRecipes = append(parsed.remainderRecipes, recipe)
				continue
			}
			for _, rule := range current {
				rule.recipe = append(rule.recipe, recipe)
				rule.block = append(rule.block, line)
			}
			continue
		}
		trimmed := strings.TrimSpace(driverMakeStrip(line))
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "$(eval") || strings.Contains(trimmed, "${eval") || strings.HasPrefix(trimmed, ".RECIPEPREFIX") || strings.HasPrefix(trimmed, ".ONESHELL") {
			parsed.unparsable = true
			return parsed
		}
		first, rest := driverMakeFirst(trimmed)
		rest = strings.TrimSpace(rest)
		for first == "export" || first == "override" || first == "private" || first == "unexport" {
			if first == "export" && !parsed.exportCertified(rest) {
				parsed.unmodeledExports = true
			}
			if rest == "" || driverMakeOperator(rest) < 0 && !strings.HasPrefix(rest, "define") {
				break
			}
			first, rest = driverMakeFirst(rest)
			rest = strings.TrimSpace(rest)
		}
		switch {
		case driverMakeConditionals[first]:
			conditional++
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventIf, text: rest})
			continue
		case first == "else":
			if conditional == 0 {
				parsed.unparsable = true
				return parsed
			}
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventElse, bare: rest == "", text: rest})
			continue
		case first == "endif":
			conditional--
			if conditional < 0 {
				parsed.unparsable = true
				return parsed
			}
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventEndif})
			continue
		case first == "define":
			endRule()
			parsed.remainder = append(parsed.remainder, trimmed)
			defineName = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rest), ":!?+="))
			defineOp, defineImmediate = driverMakeDefineOperator(rest)
			defineDepth = 1
			defineBody = nil
			continue
		case first == "endef":
			parsed.unparsable = true
			return parsed
		case first == "include" || first == "-include" || first == "sinclude":
			endRule()
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.includes = append(parsed.includes, rest)
			parsed.includeAt = append(parsed.includeAt, len(parsed.events))
			continue
		case first == "vpath" || first == "export" || first == "unexport" || first == "override" || first == "private":
			parsed.remainder = append(parsed.remainder, trimmed)
			continue
		}
		statement := trimmed
		if first != "" && first != strings.SplitN(trimmed, " ", 2)[0] {
			statement = strings.TrimSpace(first + " " + rest)
		}
		operator := driverMakeOperator(statement)
		if operator < 0 {
			if strings.HasPrefix(statement, "$(") || strings.HasPrefix(statement, "${") {
				parsed.remainder = append(parsed.remainder, trimmed)
				parsed.statements = append(parsed.statements, trimmed)
				parsed.statementAt = append(parsed.statementAt, len(parsed.events))
				continue
			}
			parsed.unparsable = true
			return parsed
		}
		if assignment, name, value := driverMakeAssignment(statement, operator); assignment {
			endRule()
			parsed.remainder = append(parsed.remainder, trimmed)
			event := driverMakeAssignmentEvent(statement, operator, name, value, false)
			if name != "" {
				parsed.vars[name] = append(parsed.vars[name], value)
			}
			if name != "" || !driverMakeSupportedAssignment(event) {
				parsed.events = append(parsed.events, event)
			}
			continue
		}
		if !parsed.rule(statement, operator, line, &current, &inRemainderRule) {
			parsed.unparsable = true
			return parsed
		}
	}
	if conditional != 0 || defineDepth != 0 {
		parsed.unparsable = true
		return parsed
	}
	for _, name := range parsed.phony {
		if rule, present := parsed.rules[name]; present {
			rule.block = append(rule.block, ".PHONY")
		} else {
			parsed.remainder = append(parsed.remainder, ".PHONY: "+name)
		}
	}
	if goals, present := parsed.vars[".DEFAULT_GOAL"]; present {
		parsed.defaultGoal = strings.TrimSpace(strings.Join(goals, " "))
	}
	parsed.remainder = append(parsed.remainder, "default-goal="+parsed.defaultGoal)
	return parsed
}

func driverMakeFirst(text string) (string, string) {
	if split := strings.IndexAny(text, " \t"); split >= 0 {
		return text[:split], strings.TrimSpace(text[split+1:])
	}
	return text, ""
}

// exportCertified admits only project controls that none of the
// certified built-in commands interpret. Unknown names, computed names,
// export-all, and toolchain settings retain the environment's read boundary.
func (parsed *driverMakeFile) exportCertified(text string) bool {
	if index := driverMakeOperator(text); index >= 0 {
		assignment, name, _ := driverMakeAssignment(text, index)
		if !assignment {
			return false
		}
		text = name
	}
	names := strings.Fields(text)
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		switch name {
		case "CORVINT_REQUIRE_NODE", "HELP_BASE", "LANE", "DOGFOOD_FINISH_ENTRY":
			parsed.exportedNames = append(parsed.exportedNames, name)
		default:
			return false
		}
	}
	return true
}

// driverMakeAssignment classifies a statement whose first operator is at
// index. Colon-assignment forms are recognized syntactically, but only
// `=`, `:=`, `::=`, `?=`, `+=` and `!=` have supported semantics.
func driverMakeAssignment(statement string, index int) (bool, string, string) {
	name := statement[:index]
	op := statement[index:]
	switch {
	case op[0] == '=':
		op = "="
	case strings.HasPrefix(strings.TrimLeft(op, ":"), "="):
		op = op[:strings.IndexByte(op, '=')+1]
	default:
		return false, "", ""
	}
	value := strings.TrimSpace(statement[index+len(op):])
	if strings.HasSuffix(name, "!") {
		value = driverComputed
	}
	name = strings.TrimSpace(strings.TrimRight(name, "?+!"))
	if strings.Contains(name, "$") {
		name = ""
	}
	return true, name, value
}

// driverMakeAssignmentEvent is the event of the assignment whose first
// operator is at index. Its op is "?=", "+=", "!=" or "=" for every plain or
// supported immediate form. Other operators remain unsupported (AFP-V0-3125).
// A `!=` event keeps its shell command, which make runs where it reads it.
func driverMakeAssignmentEvent(statement string, index int, name, value string, scoped bool) driverMakeEvent {
	event := driverMakeEvent{kind: driverEventAssign, scoped: scoped, name: name, op: "=", value: value}
	head := strings.TrimRight(statement[:index], " \t")
	switch {
	case head != "" && strings.IndexByte("?+!", head[len(head)-1]) >= 0:
		event.op = head[len(head)-1:] + "="
	case statement[index] == ':':
		op := statement[index : index+strings.IndexByte(statement[index:], '=')+1]
		if op == ":=" || op == "::=" {
			event.immediate = true
		} else {
			event.op = "unsupported"
		}
	}
	if event.op == "!=" {
		event.command = strings.TrimSpace(statement[index+1:])
	}
	return event
}

func driverMakeSupportedAssignment(event driverMakeEvent) bool {
	return event.op == "=" || event.op == "?=" || event.op == "+=" || event.op == "!="
}

// driverMakeDefineOperator is the kind of a `define NAME [op]` line and
// whether it expands its body at once.
func driverMakeDefineOperator(rest string) (string, bool) {
	body, assigns := strings.CutSuffix(strings.TrimSpace(rest), "=")
	switch {
	case !assigns:
		return "=", false
	case strings.HasSuffix(body, ":"):
		operator := driverMakeOperator(strings.TrimSpace(rest))
		if operator >= 0 {
			op := strings.TrimSpace(rest)[operator:]
			if op == ":=" || op == "::=" {
				return "=", true
			}
		}
		return "unsupported", false
	case body != "" && strings.IndexByte("?+!", body[len(body)-1]) >= 0:
		return body[len(body)-1:] + "=", false
	}
	return "=", false
}

// driverMaxAlternatives bounds the distinct values one variable can take; a
// variable past it is retained as ambiguous (AZU-V0-9702).
const driverMaxAlternatives = 16

// driverUndefined stands for a variable no assignment has set, which the
// environment or command line may supply.
const driverUndefined = "\x03"

// driverAlternativesKey names the values of one variable after the first
// limit events of its file.
type driverAlternativesKey struct {
	name  string
	limit int
}

// driverMakeValue is one value a variable can hold: unexpanded text for a
// recursive variable, or for a simple one the text make captured when it
// read the assignment, its `$` escaped so a later expansion returns it.
type driverMakeValue struct {
	text   string
	simple bool
}

// driverAlternatives is every value a variable can hold at one point of its
// file. ok is false past driverMaxAlternatives or where an immediate value
// cannot be represented, and evaluated marks an immediate value whose
// expansion used what the claimant before AFP-V0-3121 retained as ambiguous.
type driverAlternatives struct {
	values    []driverMakeValue
	ok        bool
	evaluated bool
	// evaluatedBy names the variables and functions that set evaluated
	// (AZU-V0-611). Diagnostic only.
	evaluatedBy map[string]bool
}

// driverMergeReasons adds every reason of src to *dst, allocating it on
// first use.
func driverMergeReasons(dst *map[string]bool, src map[string]bool) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = map[string]bool{}
	}
	for reason := range src {
		(*dst)[reason] = true
	}
}

// variableAlternatives is every value the file can leave in name after its
// first limit events (every event when limit is negative), each as text to
// expand, or driverUndefined where no assignment need run (AFP-V0-3121).
// Every branch of a conditional is taken, a branch chain without a bare
// `else` may also take none. The values are those of the default
// invocation: a variable the file sets is not taken from the environment or
// the command line. An immediate assignment captures the expansion of its
// value against the values assigned before it, and `+=` appends to a simple
// value the same way (AFP-V0-3125). A target-specific value is in force for
// its target whatever global assignment follows it, so it stands beside
// every global value (AFP-V0-3126).
func (parsed *driverMakeFile) variableAlternatives(name string, limit int) driverAlternatives {
	if limit < 0 || limit > len(parsed.events) {
		limit = len(parsed.events)
	}
	if parsed.alternatives == nil {
		parsed.alternatives = map[driverAlternativesKey]driverAlternatives{}
	}
	key := driverAlternativesKey{name: name, limit: limit}
	if cached, present := parsed.alternatives[key]; present {
		return cached
	}
	// An immediate value expands against strictly earlier events, so this
	// guard is never read; it only keeps a malformed walk from recursing.
	parsed.alternatives[key] = driverAlternatives{}
	result := parsed.walkAlternatives(name, limit)
	parsed.alternatives[key] = result
	return result
}

func (parsed *driverMakeFile) walkAlternatives(name string, limit int) driverAlternatives {
	evaluated := false
	var evaluatedBy map[string]bool
	union := func(sets ...[]driverMakeValue) []driverMakeValue {
		var out []driverMakeValue
		for _, set := range sets {
			for _, value := range set {
				if !slices.Contains(out, value) {
					out = append(out, value)
				}
			}
		}
		return out
	}
	// captured is text expanded where event at is read, each complete
	// expansion one simple value.
	captured := func(text string, at int) ([]string, bool) {
		expander := parsed.expanderAt(at)
		variants := expander.expandAll(text)
		evaluated = evaluated || expander.evaluated
		driverMergeReasons(&evaluatedBy, expander.evaluatedBy)
		if expander.ambiguous {
			return nil, false
		}
		for index, variant := range variants {
			variants[index] = strings.ReplaceAll(variant, "$", "$$")
		}
		return variants, true
	}
	assign := func(current []driverMakeValue, event driverMakeEvent, at int) ([]driverMakeValue, bool) {
		var next []driverMakeValue
		for _, value := range current {
			defined := value.text != driverUndefined
			switch {
			case !driverMakeSupportedAssignment(event):
				return nil, false
			case event.op == "!=":
				next = append(next, driverMakeValue{text: driverComputed, simple: true})
			case event.op == "?=" && defined:
				next = append(next, value)
			case event.op == "+=" && defined && value.simple:
				variants, ok := captured(event.value, at)
				if !ok {
					return nil, false
				}
				for _, variant := range variants {
					next = append(next, driverMakeValue{text: value.text + " " + variant, simple: true})
				}
			case event.op == "+=" && defined:
				next = append(next, driverMakeValue{text: value.text + " " + event.value})
			case event.immediate:
				variants, ok := captured(event.value, at)
				if !ok {
					return nil, false
				}
				for _, variant := range variants {
					next = append(next, driverMakeValue{text: variant, simple: true})
				}
			default:
				next = append(next, driverMakeValue{text: event.value})
			}
		}
		return union(next), true
	}
	type frame struct {
		entry, done []driverMakeValue
		bare        bool
	}
	current := []driverMakeValue{{text: driverUndefined}}
	var stack []frame
	var scoped []int
	for at, event := range parsed.events[:limit] {
		switch event.kind {
		case driverEventIf:
			stack = append(stack, frame{entry: current})
		case driverEventElse:
			if len(stack) == 0 {
				continue
			}
			top := &stack[len(stack)-1]
			top.done = union(top.done, current)
			top.bare = top.bare || event.bare
			current = top.entry
		case driverEventEndif:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			current = union(top.done, current)
			if !top.bare {
				current = union(current, top.entry)
			}
		case driverEventAssign:
			if event.name != name {
				continue
			}
			if event.scoped {
				scoped = append(scoped, at)
				continue
			}
			next, ok := assign(current, event, at)
			if !ok {
				return driverAlternatives{evaluated: evaluated, evaluatedBy: evaluatedBy}
			}
			current = next
		}
		if len(current) > driverMaxAlternatives {
			return driverAlternatives{evaluated: evaluated, evaluatedBy: evaluatedBy}
		}
	}
	// AFP-V0-3126: a target-specific value applies over the global value
	// make leaves at the end of the file, or over an earlier target-specific
	// value of the same target; each is one more alternative.
	all := current
	for _, at := range scoped {
		next, ok := assign(all, parsed.events[at], at)
		if !ok {
			return driverAlternatives{evaluated: evaluated, evaluatedBy: evaluatedBy}
		}
		all = union(all, next)
		if len(all) > driverMaxAlternatives {
			return driverAlternatives{evaluated: evaluated, evaluatedBy: evaluatedBy}
		}
	}
	return driverAlternatives{values: all, ok: true, evaluated: evaluated, evaluatedBy: evaluatedBy}
}

// driverParseTime is what make expands and runs while it reads a file,
// whatever target it then builds (AFP-V0-3127): the commands of every
// `$(shell ...)` and `!=` assignment it reaches there, whether one of them
// cannot be represented, and whether an expansion there was evaluated.
type driverParseTime struct {
	readPaths      []string
	shells         []string
	shellAmbiguous bool
	evaluated      bool
	evaluatedBy    map[string]bool
}

// parseTime expands every text make expands while it reads the file, each
// against the values assigned before it: conditional lines, immediate
// values, `+=` onto a simple value, `!=` commands, bare statements, includes
// and every rule's prerequisites.
func (parsed *driverMakeFile) parseTime() driverParseTime {
	var result driverParseTime
	// expand expands text where make reads it. observed marks text whose
	// expanded value every target observes (a `!=` command, a bare
	// statement, an include); evaluation of any other text marks the file
	// only when it ran a command, read a file or met an unmodeled function
	// (AZU-V0-610). A conditional's condition is never evaluated into a
	// branch choice (every branch is an alternative), an immediate value
	// reaches a target only through the alternatives of the variable it
	// sets, and a prerequisite list only its own rule: each of those marks
	// the target that expands it (AFP-V0-3121, AFP-V0-3122).
	expand := func(text string, at int, observed bool) ([]string, bool) {
		if !strings.Contains(text, "$") {
			return []string{text}, false
		}
		expander := parsed.expanderAt(at)
		variants := expander.expandAll(text)
		for _, script := range expander.shells {
			if !slices.Contains(result.shells, script) {
				result.shells = append(result.shells, script)
			}
		}
		result.readPaths = append(result.readPaths, expander.readPaths...)
		result.shellAmbiguous = result.shellAmbiguous || expander.shellAmbiguous || expander.unmodeledEffects
		effects := len(expander.shells) != 0 || len(expander.readPaths) != 0 || expander.shellAmbiguous || expander.unmodeledEffects || expander.ambiguous
		if observed || effects {
			result.evaluated = result.evaluated || expander.evaluated
			driverMergeReasons(&result.evaluatedBy, expander.evaluatedBy)
		}
		return variants, expander.ambiguous
	}
	for at, event := range parsed.events {
		switch {
		case event.kind == driverEventIf || event.kind == driverEventElse:
			expand(event.text, at, false)
		case event.kind != driverEventAssign:
		case event.op == "!=":
			variants, ambiguous := expand(event.command, at, true)
			result.shellAmbiguous = result.shellAmbiguous || ambiguous
			for _, script := range variants {
				if !slices.Contains(result.shells, script) {
					result.shells = append(result.shells, script)
				}
			}
		case event.immediate:
			expand(event.value, at, false)
		case event.op == "+=":
			alternatives := parsed.variableAlternatives(event.name, at)
			if !alternatives.ok || slices.ContainsFunc(alternatives.values, func(value driverMakeValue) bool { return value.simple }) {
				expand(event.value, at, false)
			}
		}
	}
	for index, statement := range parsed.statements {
		expand(statement, parsed.statementAt[index], true)
	}
	for index, include := range parsed.includes {
		expand(include, parsed.includeAt[index], true)
	}
	for _, target := range parsed.order {
		rule := parsed.rules[target]
		for index, prerequisites := range rule.prereqs {
			expand(prerequisites, rule.prereqAt[index], false)
		}
	}
	return result
}

// exportsCertified admits only the parsed project-control allowlist with
// effect-free values. Its refusal applies even to a completely literal recipe.
func (parsed *driverMakeFile) exportsCertified() bool {
	if parsed.unmodeledExports {
		return false
	}
	expander := parsed.expander(map[string]string{})
	for _, name := range parsed.exportedNames {
		expander.expandAll("$(" + name + ")")
	}
	if expander.ambiguous || len(expander.shells) != 0 || len(expander.readPaths) != 0 {
		return false
	}
	return true
}

// recipeShellCertified reports that make hands every recipe line to a plain
// shell with a `-c` flag: a SHELL or .SHELLFLAGS assignment that names any
// other program, or adds an operand, runs that program for every target
// (AFP-V0-3127). Make never takes SHELL from the environment.
func (parsed *driverMakeFile) recipeShellCertified() bool {
	expander := parsed.expander(map[string]string{})
	for _, variant := range expander.expandAll("$(SHELL)") {
		fields := strings.Fields(variant)
		if len(fields) != 1 || driverDynamic(variant) || !driverShells[path.Base(fields[0])] {
			return false
		}
		shell := fields[0]
		// AFP-V0-3127: a repository wrapper can share a shell basename
		// while reading files outside the recipe's bounded read set.
		if strings.Contains(shell, "/") && shell != "/bin/"+path.Base(shell) && shell != "/usr/bin/"+path.Base(shell) {
			return false
		}
	}
	if _, set := parsed.vars[".SHELLFLAGS"]; set {
		for _, variant := range expander.expandAll("$(.SHELLFLAGS)") {
			fields := strings.Fields(variant)
			if driverDynamic(variant) || len(fields) == 0 || !strings.HasPrefix(fields[len(fields)-1], "-") || !strings.HasSuffix(fields[len(fields)-1], "c") {
				return false
			}
			for _, field := range fields {
				if !strings.HasPrefix(field, "-") && field != "pipefail" && field != "errexit" && field != "nounset" {
					return false
				}
			}
		}
	}
	return !expander.ambiguous
}

var driverMakeSpecials = map[string]bool{
	".PHONY": true, ".SUFFIXES": true, ".DEFAULT": true, ".PRECIOUS": true, ".INTERMEDIATE": true,
	".NOTINTERMEDIATE": true, ".SECONDARY": true, ".SECONDEXPANSION": true, ".DELETE_ON_ERROR": true,
	".IGNORE": true, ".LOW_RESOLUTION_TIME": true, ".SILENT": true, ".EXPORT_ALL_VARIABLES": true,
	".NOTPARALLEL": true, ".POSIX": true, ".WAIT": true, ".EXTRA_PREREQS": true,
}

// rule records a rule line. It returns false for a form it does not model
// (a static pattern rule).
func (parsed *driverMakeFile) rule(statement string, colon int, line string, current *[]*driverMakeRule, inRemainderRule *bool) bool {
	targetText := strings.TrimSuffix(strings.TrimSpace(statement[:colon]), "&")
	after := statement[colon+1:]
	after = strings.TrimPrefix(after, ":")
	recipe := ""
	if parts := driverMakeSplitTop(after, ';', 2); len(parts) == 2 {
		after, recipe = parts[0], parts[1]
	}
	targetSpecific := false
	if operator := driverMakeOperator(after); operator >= 0 {
		if assignment, _, _ := driverMakeAssignment(after, operator); !assignment {
			return false
		}
		targetSpecific = true
	}
	targets := strings.Fields(targetText)
	special := false
	for _, target := range targets {
		if target == ".EXPORT_ALL_VARIABLES" {
			parsed.unmodeledExports = true
		}
		if driverMakeSpecials[target] {
			special = true
		}
	}
	if len(targets) == 0 {
		return false
	}
	if special && len(targets) == 1 && targets[0] == ".PHONY" && !targetSpecific && !strings.Contains(after, "$") {
		parsed.phony = append(parsed.phony, strings.Fields(after)...)
		*current, *inRemainderRule = nil, true
		if strings.TrimSpace(recipe) != "" {
			parsed.remainderRecipes = append(parsed.remainderRecipes, recipe)
		}
		return true
	}
	if special || strings.Contains(targetText, "$") {
		if strings.Contains(targetText, "$") {
			parsed.computedTargets = true
		}
		parsed.remainder = append(parsed.remainder, line)
		*current, *inRemainderRule = nil, true
		if strings.TrimSpace(recipe) != "" {
			parsed.remainderRecipes = append(parsed.remainderRecipes, recipe)
		}
		return true
	}
	rules := make([]*driverMakeRule, 0, len(targets))
	for _, target := range targets {
		rule, present := parsed.rules[target]
		if !present {
			rule = &driverMakeRule{name: target, pattern: strings.Contains(target, "%")}
			parsed.rules[target] = rule
			parsed.order = append(parsed.order, target)
			if parsed.defaultGoal == "" && !rule.pattern && (!strings.HasPrefix(target, ".") || strings.Contains(target, "/")) {
				parsed.defaultGoal = target
			}
		}
		rule.block = append(rule.block, line)
		if targetSpecific {
			statement := strings.TrimSpace(after)
			for {
				modifier, rest := driverMakeFirst(statement)
				if modifier != "export" && modifier != "override" && modifier != "private" && modifier != "unexport" {
					break
				}
				if modifier == "export" && !parsed.exportCertified(rest) {
					parsed.unmodeledExports = true
				}
				statement = rest
			}
			operator := driverMakeOperator(statement)
			if assignment, name, value := driverMakeAssignment(statement, operator); assignment {
				event := driverMakeAssignmentEvent(statement, operator, name, value, true)
				if name != "" {
					parsed.vars[name] = append(parsed.vars[name], value)
				}
				if name != "" || !driverMakeSupportedAssignment(event) {
					parsed.events = append(parsed.events, event)
				}
			}
		} else {
			rule.prereqs = append(rule.prereqs, after)
			rule.prereqAt = append(rule.prereqAt, len(parsed.events))
		}
		if strings.TrimSpace(recipe) != "" {
			rule.recipe = append(rule.recipe, recipe)
		}
		rules = append(rules, rule)
	}
	if targetSpecific {
		// A target-specific variable line opens no recipe context.
		*current, *inRemainderRule = nil, false
		return true
	}
	*current, *inRemainderRule = rules, false
	return true
}

// driverMakeBlocks is a file's per-target text and remainder, the inputs a
// diff is attributed by. ok is false for an unparsable file.
func driverMakeBlocks(file string, body []byte) (blocks map[string]string, remainder string, ok bool) {
	parsed := parseDriverMake(file, body)
	if parsed.unparsable {
		return nil, "", false
	}
	blocks = make(map[string]string, len(parsed.rules))
	for name, rule := range parsed.rules {
		blocks[driverMakeUnitID(file, name)] = strings.Join(rule.block, "\n")
	}
	return blocks, strings.Join(parsed.remainder, "\n"), true
}

func driverMakeUnitID(file, target string) string {
	return DriverNamespace + ":make:" + file + "#" + target
}

// driverExpander expands make references. A reference with several possible
// values (a reassigned or conditional variable, an `if`, `or` or `and` whose
// condition is unknown) is a choice: expandAll enumerates every complete
// expansion, one per combination of choices, so no alternative is joined
// into a different shell word (AFP-V0-3121, AZU-V0-9702). What it cannot
// represent exactly sets ambiguous, which retains the computed invocation;
// a value only the environment can supply becomes driverComputed.
type driverExpander struct {
	file *driverMakeFile
	// limit is how many of the file's events have been read where the text
	// is expanded: every event (-1) for a recipe, which make expands after
	// reading the whole file, fewer for text make expands at once.
	limit     int
	auto      map[string]string
	locals    map[string]string
	visiting  map[string]bool
	depth     int
	ambiguous bool
	// Effects survive enumeration of a condition: evaluation may read files.
	unmodeledEffects bool
	// readPaths survive value selection, including discarded conditions.
	readPaths []string
	// shellAmbiguous marks an ambiguous expansion inside a `$(shell ...)`
	// command, which make runs whatever target is built.
	shellAmbiguous bool
	// evaluated marks an expansion that resolved what the claimant before
	// AFP-V0-3121 retained as ambiguous: a reassigned variable or one of the
	// conditional and word functions. Such a target stays a computed site
	// unless every command it runs is certified (AFP-V0-3122).
	evaluated bool
	// evaluatedBy names each variable or function that set evaluated
	// (AZU-V0-611). Diagnostic only: no unit field or selection reads it.
	evaluatedBy map[string]bool
	// shells are the expanded commands of every `$(shell ...)` reached.
	shells []string
	// autoChoices holds the alternatives of an automatic variable whose
	// prerequisites expand in several ways.
	autoChoices map[string][]string
	// trail is the choice sequence of the expansion in progress, cursor the
	// next choice it replays, and picked each variable's choice in this
	// expansion, since make gives a variable one value per run.
	trail  []driverChoice
	cursor int
	picked map[string]int
}

// driverChoice is one choice point of an expansion: how many alternatives it
// has and which one the current expansion takes.
type driverChoice struct{ arity, pick int }

const driverExpandLimit = 1 << 16

// driverMaxVariants bounds the expansions walked for one text; text past it
// is retained as ambiguous (AFP-V0-3123). The repository's own Makefile needs
// at most 112 distinct expansions for one recipe line.
const driverMaxVariants = 256

func (parsed *driverMakeFile) expander(auto map[string]string) *driverExpander {
	return &driverExpander{file: parsed, limit: -1, auto: auto, locals: map[string]string{}, visiting: map[string]bool{}, picked: map[string]int{}}
}

// expanderAt expands text make expands as it reads the file, after its first
// at events: an immediate value, a prerequisite, an include, a conditional or
// a bare statement (AFP-V0-3125). An automatic variable, empty there, is
// taken as unknown.
func (parsed *driverMakeFile) expanderAt(at int) *driverExpander {
	expander := parsed.expander(map[string]string{})
	expander.limit = at
	return expander
}

// expandAll returns every distinct complete expansion of text, in the order
// a depth-first walk of its choices finds them. Past driverMaxVariants it
// marks the expander ambiguous and returns the expansions found so far.
func (e *driverExpander) expandAll(text string) []string {
	if !strings.Contains(text, "$") {
		return []string{text}
	}
	e.trail = e.trail[:0]
	var variants []string
	for run := 0; ; run++ {
		if run == driverMaxVariants {
			e.ambiguous = true
			break
		}
		e.cursor = 0
		clear(e.picked)
		value := e.expand(text)
		if !slices.Contains(variants, value) {
			variants = append(variants, value)
		}
		e.trail = e.trail[:e.cursor]
		for len(e.trail) != 0 && e.trail[len(e.trail)-1].pick+1 >= e.trail[len(e.trail)-1].arity {
			e.trail = e.trail[:len(e.trail)-1]
		}
		if len(e.trail) == 0 {
			break
		}
		e.trail[len(e.trail)-1].pick++
	}
	e.trail, e.cursor = e.trail[:0], 0
	clear(e.picked)
	e.ambiguous = e.ambiguous || e.unmodeledEffects
	return variants
}

// choose returns which of arity alternatives the current expansion takes.
func (e *driverExpander) choose(arity int) int {
	if arity <= 1 {
		return 0
	}
	if e.cursor < len(e.trail) {
		pick := e.trail[e.cursor].pick
		e.cursor++
		return min(pick, arity-1)
	}
	e.trail = append(e.trail, driverChoice{arity: arity})
	e.cursor++
	return 0
}

// pick is choose memoized under key for the current expansion.
func (e *driverExpander) pick(key string, arity int) int {
	if picked, present := e.picked[key]; present {
		return picked
	}
	picked := e.choose(arity)
	e.picked[key] = picked
	return picked
}

// driverTruth is whether a make condition's expansion is non-empty: 1 when
// it holds a literal byte, 0 when it is empty, and -1 when only a value the
// environment supplies decides it.
func driverTruth(value string) int {
	switch {
	case strings.ReplaceAll(value, driverComputed, "") != "":
		return 1
	case value == "":
		return 0
	}
	return -1
}

// condition evaluates an `if` condition. Its value selects a branch and is
// never part of the result, so a condition the expander cannot represent
// takes both branches instead of retaining the whole invocation.
func (e *driverExpander) condition(text string) int {
	saved := e.ambiguous
	e.ambiguous = false
	trimmed, exact := driverMakeTrim(text)
	value := e.expand(trimmed)
	unknown := e.ambiguous
	e.ambiguous = saved
	if unknown || !exact {
		return -1
	}
	return driverTruth(value)
}

// definite reports values that hold no part only the environment supplies
// and that make splits into the same words as this evaluator. GNU make splits
// words at the bytes C isspace accepts, which for bytes past ASCII depends on
// make's locale; Go's strings.Fields splits at Unicode spaces. Only ASCII
// values are therefore exact (AFP-V0-9050).
func definite(values ...string) bool {
	for _, value := range values {
		if driverDynamic(value) || !driverASCII(value) {
			return false
		}
	}
	return true
}

// driverASCII reports text whose every byte is ASCII.
func driverASCII(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] >= 0x80 {
			return false
		}
	}
	return true
}

// driverMakeSpace is the whitespace GNU make's ISSPACE accepts in ASCII.
const driverMakeSpace = " \t\n\r\v\f"

// driverMakeTrim strips the whitespace make strips from an `if` condition or
// an `or`/`and` argument before expanding it. It reports false when a byte past
// ASCII borders the result: make's locale decides whether that is whitespace.
func driverMakeTrim(text string) (string, bool) {
	trimmed := strings.Trim(text, driverMakeSpace)
	if trimmed != "" && (trimmed[0] >= 0x80 || trimmed[len(trimmed)-1] >= 0x80) {
		return trimmed, false
	}
	return trimmed, true
}

// environmental reports a value made only of parts the environment supplies:
// any word selected from it is environment-supplied too, or there is none.
func environmental(value string) bool {
	return strings.Contains(value, driverComputed) && strings.Trim(strings.ReplaceAll(value, driverComputed, ""), driverMakeSpace) == ""
}

// selected is the result of selecting words from an environmental value:
// either some of its words, as unknown as the value itself, or none, which
// leaves any attached literal on its own. Both are choices.
func (e *driverExpander) selected() (string, bool) {
	if e.choose(2) == 0 {
		return driverComputed, true
	}
	return "", true
}

func (e *driverExpander) expand(text string) string {
	if !strings.Contains(text, "$") {
		return text
	}
	if e.depth > 10 {
		e.unmodeledEffects = true
		return driverComputed
	}
	e.depth++
	defer func() { e.depth-- }()
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		char := text[index]
		if char != '$' || index+1 >= len(text) {
			out.WriteByte(char)
			continue
		}
		next := text[index+1]
		switch {
		case next == '$':
			out.WriteByte('$')
			index++
		case next == '(' || next == '{':
			end := driverMakeClose(text, index+1)
			if end < 0 {
				e.unmodeledEffects = true
				out.WriteString(driverComputed)
				index = len(text)
				continue
			}
			out.WriteString(e.reference(text[index+2 : end]))
			index = end
		default:
			out.WriteString(e.lookup(string(next)))
			index++
		}
		if out.Len() > driverExpandLimit {
			e.unmodeledEffects = true
			return driverComputed
		}
	}
	return out.String()
}

// driverMakeClose finds the bracket closing the one at open, or -1.
func driverMakeClose(text string, open int) int {
	depth := 0
	for index := open; index < len(text); index++ {
		switch text[index] {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func (e *driverExpander) reference(inner string) string {
	// A function name ends at the first blank; make skips every blank
	// after it, so they are no part of the first argument (AFP-V0-9050).
	name, args, spaced := inner, "", false
	if blank := strings.IndexAny(inner, " \t"); blank >= 0 {
		name, args, spaced = inner[:blank], strings.TrimLeft(inner[blank:], driverMakeSpace), true
	}
	if spaced && strings.Contains(name, "$") {
		e.unmodeledEffects = true
		return driverComputed
	}
	if spaced {
		if value, known := e.function(name, args); known {
			return value
		}
	}
	if colon := strings.IndexByte(inner, ':'); colon > 0 && strings.Contains(inner[colon:], "=") && !strings.Contains(inner[:colon], "$") {
		// Make expands the whole reference before it splits a
		// substitution reference, so a reference in the patterns is
		// their expansion, never their spelling (AFP-V0-9050).
		from, to, _ := strings.Cut(e.expand(inner[colon+1:]), "=")
		value := e.lookup(inner[:colon])
		if driverDynamic(from) || !driverASCII(from+to+value) || strings.Contains(from, "%") && strings.Contains(from+to, "\\") {
			e.ambiguous = true
			return driverComputed
		}
		words := strings.Fields(value)
		for index, word := range words {
			switch {
			case driverDynamic(word):
				// A partly unknown word can end in from once known; a
				// wholly unknown one stays unknown, or no word at all.
				if strings.Trim(word, driverComputed+driverExpansion) != "" {
					e.ambiguous = true
					return driverComputed
				}
			case strings.Contains(from, "%"):
				words[index] = driverPatsubst(from, to, word)
			case strings.HasSuffix(word, from):
				words[index] = strings.TrimSuffix(word, from) + to
			}
		}
		return strings.Join(words, " ")
	}
	resolved := e.expand(inner)
	if driverDynamic(resolved) {
		return driverComputed
	}
	return e.lookup(resolved)
}

func (e *driverExpander) lookup(name string) string {
	if value, present := e.locals[name]; present {
		return value
	}
	switch name {
	case "@", "<", "^", "+", "?", "*", "@D", "@F", "<D", "<F", "^D", "^F", "*D", "*F", "|", "%":
		if values := e.autoChoices[name]; len(values) != 0 {
			return values[e.pick("\x00auto:"+name, len(values))]
		}
		if value, present := e.auto[name]; present {
			return value
		}
		return driverComputed
	case "MAKE":
		return "make"
	case "MAKEFILE_LIST":
		return path.Base(e.file.path)
	case "MAKEFLAGS", "MFLAGS":
		return ""
	}
	alternatives := e.file.variableAlternatives(name, e.limit)
	if len(e.file.vars[name]) > 1 || alternatives.evaluated {
		e.evaluated = true
		driverMergeReasons(&e.evaluatedBy, alternatives.evaluatedBy)
		if len(e.file.vars[name]) > 1 {
			driverMergeReasons(&e.evaluatedBy, map[string]bool{name: true})
		}
	}
	if !alternatives.ok {
		e.unmodeledEffects = true
		e.ambiguous = true
		return driverComputed
	}
	// Each alternative is one complete value: the expansion in progress
	// takes one of them for every reference to name (AFP-V0-3121).
	value := alternatives.values[e.pick(name, len(alternatives.values))].text
	if value == driverUndefined {
		if name == "SHELL" {
			return "sh"
		}
		return driverComputed
	}
	if e.visiting[name] {
		e.unmodeledEffects = true
		return driverComputed
	}
	e.visiting[name] = true
	defer delete(e.visiting, name)
	return e.expand(value)
}

func (e *driverExpander) function(name, args string) (string, bool) {
	split := func(limit int) []string {
		parts := driverMakeSplitTop(args, ',', limit)
		for len(parts) < limit {
			parts = append(parts, "")
		}
		return parts
	}
	// mapWords applies transform to each word. GNU make splits words at
	// ASCII whitespace only, so non-ASCII values retain. A wholly unknown
	// word stays unknown. A word only partly known may expand to other
	// words or a different spelling, so an exact transform retains it;
	// addprefix and addsuffix keep it unchanged, because the literal they
	// would add can neither empty the result nor be selected by a filter
	// that cannot match the unknown word (AFP-V0-9050).
	mapWords := func(text string, exact bool, transform func(string) (string, bool)) (string, bool) {
		value := e.expand(text)
		if !driverASCII(value) {
			e.ambiguous = true
			return driverComputed, true
		}
		var out []string
		for _, word := range strings.Fields(value) {
			if driverDynamic(word) {
				if exact && strings.Trim(word, driverComputed+driverExpansion) != "" {
					e.ambiguous = true
					return driverComputed, true
				}
				out = append(out, word)
				continue
			}
			if mapped, keep := transform(word); keep {
				out = append(out, mapped)
			}
		}
		return strings.Join(out, " "), true
	}
	each := func(transform func(string) string) func(string) (string, bool) {
		return func(word string) (string, bool) { return transform(word), true }
	}
	// escaped reports a pattern whose backslashes GNU make may treat as
	// quoting a percent sign; this evaluator does not model that quoting.
	escaped := func(values ...string) bool {
		joined := strings.Join(values, "")
		return strings.Contains(joined, "%") && strings.Contains(joined, "\\")
	}
	// AZU-V0-9850: a superset of words is not conservative when a literal
	// prefix or suffix attaches to a different word. These functions are
	// evaluated exactly over definite values (AFP-V0-3121). Over a value the
	// environment supplies, a conditional takes each branch as a choice, a
	// word selection from a wholly environmental value is that value or
	// nothing, and any other mix retains the whole invocation.
	retain := func() (string, bool) {
		e.ambiguous = true
		return driverComputed, true
	}
	// Certify effects independently of returned values. Only these forms have
	// modeled effects; adding value evaluation for a function is not proof of
	// its read set. Wildcard below must also prove and retain every read edge.
	switch name {
	case "strip", "if", "or", "and", "filter", "filter-out", "findstring",
		"word", "wordlist", "firstword", "lastword", "join", "addprefix",
		"addsuffix", "patsubst", "subst", "sort", "dir", "notdir", "basename",
		"suffix", "error", "warning", "info", "foreach", "origin", "flavor",
		"words", "shell", "wildcard":
	default:
		e.unmodeledEffects = true
	}
	switch name {
	case "strip", "if", "or", "and", "filter", "filter-out", "findstring", "word", "wordlist", "firstword", "lastword", "join":
		e.evaluated = true
		driverMergeReasons(&e.evaluatedBy, map[string]bool{"$(" + name + ")": true})
	}
	switch name {
	case "strip":
		value := e.expand(args)
		if !definite(value) {
			return retain()
		}
		// GNU make collapses ASCII whitespace after expansion, before a
		// surrounding conditional tests the result (AFP-V0-3121).
		return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
			return strings.ContainsRune(" \t\n\r\v\f", r)
		}), " "), true
	case "wildcard":
		// Literal names that exist in the explicit repository source are
		// their own wildcard result. Patterns, absent names and unknown
		// directories stay unknown (AFP-V0-3121), never a truthy argument.
		value := e.expand(args)
		// Only the root makefile's default source-root context is known.
		// A nested file can be invoked with -f from a different directory;
		// its location is not evidence of the invocation's working directory.
		if !definite(value) || e.file.source == nil || e.file.dir != "." {
			e.unmodeledEffects = true
			return retain()
		}
		seen := map[string]bool{}
		for _, word := range strings.Fields(value) {
			// GNU make returns a literal name as spelled. Only a clean
			// spelling (optionally "./"-prefixed) names exactly the
			// recorded path; duplicates and symlinks are not modeled
			// (AFP-V0-9050).
			clean := strings.TrimPrefix(word, "./")
			if seen[word] || clean == "" || path.Clean(clean) != clean || strings.HasPrefix(clean, "../") || clean == ".." {
				e.unmodeledEffects = true
				return retain()
			}
			seen[word] = true
			for _, char := range word {
				if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("/._-+", char)) {
					e.unmodeledEffects = true
					return retain()
				}
			}
			relative := driverJoin(e.file.dir, word)
			if !ValidRelativePath(relative) {
				e.unmodeledEffects = true
				return retain()
			}
			if !driverPlainPath(e.file.source, relative) {
				e.unmodeledEffects = true
				return retain()
			}
			e.readPaths = append(e.readPaths, relative)
		}
		return strings.Join(strings.Fields(value), " "), true
	case "realpath", "abspath":
		// Filesystem results and absolute paths are not their arguments.
		// Keep an unknown value so conditions consider both outcomes.
		e.expand(args)
		return retain()
	case "if":
		parts := split(3)
		truth := e.condition(parts[0])
		if truth < 0 {
			truth = 1 - e.choose(2)
		}
		if truth == 1 {
			return e.expand(parts[1]), true
		}
		return e.expand(parts[2]), true
	case "or", "and":
		value := ""
		for _, part := range driverMakeSplitTop(args, ',', 0) {
			// GNU make strips ASCII whitespace from the unexpanded argument
			// before testing it (AFP-V0-9050). An empty conjunction
			// argument is false, even before expansion.
			trimmed, exact := driverMakeTrim(part)
			if trimmed == "" {
				if name == "and" {
					return "", true
				}
				continue
			}
			value = e.expand(trimmed)
			truth := driverTruth(value)
			if !exact {
				truth = -1
			}
			if truth < 0 {
				// Only the environment decides whether this argument is
				// empty: both outcomes are choices.
				truth = 1 - e.choose(2)
			}
			if name == "or" && truth == 1 {
				return value, true
			}
			if name == "and" && truth == 0 {
				return "", true
			}
		}
		if name == "or" {
			return "", true
		}
		return value, true
	case "filter", "filter-out":
		parts := split(2)
		patterns, text := e.expand(parts[0]), e.expand(parts[1])
		if escaped(patterns) {
			return retain()
		}
		if definite(patterns) && environmental(text) {
			return e.selected()
		}
		if !definite(patterns, text) {
			return retain()
		}
		var out []string
		for _, word := range strings.Fields(text) {
			matched := slices.ContainsFunc(strings.Fields(patterns), func(pattern string) bool {
				return pattern == word || driverPatternMatches(pattern, word)
			})
			if matched == (name == "filter") {
				out = append(out, word)
			}
		}
		return strings.Join(out, " "), true
	case "findstring":
		parts := split(2)
		find, text := e.expand(parts[0]), e.expand(parts[1])
		if !definite(find, text) {
			return retain()
		}
		if strings.Contains(text, find) {
			return find, true
		}
		return "", true
	case "firstword", "lastword":
		text := e.expand(args)
		if environmental(text) {
			return e.selected()
		}
		if !definite(text) {
			return retain()
		}
		fields := strings.Fields(text)
		switch {
		case len(fields) == 0:
			return "", true
		case name == "firstword":
			return fields[0], true
		}
		return fields[len(fields)-1], true
	case "word", "wordlist":
		limit := 2
		if name == "wordlist" {
			limit = 3
		}
		parts := split(limit)
		for index := range parts {
			parts[index] = e.expand(parts[index])
		}
		if definite(parts[:limit-1]...) && environmental(parts[limit-1]) {
			return e.selected()
		}
		if !definite(parts...) {
			return retain()
		}
		start, startErr := driverMakeNumber(parts[0])
		end := start
		var endErr error
		if name == "wordlist" {
			end, endErr = driverMakeNumber(parts[1])
		}
		fields := strings.Fields(parts[limit-1])
		if startErr != nil || endErr != nil || start < 1 || end < 0 {
			return retain()
		}
		if start > len(fields) || end < start {
			return "", true
		}
		return strings.Join(fields[start-1:min(end, len(fields))], " "), true
	case "join":
		parts := split(2)
		left, right := e.expand(parts[0]), e.expand(parts[1])
		if !definite(left, right) {
			return retain()
		}
		first, second := strings.Fields(left), strings.Fields(right)
		out := make([]string, max(len(first), len(second)))
		for index := range out {
			if index < len(first) {
				out[index] = first[index]
			}
			if index < len(second) {
				out[index] += second[index]
			}
		}
		return strings.Join(out, " "), true
	case "error", "warning", "info":
		// Discard the diagnostic value, but preserve argument evaluation effects.
		savedAmbiguous, savedEvaluated, savedEvaluatedBy := e.ambiguous, e.evaluated, e.evaluatedBy
		e.evaluatedBy = nil
		e.expand(args)
		e.ambiguous, e.evaluated, e.evaluatedBy = savedAmbiguous, savedEvaluated, savedEvaluatedBy
		return "", true
	case "addprefix":
		parts := split(2)
		prefix := e.expand(parts[0])
		return mapWords(parts[1], false, each(func(word string) string { return prefix + word }))
	case "addsuffix":
		parts := split(2)
		suffix := e.expand(parts[0])
		return mapWords(parts[1], false, each(func(word string) string { return word + suffix }))
	case "patsubst":
		parts := split(3)
		from, to := e.expand(parts[0]), e.expand(parts[1])
		if !definite(from) || !driverASCII(to) || escaped(from, to) {
			return retain()
		}
		return mapWords(parts[2], true, each(func(word string) string { return driverPatsubst(from, to, word) }))
	case "subst":
		// GNU make substitutes in the raw text, and an empty search string
		// matches once, at the end (AFP-V0-9050).
		parts := split(3)
		from, to, text := e.expand(parts[0]), e.expand(parts[1]), e.expand(parts[2])
		switch {
		case !definite(from) || !driverASCII(to+text):
			return retain()
		case environmental(text):
			return driverComputed, true
		case driverDynamic(text):
			return retain()
		case from == "":
			return text + to, true
		}
		return strings.ReplaceAll(text, from, to), true
	case "sort":
		value := e.expand(args)
		if !driverASCII(value) {
			return retain()
		}
		out := strings.Fields(value)
		sort.Strings(out)
		return strings.Join(slices.Compact(out), " "), true
	// GNU make keeps each word's literal spelling: dir is the prefix through
	// the last slash, else "./"; notdir removes that prefix, so a trailing
	// slash yields an empty word; suffix emits nothing for a word without
	// one. path.Dir would clean the spelling a later filter compares
	// (AFP-V0-9050).
	case "dir":
		return mapWords(args, true, each(func(word string) string {
			if slash := strings.LastIndexByte(word, '/'); slash >= 0 {
				return word[:slash+1]
			}
			return "./"
		}))
	case "notdir":
		return mapWords(args, true, each(func(word string) string { return word[strings.LastIndexByte(word, '/')+1:] }))
	case "basename":
		return mapWords(args, true, each(func(word string) string { return strings.TrimSuffix(word, path.Ext(word)) }))
	case "suffix":
		return mapWords(args, true, func(word string) (string, bool) {
			suffix := path.Ext(word)
			return suffix, suffix != ""
		})
	case "value":
		e.unmodeledEffects = true
		return e.lookup(strings.TrimSpace(e.expand(args))), true
	case "call":
		// Built-in dispatch and computed macro bodies are not bounded here.
		e.unmodeledEffects = true
		parts := driverMakeSplitTop(args, ',', 0)
		macro := strings.TrimSpace(e.expand(parts[0]))
		if driverDynamic(macro) {
			e.unmodeledEffects = true
			return driverComputed, true
		}
		saved := e.locals
		locals := make(map[string]string, len(saved)+len(parts))
		for key, value := range saved {
			locals[key] = value
		}
		locals["0"] = macro
		for index := 1; index < len(parts); index++ {
			locals[string(rune('0'+index%10))] = e.expand(parts[index])
		}
		e.locals = locals
		value := e.lookup(macro)
		e.locals = saved
		return value, true
	case "foreach":
		parts := split(3)
		variable := strings.TrimSpace(e.expand(parts[0]))
		list := e.expand(parts[1])
		if driverDynamic(variable) || driverDynamic(list) {
			e.unmodeledEffects = true
			return driverComputed, true
		}
		items := strings.Fields(list)
		if len(items) > 256 {
			// The unvisited body may run commands even if a caller discards
			// its value. Only fully visited bounded loops certify effects.
			e.unmodeledEffects = true
			return driverComputed, true
		}
		saved, hadSaved := e.locals[variable]
		out := make([]string, 0, len(items))
		for _, item := range items {
			e.locals[variable] = item
			out = append(out, e.expand(parts[2]))
		}
		if hadSaved {
			e.locals[variable] = saved
		} else {
			delete(e.locals, variable)
		}
		return strings.Join(out, " "), true
	}
	switch name {
	case "shell":
		// The command runs while make reads the file; its output is unknown.
		saved := e.ambiguous
		e.ambiguous = false
		script := e.expand(args)
		e.shellAmbiguous = e.shellAmbiguous || e.ambiguous
		e.ambiguous = e.ambiguous || saved
		if !slices.Contains(e.shells, script) {
			e.shells = append(e.shells, script)
		}
		return driverComputed, true
	case "origin", "flavor", "words":
		e.expand(args)
		return driverComputed, true
	}
	// Only the cases above have modeled evaluation effects. Future functions
	// retain uncertainty even when a condition discards their returned value.
	e.unmodeledEffects = true
	return driverComputed, true
}

// driverMakeNumber parses a word or wordlist index as GNU make does: ASCII
// whitespace around decimal digits only. Anything else retains.
func driverMakeNumber(text string) (int, error) {
	trimmed, exact := driverMakeTrim(text)
	if !exact || trimmed == "" || strings.Trim(trimmed, "0123456789") != "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(trimmed)
}

// driverPlainPath reports a repository path whose every component is a
// directory or, for the last, a regular file or directory, with no symlink
// whose target the recorded read edge would not cover (AFP-V0-9050).
func driverPlainPath(source *Source, relative string) bool {
	for index := 0; index <= len(relative); index++ {
		if index < len(relative) && relative[index] != '/' {
			continue
		}
		info, err := source.Lstat(relative[:index])
		if err != nil || info.Mode()&fs.ModeSymlink != 0 || index < len(relative) && !info.IsDir() || !info.IsDir() && !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// driverPatsubst applies one make pattern substitution to word.
func driverPatsubst(from, to, word string) string {
	prefix, suffix, wildcard := strings.Cut(from, "%")
	if !wildcard {
		if word == from {
			return to
		}
		return word
	}
	if len(word) < len(prefix)+len(suffix) || !strings.HasPrefix(word, prefix) || !strings.HasSuffix(word, suffix) {
		return word
	}
	stem := word[len(prefix) : len(word)-len(suffix)]
	if strings.Contains(to, "%") {
		return strings.Replace(to, "%", stem, 1)
	}
	return to
}

// driverPatternMatches reports whether a make pattern such as `ledger/%`
// matches name.
func driverPatternMatches(pattern, name string) bool {
	prefix, suffix, wildcard := strings.Cut(pattern, "%")
	return wildcard && len(name) >= len(prefix)+len(suffix) && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix)
}
