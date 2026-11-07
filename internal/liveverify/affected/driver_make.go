package affected

import (
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
	// events are the assignments and conditional lines in file order, from
	// which a variable's alternative values are derived (AFP-V0-3121).
	events       []driverMakeEvent
	alternatives map[string][]string
	// statements are the lines that are a bare reference, whose expansion
	// make reads again as make file text.
	statements []string
}

// driverMakeEvent is one assignment or conditional line of a make file.
// Conditions are never evaluated: every branch is an alternative.
type driverMakeEvent struct {
	kind byte
	// bare marks an `else` with no condition of its own, which makes the
	// chain exhaustive.
	bare bool
	// scoped marks a target-specific assignment, in force for some targets
	// only, so it is an alternative beside the value it replaces.
	scoped          bool
	name, op, value string
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
	recipe  []string
	block   []string
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
					parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventAssign, name: defineName, op: defineOp, value: strings.Join(defineBody, "\n")})
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
		first, rest, _ := strings.Cut(trimmed, " ")
		rest = strings.TrimSpace(rest)
		for first == "export" || first == "override" || first == "private" || first == "unexport" {
			if rest == "" || driverMakeOperator(rest) < 0 && !strings.HasPrefix(rest, "define") {
				break
			}
			first, rest, _ = strings.Cut(rest, " ")
			rest = strings.TrimSpace(rest)
		}
		switch {
		case driverMakeConditionals[first]:
			conditional++
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventIf})
			continue
		case first == "else":
			if conditional == 0 {
				parsed.unparsable = true
				return parsed
			}
			parsed.remainder = append(parsed.remainder, trimmed)
			parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventElse, bare: rest == ""})
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
			defineOp = driverMakeDefineOperator(rest)
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
				continue
			}
			parsed.unparsable = true
			return parsed
		}
		if assignment, name, value := driverMakeAssignment(statement, operator); assignment {
			endRule()
			parsed.remainder = append(parsed.remainder, trimmed)
			if name != "" {
				parsed.vars[name] = append(parsed.vars[name], value)
				parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventAssign, name: name, op: driverMakeAssignmentOperator(statement, operator), value: value})
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

// driverMakeAssignment classifies a statement whose first operator is at
// index: `=`, `:=`, `::=`, `:::=`, `?=`, `+=` and `!=` assign.
func driverMakeAssignment(statement string, index int) (bool, string, string) {
	name := statement[:index]
	op := statement[index:]
	switch {
	case op[0] == '=':
		op = "="
	case strings.HasPrefix(op, ":=") || strings.HasPrefix(op, "::=") || strings.HasPrefix(op, ":::="):
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

// driverMakeAssignmentOperator is the kind of the assignment whose first
// operator is at index: "?=", "+=", "!=" or "=" for every plain or immediate
// form.
func driverMakeAssignmentOperator(statement string, index int) string {
	name := strings.TrimRight(statement[:index], " \t")
	if name != "" && strings.IndexByte("?+!", name[len(name)-1]) >= 0 {
		return name[len(name)-1:] + "="
	}
	return "="
}

// driverMakeDefineOperator is the kind of a `define NAME [op]` line.
func driverMakeDefineOperator(rest string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(rest), ":=")
	if trimmed != "" && strings.IndexByte("?+!", trimmed[len(trimmed)-1]) >= 0 && strings.HasSuffix(strings.TrimSpace(rest), "=") {
		return trimmed[len(trimmed)-1:] + "="
	}
	return "="
}

// driverMaxAlternatives bounds the distinct values one variable can take; a
// variable past it is retained as ambiguous (AZU-V0-9702).
const driverMaxAlternatives = 16

// driverUndefined stands for a variable no assignment has set, which the
// environment or command line may supply.
const driverUndefined = "\x03"

// variableAlternatives is every value the file can leave in name once make
// has read it, each as unexpanded text, or driverUndefined where no
// assignment need run (AFP-V0-3121). Every branch of a conditional is taken,
// a branch chain without a bare `else` may also take none, and a
// target-specific assignment stands beside the value it replaces. ok is false
// past driverMaxAlternatives.
func (parsed *driverMakeFile) variableAlternatives(name string) ([]string, bool) {
	if parsed.alternatives == nil {
		parsed.alternatives = map[string][]string{}
	}
	if values, cached := parsed.alternatives[name]; cached {
		return values, values != nil
	}
	type frame struct {
		entry, done []string
		bare        bool
	}
	current := []string{driverUndefined}
	var stack []frame
	union := func(sets ...[]string) []string {
		var out []string
		for _, set := range sets {
			for _, value := range set {
				if !slices.Contains(out, value) {
					out = append(out, value)
				}
			}
		}
		return out
	}
	for _, event := range parsed.events {
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
			next := make([]string, 0, len(current))
			for _, value := range current {
				switch {
				case event.op == "?=" && value != driverUndefined:
					next = append(next, value)
				case event.op == "+=" && value != driverUndefined:
					next = append(next, value+" "+event.value)
				default:
					next = append(next, event.value)
				}
			}
			if event.scoped {
				next = union(current, next)
			}
			current = union(next)
		}
		if len(current) > driverMaxAlternatives {
			parsed.alternatives[name] = nil
			return nil, false
		}
	}
	parsed.alternatives[name] = current
	return current, true
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
		if after[operator] == ':' && (operator+1 >= len(after) || after[operator+1] != '=') {
			return false
		}
		targetSpecific = true
	}
	targets := strings.Fields(targetText)
	special := false
	for _, target := range targets {
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
			statement := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(after), "export "))
			operator := driverMakeOperator(statement)
			if assignment, name, value := driverMakeAssignment(statement, operator); assignment && name != "" {
				parsed.vars[name] = append(parsed.vars[name], value)
				parsed.events = append(parsed.events, driverMakeEvent{kind: driverEventAssign, scoped: true, name: name, op: driverMakeAssignmentOperator(statement, operator), value: value})
			}
		} else {
			rule.prereqs = append(rule.prereqs, after)
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
	file      *driverMakeFile
	auto      map[string]string
	locals    map[string]string
	visiting  map[string]bool
	depth     int
	ambiguous bool
	// shellAmbiguous marks an ambiguous expansion inside a `$(shell ...)`
	// command, which make runs whatever target is built.
	shellAmbiguous bool
	// evaluated marks an expansion that resolved what the claimant before
	// AFP-V0-3121 retained as ambiguous: a reassigned variable or one of the
	// conditional and word functions. Such a target stays a computed site
	// unless every command it runs is certified (AFP-V0-3122).
	evaluated bool
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
	return &driverExpander{file: parsed, auto: auto, locals: map[string]string{}, visiting: map[string]bool{}, picked: map[string]int{}}
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
	value := e.expand(strings.TrimSpace(text))
	unknown := e.ambiguous
	e.ambiguous = saved
	if unknown {
		return -1
	}
	return driverTruth(value)
}

// definite reports values that hold no part only the environment supplies.
func definite(values ...string) bool {
	for _, value := range values {
		if driverDynamic(value) {
			return false
		}
	}
	return true
}

// environmental reports a value made only of parts the environment supplies:
// any word selected from it is environment-supplied too, or there is none.
func environmental(value string) bool {
	return strings.Contains(value, driverComputed) && strings.TrimSpace(strings.ReplaceAll(value, driverComputed, "")) == ""
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

var driverMakePassThrough = map[string]bool{"wildcard": true, "abspath": true, "realpath": true, "strip": true}

func (e *driverExpander) reference(inner string) string {
	name, args, spaced := strings.Cut(inner, " ")
	if !spaced {
		name, args, spaced = strings.Cut(inner, "\t")
	}
	if spaced && !strings.Contains(name, "$") {
		if value, known := e.function(name, args); known {
			return value
		}
	}
	if colon := strings.IndexByte(inner, ':'); colon > 0 && strings.Contains(inner[colon:], "=") && !strings.Contains(inner[:colon], "$") {
		from, to, _ := strings.Cut(inner[colon+1:], "=")
		words := strings.Fields(e.lookup(inner[:colon]))
		for index, word := range words {
			if strings.Contains(from, "%") {
				words[index] = driverPatsubst(from, to, word)
			} else if strings.HasSuffix(word, from) {
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
	if len(e.file.vars[name]) > 1 {
		e.evaluated = true
	}
	values, bounded := e.file.variableAlternatives(name)
	if !bounded {
		e.ambiguous = true
		return driverComputed
	}
	// Each alternative is one complete value: the expansion in progress
	// takes one of them for every reference to name (AFP-V0-3121).
	value := values[e.pick(name, len(values))]
	if value == driverUndefined {
		if name == "SHELL" {
			return "sh"
		}
		return driverComputed
	}
	if e.visiting[name] {
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
	words := func(text string) []string { return strings.Fields(e.expand(text)) }
	mapWords := func(text string, transform func(string) string) string {
		out := words(text)
		for index, word := range out {
			if !driverDynamic(word) {
				out[index] = transform(word)
			}
		}
		return strings.Join(out, " ")
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
	switch name {
	case "if", "or", "and", "filter", "filter-out", "findstring", "word", "wordlist", "firstword", "lastword", "join":
		e.evaluated = true
	}
	switch name {
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
			value = e.expand(strings.TrimSpace(part))
			truth := driverTruth(value)
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
		start, startErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		end := start
		var endErr error
		if name == "wordlist" {
			end, endErr = strconv.Atoi(strings.TrimSpace(parts[1]))
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
		return "", true
	case "addprefix":
		parts := split(2)
		prefix := e.expand(parts[0])
		return mapWords(parts[1], func(word string) string { return prefix + word }), true
	case "addsuffix":
		parts := split(2)
		suffix := e.expand(parts[0])
		return mapWords(parts[1], func(word string) string { return word + suffix }), true
	case "patsubst":
		parts := split(3)
		from, to := e.expand(parts[0]), e.expand(parts[1])
		return mapWords(parts[2], func(word string) string { return driverPatsubst(from, to, word) }), true
	case "subst":
		parts := split(3)
		from, to := e.expand(parts[0]), e.expand(parts[1])
		if from == "" {
			return e.expand(parts[2]), true
		}
		return strings.ReplaceAll(e.expand(parts[2]), from, to), true
	case "sort":
		out := words(args)
		sort.Strings(out)
		return strings.Join(slices.Compact(out), " "), true
	case "dir":
		return mapWords(args, func(word string) string { return path.Dir(word) + "/" }), true
	case "notdir":
		return mapWords(args, path.Base), true
	case "basename":
		return mapWords(args, func(word string) string { return strings.TrimSuffix(word, path.Ext(word)) }), true
	case "suffix":
		return mapWords(args, path.Ext), true
	case "value":
		return e.lookup(strings.TrimSpace(e.expand(args))), true
	case "call":
		parts := driverMakeSplitTop(args, ',', 0)
		macro := strings.TrimSpace(e.expand(parts[0]))
		if driverDynamic(macro) {
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
			return driverComputed, true
		}
		items := strings.Fields(list)
		if len(items) > 256 {
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
	if driverMakePassThrough[name] {
		return e.expand(args), true
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
	case "origin", "flavor", "file", "guile", "let", "intcmp", "words", "eval":
		return driverComputed, true
	}
	return "", false
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
