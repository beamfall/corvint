// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// MakeEdit is the attribution of one make file edit to the explicit rules it
// changes (NCM-V1-010 to NCM-V1-012). When every changed line is
// attributable, Targets holds the changed rules' names, sorted; otherwise
// Reason names the first change that is not, and Targets is nil.
type MakeEdit struct {
	Targets []string
	Reason  string
}

// AttributeMakeEdit attributes the edit of the make file named file from base
// to head with the driver model's own parse (AZU-V0-001), never a second one.
// A changed line is attributable when it lies in an explicit rule's text (its
// header, prerequisites, target-specific assignments, recipe or .PHONY
// membership) or is a plain top-level assignment of a variable every use site
// of which is such text, or another such variable's plain assignment
// (NCM-V1-011). Anything else refuses (NCM-V1-012): either side outside the
// parsed subset ($(eval), .ONESHELL, .RECIPEPREFIX, unbalanced blocks); a
// changed include, conditional, define block, export, override, private,
// vpath, special-target, computed-target or default-goal line; a removed
// rule; a changed pattern, `::` or conditional rule; a changed variable that
// make or the environment reads (a special or built-in-rule name, an exported
// name, a file that exports what it does not model), that is used outside a
// rule or by no rule, whose changed assignment calls a function with effects
// or runs a shell, or any changed variable in a file that names a variable
// by a computed reference.
func AttributeMakeEdit(file string, base, head []byte) MakeEdit {
	before, after := parseDriverMake(file, base), parseDriverMake(file, head)
	switch {
	case before.unparsable:
		return makeRefusal("the make file at the base is outside the parsed subset")
	case after.unparsable:
		return makeRefusal("the make file at the head is outside the parsed subset")
	}
	if line, changed := makeFirstDifference(before.residual(), after.residual()); changed {
		return makeRefusal(makeLineKind(line) + " changed: " + makeQuote(line))
	}
	targets := map[string]bool{}
	for _, name := range makeRuleNames(before, after) {
		old, inBase := before.rules[name]
		updated, inHead := after.rules[name]
		switch {
		case !inHead:
			return makeRefusal("make target " + makeQuote(name) + " was removed")
		case !inBase || !slices.Equal(old.block, updated.block):
			targets[name] = true
		}
	}
	if reason := attributeMakeVariables(before, after, targets); reason != "" {
		return makeRefusal(reason)
	}
	names := make([]string, 0, len(targets))
	for name := range targets {
		for _, parsed := range []*driverMakeFile{before, after} {
			rule, present := parsed.rules[name]
			switch {
			case !present:
			case rule.pattern:
				return makeRefusal("pattern rule " + makeQuote(name) + " changed")
			case rule.doubleColon:
				return makeRefusal("double-colon rule " + makeQuote(name) + " changed")
			case rule.conditional:
				return makeRefusal("rule " + makeQuote(name) + " inside a conditional changed")
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return MakeEdit{Targets: names}
}

func makeRefusal(reason string) MakeEdit { return MakeEdit{Reason: reason} }

// residual is every remainder line but the plain assignments.
func (parsed *driverMakeFile) residual() []string {
	skip := make(map[int]bool, len(parsed.plain))
	for _, plain := range parsed.plain {
		skip[plain.at] = true
	}
	lines := make([]string, 0, len(parsed.remainder))
	for index, line := range parsed.remainder {
		if !skip[index] {
			lines = append(lines, line)
		}
	}
	return lines
}

// assignments is each variable's plain assignment lines, in file order.
func (parsed *driverMakeFile) assignments() map[string][]string {
	out := map[string][]string{}
	for _, plain := range parsed.plain {
		out[plain.name] = append(out[plain.name], parsed.remainder[plain.at])
	}
	return out
}

// makeFirstDifference returns the first line where two sequences differ,
// from the head side when it has one.
func makeFirstDifference(base, head []string) (string, bool) {
	for index := 0; index < len(base) || index < len(head); index++ {
		switch {
		case index >= len(head):
			return base[index], true
		case index >= len(base) || base[index] != head[index]:
			return head[index], true
		}
	}
	return "", false
}

func makeRuleNames(before, after *driverMakeFile) []string {
	seen := map[string]bool{}
	for _, parsed := range []*driverMakeFile{before, after} {
		for name := range parsed.rules {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// makeGlobalNames are variables make itself reads, so a change to one
// reaches every target (NCM-V1-012). Names starting with `.` or `MAKE` are
// global as well.
var makeGlobalNames = map[string]bool{
	"SHELL": true, "VPATH": true, "GPATH": true, "SUFFIXES": true, "CURDIR": true,
	"GNUMAKEFLAGS": true, "MFLAGS": true,
}

// makeBuiltinRuleNames are variables GNU make's built-in implicit rules read:
// a target with no recipe of its own may be built by one of them.
var makeBuiltinRuleNames = map[string]bool{
	"AR": true, "ARFLAGS": true, "AS": true, "ASFLAGS": true, "CC": true, "CFLAGS": true, "CO": true,
	"COFLAGS": true, "CPP": true, "CPPFLAGS": true, "CTANGLE": true, "CWEAVE": true, "CXX": true,
	"CXXFLAGS": true, "FC": true, "FFLAGS": true, "GET": true, "GFLAGS": true, "LDFLAGS": true,
	"LDLIBS": true, "LEX": true, "LFLAGS": true, "LINT": true, "LINTFLAGS": true, "LOADLIBES": true,
	"M2C": true, "MAKEINFO": true, "OUTPUT_OPTION": true, "PC": true, "PFLAGS": true, "RFLAGS": true,
	"RM": true, "TANGLE": true, "TEX": true, "TEXI2DVI": true, "WEAVE": true, "YACC": true, "YFLAGS": true,
}

// makeEffectFunctions are the make functions that run a command, print,
// stop make, write a file or evaluate text as make syntax.
var makeEffectFunctions = []string{"shell", "error", "warning", "info", "file", "eval", "call", "guile", "value"}

// attributeMakeVariables adds to targets each rule a changed plain variable
// reaches, through other plain variables, and returns why a change cannot be
// attributed, if one cannot (NCM-V1-011, NCM-V1-012).
func attributeMakeVariables(before, after *driverMakeFile, targets map[string]bool) string {
	old, updated := before.assignments(), after.assignments()
	var pending []string
	for name := range old {
		if !slices.Equal(old[name], updated[name]) {
			pending = append(pending, name)
		}
	}
	for name := range updated {
		if _, present := old[name]; !present {
			pending = append(pending, name)
		}
	}
	if len(pending) == 0 {
		return ""
	}
	sort.Strings(pending)
	for _, parsed := range []*driverMakeFile{before, after} {
		if parsed.unmodeledExports {
			return "variable " + makeQuote(pending[0]) + " changed in a make file that exports variables it does not model"
		}
		if line, computed := parsed.computedReference(); computed {
			return "variable " + makeQuote(pending[0]) + " changed in a make file that names a variable by a computed reference: " + makeQuote(line)
		}
	}
	for _, name := range pending {
		changed := makeSymmetricDifference(old[name], updated[name])
		for _, line := range changed {
			if reason := makeAssignmentEffect(name, line); reason != "" {
				return reason
			}
		}
	}
	visited := map[string]bool{}
	for len(pending) != 0 {
		name := pending[0]
		pending = pending[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		switch {
		case strings.HasPrefix(name, ".") || strings.HasPrefix(name, "MAKE") || makeGlobalNames[name]:
			return "variable " + makeQuote(name) + " is a global make setting"
		case makeBuiltinRuleNames[name]:
			return "variable " + makeQuote(name) + " is read by make's built-in rules"
		}
		uses := 0
		for _, parsed := range []*driverMakeFile{before, after} {
			if slices.Contains(parsed.exportedNames, name) {
				return "variable " + makeQuote(name) + " is exported to every recipe"
			}
			for _, rule := range parsed.rules {
				if slices.ContainsFunc(rule.block, func(line string) bool { return makeReferences(line, name) }) {
					targets[rule.name] = true
					uses++
				}
			}
			for _, plain := range parsed.plain {
				if plain.name != name && makeReferences(parsed.remainder[plain.at], name) {
					pending = append(pending, plain.name)
					uses++
				}
			}
			for _, line := range parsed.residual() {
				if makeReferences(line, name) {
					return "variable " + makeQuote(name) + " is used outside every explicit rule: " + makeQuote(line)
				}
			}
		}
		if uses == 0 {
			return "variable " + makeQuote(name) + " is used by no rule of the file"
		}
	}
	return ""
}

// makeAssignmentEffect names why a changed assignment line of name cannot be
// attributed: it runs a shell (`!=`) or calls a function with effects.
func makeAssignmentEffect(name, line string) string {
	if operator := driverMakeOperator(line); operator > 0 && line[operator-1] == '!' {
		return "variable " + makeQuote(name) + " runs a shell command where make reads the file"
	}
	for _, function := range makeEffectFunctions {
		if strings.Contains(line, "$("+function+" ") || strings.Contains(line, "${"+function+" ") {
			return "variable " + makeQuote(name) + " calls $(" + function + ")"
		}
	}
	return ""
}

func makeSymmetricDifference(base, head []string) []string {
	var out []string
	for _, line := range head {
		if !slices.Contains(base, line) {
			out = append(out, line)
		}
	}
	for _, line := range base {
		if !slices.Contains(head, line) {
			out = append(out, line)
		}
	}
	return out
}

// makeNameByte reports a byte a variable name token continues with.
func makeNameByte(char byte) bool {
	return char == '_' || char == '.' || char == '-' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

// makeReferences reports whether line holds name as a whole token: any use,
// a `$(name)`, `${name}`, `$(name:a=b)` reference or a bare word (a
// function's variable argument), so the answer can only over-approximate
// the line's references.
func makeReferences(line, name string) bool {
	for start := 0; ; {
		index := strings.Index(line[start:], name)
		if index < 0 {
			return false
		}
		index += start
		end := index + len(name)
		if (index == 0 || !makeNameByte(line[index-1])) && (end == len(line) || !makeNameByte(line[end])) {
			return true
		}
		start = index + 1
	}
}

// computedReference returns the first line of the file holding a reference
// whose variable name is itself expanded (`$($(X))`, `$(A_$(B))`), or whose
// value, origin, flavor or call argument is.
func (parsed *driverMakeFile) computedReference() (string, bool) {
	lines := slices.Clone(parsed.remainder)
	for _, name := range parsed.order {
		lines = append(lines, parsed.rules[name].block...)
	}
	for _, line := range lines {
		for index := 0; index+1 < len(line); index++ {
			if line[index] != '$' || line[index+1] != '(' && line[index+1] != '{' {
				continue
			}
			inner := line[index+2:]
			name, rest := makeReferenceName(inner)
			if strings.Contains(name, "$") {
				return line, true
			}
			switch name {
			case "value", "origin", "flavor", "call":
				argument, _ := makeReferenceName(strings.TrimLeft(rest, " \t"))
				if strings.Contains(argument, "$") || argument == "" {
					return line, true
				}
			}
		}
	}
	return "", false
}

// makeReferenceName is the text of a reference up to its first blank, colon,
// comma or closing bracket outside nested brackets, and what follows.
func makeReferenceName(text string) (string, string) {
	depth := 0
	for index := 0; index < len(text); index++ {
		switch char := text[index]; {
		case char == '(' || char == '{':
			depth++
		case (char == ')' || char == '}') && depth == 0, depth == 0 && (char == ' ' || char == '\t' || char == ':' || char == ','):
			return text[:index], text[index:]
		case char == ')' || char == '}':
			depth--
		}
	}
	return text, ""
}

// makeLineKind names what a changed remainder line is.
func makeLineKind(line string) string {
	trimmed := strings.TrimSpace(line)
	first, _ := driverMakeFirst(trimmed)
	switch {
	case first == "include" || first == "-include" || first == "sinclude":
		return "include line"
	case driverMakeConditionals[first] || first == "else" || first == "endif":
		return "conditional"
	case first == "define" || first == "endef":
		return "define block"
	case first == "export" || first == "unexport":
		return "export line"
	case first == "override" || first == "private":
		return first + " assignment"
	case first == "vpath":
		return "vpath line"
	case strings.HasPrefix(trimmed, "default-goal="):
		return "default goal"
	case strings.HasPrefix(trimmed, ".") && strings.Contains(trimmed, ":"):
		return "special target"
	case strings.Contains(trimmed, "$") && strings.Contains(trimmed, ":"):
		return "computed or conditional line"
	}
	return "line outside every explicit rule"
}

// makeQuote quotes text for a reason line, at most 96 bytes of it, with
// control characters escaped.
func makeQuote(text string) string {
	if len(text) > 96 {
		text = text[:96] + "..."
	}
	return strconv.QuoteToASCII(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text))
}

// MakeReach is what the changed targets of one make file can run
// (NCM-V1-013, NCM-V1-014).
type MakeReach struct {
	// Paths are the paths the reached make units run or read.
	Paths []string
	// Workflows are the workflow files a job of which invokes a changed
	// target directly.
	Workflows []string
	// Reason, when not empty, names why the reach cannot be bounded.
	Reason string
}

// ReachMakeTargets follows the driver units of root (AZU-V0-001) from each
// changed target of file through its prerequisites and the make invocations
// of its recipes, and collects what each reached unit runs or reads. The
// reach is what those targets can run, so it is not bounded when a reached
// unit's commands or make goals did not resolve, when its file is outside
// the parsed subset, or when it imports a unit the driver units do not hold;
// neither is it when a local action invokes a changed target or a workflow
// is outside the parsed subset. A unit whose resolved commands read what the
// graph does not bound (an evaluated, uncertified computed site, or a
// whole-tree read) still runs only what it names, so it stays bounded
// (NCM-V1-013). A workflow job that invokes a changed target directly names
// its workflow (NCM-V1-014).
func ReachMakeTargets(ctx context.Context, root, file string, targets []string) MakeReach {
	return reachMakeTargets(DiskSourceContext(ctx, root), file, targets)
}

// reachMakeTargets is ReachMakeTargets over source.
func reachMakeTargets(source *Source, file string, targets []string) MakeReach {
	unresolved := map[string]bool{}
	result, err := driverUnitsObserved(source, nil, unresolved)
	if err != nil {
		return MakeReach{Reason: "the driver units cannot be read: " + makeQuote(err.Error())}
	}
	units := make(map[string]Unit, len(result.Units))
	for _, unit := range result.Units {
		units[unit.ID] = unit
	}
	changed := map[string]bool{}
	pending := make([]string, 0, len(targets))
	for _, target := range targets {
		id := driverMakeUnitID(file, target)
		changed[id] = true
		pending = append(pending, id)
	}
	reached := map[string]bool{}
	paths := map[string]bool{}
	for len(pending) != 0 {
		id := pending[0]
		pending = pending[1:]
		if reached[id] {
			continue
		}
		reached[id] = true
		unit, present := units[id]
		_, target, _ := strings.Cut(id, "#")
		switch {
		case !present:
			return MakeReach{Reason: "make unit " + makeQuote(id) + " is not in the driver units"}
		case slices.Contains(unit.Frontier, FrontierDriverUnparsableFile):
			return MakeReach{Reason: "make unit " + makeQuote(id) + " is in a file outside the parsed subset"}
		case unresolved[id]:
			return MakeReach{Reason: "make target " + makeQuote(target) + " runs a command or make goal that does not resolve"}
		}
		for _, value := range slices.Concat(unit.PathImports, unit.BuildPathImports) {
			paths[value] = true
		}
		pending = append(pending, unit.Imports...)
	}
	workflows := map[string]bool{}
	for _, unit := range result.Units {
		rest, ok := strings.CutPrefix(unit.ID, DriverNamespace+":")
		if !ok || strings.HasPrefix(rest, "make:") {
			continue
		}
		file, _ := driverUnitFile(unit.ID)
		if strings.HasSuffix(unit.ID, "#*") {
			return MakeReach{Reason: "workflow " + makeQuote(file) + " is outside the parsed subset"}
		}
		if !slices.ContainsFunc(unit.Imports, func(id string) bool { return changed[id] }) {
			continue
		}
		if !strings.HasPrefix(rest, "ci:") {
			return MakeReach{Reason: "local action " + makeQuote(file) + " invokes a changed make target"}
		}
		workflows[file] = true
	}
	return MakeReach{Paths: driverSorted(paths), Workflows: driverSorted(workflows)}
}
