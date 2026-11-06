package affected

import (
	"path"
	"slices"
	"sort"
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
}

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
			continue
		case first == "else":
			if conditional == 0 {
				parsed.unparsable = true
				return parsed
			}
			parsed.remainder = append(parsed.remainder, trimmed)
			continue
		case first == "endif":
			conditional--
			if conditional < 0 {
				parsed.unparsable = true
				return parsed
			}
			parsed.remainder = append(parsed.remainder, trimmed)
			continue
		case first == "define":
			endRule()
			parsed.remainder = append(parsed.remainder, trimmed)
			defineName = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(rest), ":!?+="))
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
			if assignment, name, value := driverMakeAssignment(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(after), "export ")), driverMakeOperator(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(after), "export ")))); assignment && name != "" {
				parsed.vars[name] = append(parsed.vars[name], value)
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

// driverExpander expands make references, retaining ambiguous assignments
// as computed rather than joining alternatives into a different shell word.
// Both branches of a conditional are considered. What it cannot evaluate
// becomes driverComputed.
type driverExpander struct {
	file      *driverMakeFile
	auto      map[string]string
	locals    map[string]string
	visiting  map[string]bool
	depth     int
	ambiguous bool
	// shells are the expanded commands of every `$(shell ...)` reached.
	shells []string
}

const driverExpandLimit = 1 << 16

func (parsed *driverMakeFile) expander(auto map[string]string) *driverExpander {
	return &driverExpander{file: parsed, auto: auto, locals: map[string]string{}, visiting: map[string]bool{}}
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
	values, present := e.file.vars[name]
	if !present {
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
	expanded := make([]string, 0, len(values))
	for _, value := range values {
		expanded = append(expanded, e.expand(value))
	}
	// Assignment operators are not retained: equal values can still be an
	// append whose word boundaries matter when the reference has a suffix.
	if len(expanded) != 1 {
		e.ambiguous = true
		return driverComputed
	}
	return expanded[0]
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
	switch name {
	case "if", "or", "and", "filter", "filter-out", "findstring", "word", "wordlist", "firstword", "lastword", "join":
		// AZU-V0-9850: a superset of words is not conservative when a
		// literal prefix or suffix attaches to a different word. Until these
		// functions are evaluated exactly, retain the whole invocation.
		e.ambiguous = true
		return driverComputed, true
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
		e.shells = append(e.shells, e.expand(args))
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
