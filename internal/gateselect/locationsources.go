// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

// This file is the location-source pass of AFP-V0-3660 to AFP-V0-3664
// (V1-3999). It is kept identical, but for its package clause, in
// internal/gateselect and internal/liveverify/affected/golang, and
// TestLocationSourceReads_AFPV03660 holds the two rule (d) sets equal.
//
// A path a test builds at run time carries no string literal, so the literal
// lexicon of rule (c) cannot name it. Such a path has to start from a value
// that discloses a location: the working directory, the file of a stack
// frame, or the environment. The pass is therefore keyed on sources, not on
// reads: a reference to a source holds the package under rule (d) unless
// every value the reference yields is used in one of the few ways listed in
// sourceValueSafe, none of which can form a path. A use the list does not
// name, a source used as a function value, a value bound to a package-level
// variable and a file that does not parse all hold the package, so a missing
// entry costs precision and never a test.

// locationSourceReason is the rule (d) reason of a file that uses source's
// value as more than a flag, after the file name.
func locationSourceReason(source string) string {
	if source == linknameSource || source == cgoSource {
		return "has " + source + ", which can reach a location source the scan cannot see"
	}
	return "calls " + source + ", whose value may name a repository path"
}

// linknameSource and cgoSource are the two ways a file can call a location
// source without naming it: a //go:linkname directive binds a local name to
// any function, and C code reads the environment and the working directory
// itself (AFP-V0-3660).
const (
	linknameSource = "a //go:linkname directive"
	cgoSource      = `cgo (import "C")`
)

// linknameDirective reports a //go:linkname comment that starts a line, the
// only place the compiler reads it; text inside a string literal is none.
func linknameDirective(body []byte) bool {
	if !bytes.Contains(body, []byte("//go:linkname")) {
		return false
	}
	var lexer scanner.Scanner
	file := token.NewFileSet().AddFile("", -1, len(body))
	lexer.Init(file, body, nil, scanner.ScanComments)
	for {
		position, kind, text := lexer.Scan()
		if kind == token.EOF {
			return false
		}
		if kind == token.COMMENT && file.Position(position).Column == 1 && (text == "//go:linkname" || strings.HasPrefix(text, "//go:linkname ") || strings.HasPrefix(text, "//go:linkname\t")) {
			return true
		}
	}
}

// locationSourceKind says what a source yields and so which uses are safe.
type locationSourceKind int

const (
	sourceString      locationSourceKind = iota // one string result: a path or an environment value
	sourceLookup                                // (string, bool): result 0 is the value
	sourceEnvironment                           // []string of NAME=value entries
	sourceCallerFile                            // runtime.Caller: result 1 is a file path
	sourceFunc                                  // *runtime.Func: FileLine result 0 is a file path
	sourceFrames                                // *runtime.Frames: Next result 0 is a frame
	sourceFrame                                 // runtime.Frame: its File is a file path
	sourceBounded                               // a location joined to plain literal components
	sourceAlways                                // a working directory: no use is safe
)

// locationSourceFuncs maps an import path to its location sources.
// runtime.Callers yields program counters only; a path needs FuncForPC or
// CallersFrames, which are listed.
var locationSourceFuncs = map[string]map[string]locationSourceKind{
	"os":      {"Getenv": sourceString, "LookupEnv": sourceLookup, "Environ": sourceEnvironment, "ExpandEnv": sourceString},
	"syscall": {"Getenv": sourceLookup, "Environ": sourceEnvironment, "Getwd": sourceAlways},
	"runtime": {"Caller": sourceCallerFile, "FuncForPC": sourceFunc, "CallersFrames": sourceFrames},
}

// parseFuncs are the calls whose result is a number, a boolean or a duration
// whatever string they parse: import path to function names.
var parseFuncs = map[string]map[string]bool{
	"strconv": {"Atoi": true, "ParseBool": true, "ParseInt": true, "ParseUint": true, "ParseFloat": true},
	"time":    {"ParseDuration": true},
}

// environmentEntryPrefix is a literal that starts one NAME=value entry of a
// child process environment.
var environmentEntryPrefix = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=$`)

// locationSources returns, sorted, each location source of body whose value
// may form a path, as `os.Getenv` or `os.Getenv (dot import)`. imports maps
// every local qualifier of the file to its import path and dotImported marks
// the dot imports (qualifiedImports over every import). body may start after
// the imports; a synthetic package clause lets it parse.
func locationSources(body []byte, imports map[string]string, dotImported map[string]bool) []string {
	return sortedKeys(locationSourceUses(body, imports, dotImported))
}

// sourceUse is one reference to a location source that is not proven safe.
// Commands is nil when the reference holds the package outright. Otherwise
// every use of its value is safe but for the working directory of the
// commands built at these byte offsets (the offset of each os/exec
// Command or CommandContext selector), so it holds the package unless each of
// them is a proven `go list` call (AFP-V0-3664).
type sourceUse struct {
	Offset   int
	Commands []int
}

// locationHolds reports whether a source's uses hold the package whatever
// the commands it reaches are: a source named by a file that does not parse
// (no use), or one use that is not contingent on a command.
func locationHolds(uses []sourceUse) bool {
	for _, use := range uses {
		if use.Commands == nil {
			return true
		}
	}
	return len(uses) == 0
}

// locationSourceUses maps each location source of body whose value may form
// a path to its references that are not proven safe; a file that does not
// parse maps each source it names to no reference.
func locationSourceUses(body []byte, imports map[string]string, dotImported map[string]bool) map[string][]sourceUse {
	found := map[string][]sourceUse{}
	for _, importPath := range imports {
		if importPath == "C" {
			found[cgoSource] = nil
		}
	}
	if linknameDirective(body) {
		found[linknameSource] = nil
	}
	qualified := map[string]string{}
	for qualifier, importPath := range imports {
		if locationSourceFuncs[importPath] != nil {
			qualified[qualifier] = importPath
		}
	}
	dotted := map[string]string{} // source name -> import path, for dot imports
	for importPath := range dotImported {
		for name := range locationSourceFuncs[importPath] {
			dotted[name] = importPath
		}
	}
	if len(qualified) == 0 && len(dotted) == 0 {
		return found
	}
	mentioned := false
	for _, importPath := range qualified {
		for name := range locationSourceFuncs[importPath] {
			mentioned = mentioned || bytes.Contains(body, []byte(name))
		}
	}
	for name := range dotted {
		mentioned = mentioned || bytes.Contains(body, []byte(name))
	}
	if !mentioned {
		return found
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "", body, 0)
	shift := 0
	if err != nil {
		const clause = "package locationsources\n"
		positions = token.NewFileSet()
		file, err = parser.ParseFile(positions, "", append([]byte(clause), body...), 0)
		shift = len(clause)
	}
	if err != nil {
		// A file that does not parse proves no use safe: hold every source it
		// names (AFP-V0-3662).
		for qualifier, importPath := range qualified {
			for name := range locationSourceFuncs[importPath] {
				if bytes.Contains(body, []byte(qualifier+"."+name)) {
					found[importPath+"."+name] = nil
				}
			}
		}
		for name, importPath := range dotted {
			if bytes.Contains(body, []byte(name)) {
				found[importPath+"."+name+" (dot import)"] = nil
			}
		}
		return found
	}
	flow := sourceFlow{parents: map[ast.Node]ast.Node{}, packageLevel: map[*ast.Object]bool{}, imports: imports, visiting: map[*ast.Object]bool{}, commands: new([]int)}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) != 0 {
			flow.parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	for _, declaration := range file.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok {
			for _, spec := range general.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range value.Names {
						if name.Obj != nil {
							flow.packageLevel[name.Obj] = true
						}
					}
				}
			}
		}
	}
	flow.uses = map[*ast.Object][]*ast.Ident{}
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Obj != nil {
			flow.uses[ident.Obj] = append(flow.uses[ident.Obj], ident)
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		var reference ast.Expr
		var importPath, name, label string
		switch expression := node.(type) {
		case *ast.SelectorExpr:
			qualifier, ok := expression.X.(*ast.Ident)
			if !ok || qualifier.Obj != nil || qualified[qualifier.Name] == "" {
				return true
			}
			importPath, name = qualified[qualifier.Name], expression.Sel.Name
			if _, ok := locationSourceFuncs[importPath][name]; !ok {
				return true
			}
			reference, label = expression, importPath+"."+name
		case *ast.Ident:
			if expression.Obj != nil || dotted[expression.Name] == "" {
				return true
			}
			if selector, ok := flow.parents[expression].(*ast.SelectorExpr); ok && selector.Sel == expression {
				return true
			}
			importPath, name = dotted[expression.Name], expression.Name
			reference, label = expression, importPath+"."+name+" (dot import)"
		default:
			return true
		}
		*flow.commands = nil
		offset := positions.Position(reference.Pos()).Offset - shift
		switch {
		case !flow.referenceSafe(reference, locationSourceFuncs[importPath][name]):
			found[label] = append(found[label], sourceUse{Offset: offset})
		case len(*flow.commands) != 0:
			commands := make([]int, len(*flow.commands))
			for i, command := range *flow.commands {
				commands[i] = positions.Position(token.Pos(command)).Offset - shift
			}
			found[label] = append(found[label], sourceUse{Offset: offset, Commands: commands})
		}
		return true
	})
	return found
}

// sourceFlow follows a source's values through one parsed file.
type sourceFlow struct {
	parents      map[ast.Node]ast.Node
	packageLevel map[*ast.Object]bool
	uses         map[*ast.Object][]*ast.Ident
	imports      map[string]string
	visiting     map[*ast.Object]bool
	commands     *[]int // the os/exec selector positions of the commands the current value is the directory of
}

// referenceSafe reports whether reference, a source of kind, is a function
// value that is discarded, or is called and every value the call yields is
// used safely.
func (flow sourceFlow) referenceSafe(reference ast.Expr, kind locationSourceKind) bool {
	if assignment, ok := flow.parents[reference].(*ast.AssignStmt); ok && len(assignment.Lhs) == len(assignment.Rhs) {
		for i, value := range assignment.Rhs {
			if blank, ok := assignment.Lhs[i].(*ast.Ident); ok && value == reference && blank.Name == "_" {
				return true
			}
		}
	}
	call, ok := flow.parents[reference].(*ast.CallExpr)
	if !ok || call.Fun != reference || kind == sourceAlways {
		return false
	}
	switch kind {
	case sourceLookup:
		return flow.resultSafe(call, 0, sourceString)
	case sourceCallerFile:
		return flow.resultSafe(call, 1, sourceString)
	}
	return flow.valueSafe(call, kind)
}

// resultSafe reports whether result index of the multi-valued call is used
// safely as a value of kind: the call stands alone, or is the single right
// side of an assignment or declaration whose operand at index is blank or a
// safely used local variable.
func (flow sourceFlow) resultSafe(call *ast.CallExpr, index int, kind locationSourceKind) bool {
	switch parent := flow.parents[call].(type) {
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		if len(parent.Rhs) != 1 || len(parent.Lhs) <= index || (parent.Tok != token.DEFINE && parent.Tok != token.ASSIGN) {
			return false
		}
		ident, ok := parent.Lhs[index].(*ast.Ident)
		return ok && flow.bindingSafe(ident, kind)
	case *ast.ValueSpec:
		if len(parent.Values) != 1 || len(parent.Names) <= index {
			return false
		}
		return flow.bindingSafe(parent.Names[index], kind)
	}
	return false
}

// bindingSafe reports whether ident, which receives a source value of kind,
// is blank, or is a local variable whose every read is a safe use.
func (flow sourceFlow) bindingSafe(ident *ast.Ident, kind locationSourceKind) bool {
	if ident.Name == "_" {
		return true
	}
	object := ident.Obj
	if object == nil || object.Kind != ast.Var || flow.packageLevel[object] {
		return false
	}
	if flow.visiting[object] {
		return true // its other reads are being checked
	}
	flow.visiting[object] = true
	defer delete(flow.visiting, object)
	for _, use := range flow.uses[object] {
		if use == ident || flow.written(use) {
			continue
		}
		if !flow.valueSafe(use, kind) {
			return false
		}
	}
	return true
}

// written reports whether use is a plain assignment target, which replaces
// the variable's value and reads nothing.
func (flow sourceFlow) written(use *ast.Ident) bool {
	switch parent := flow.parents[use].(type) {
	case *ast.AssignStmt:
		if parent.Tok != token.ASSIGN && parent.Tok != token.DEFINE {
			return false
		}
		for _, target := range parent.Lhs {
			if target == use {
				return true
			}
		}
	case *ast.ValueSpec:
		for _, name := range parent.Names {
			if name == use {
				return true
			}
		}
	}
	return false
}

// valueSafe reports whether expression, a value of kind, is used safely
// where it stands (AFP-V0-3661): discarded, assigned to the blank
// identifier, bound to a local variable whose reads are all safe, or used in
// a way of its kind that cannot form a path.
func (flow sourceFlow) valueSafe(expression ast.Expr, kind locationSourceKind) bool {
	parent := flow.parents[expression]
	switch parent := parent.(type) {
	case *ast.ParenExpr:
		return flow.valueSafe(parent, kind)
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		if len(parent.Lhs) != len(parent.Rhs) || (parent.Tok != token.DEFINE && parent.Tok != token.ASSIGN) {
			return false
		}
		for i, value := range parent.Rhs {
			if value != expression {
				continue
			}
			// A store to anything but a variable is judged by its kind
			// below: only an Env field may receive an environment.
			if ident, ok := parent.Lhs[i].(*ast.Ident); ok {
				return flow.bindingSafe(ident, kind)
			}
		}
	case *ast.ValueSpec:
		if len(parent.Names) != len(parent.Values) {
			return false
		}
		for i, value := range parent.Values {
			if value == expression {
				return flow.bindingSafe(parent.Names[i], kind)
			}
		}
		return false
	}
	switch kind {
	case sourceString:
		return flow.stringSafe(expression, parent)
	case sourceBounded:
		return flow.boundedSafe(expression, parent)
	case sourceEnvironment:
		return flow.environmentSafe(expression, parent)
	case sourceFunc:
		return flow.memberSafe(expression, parent, map[string]bool{"Name": true, "Entry": true}, "FileLine", sourceString)
	case sourceFrames:
		return flow.memberSafe(expression, parent, nil, "Next", sourceFrame)
	case sourceFrame:
		selector, ok := parent.(*ast.SelectorExpr)
		if !ok || selector.X != expression {
			return false
		}
		switch selector.Sel.Name {
		case "Function", "Line", "PC", "Func", "Entry":
			return true
		case "File":
			return flow.valueSafe(selector, sourceString)
		}
	}
	return false
}

// stringSafe lists the uses of a string value that cannot form a path: an
// operand of a comparison, a switch tag, the argument of len or of a number,
// boolean or duration parser, and the value of a NAME=value entry written into
// a string slice for a child process.
func (flow sourceFlow) stringSafe(expression ast.Expr, parent ast.Node) bool {
	switch parent := parent.(type) {
	case *ast.BinaryExpr:
		switch parent.Op {
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			return true
		case token.ADD:
			if parent.Y == expression && environmentEntry(parent.X) && flow.listElement(parent) {
				return true
			}
			if parent.X == expression && plainSuffix(parent.Y) {
				return flow.valueSafe(parent, sourceBounded)
			}
			// Any other concatenation is again a location value.
			return flow.valueSafe(parent, sourceString)
		}
	case *ast.AssignStmt:
		return flow.commandDirectory(expression, parent)
	case *ast.SwitchStmt:
		return parent.Tag == expression
	case *ast.CallExpr:
		if len(parent.Args) == 0 || parent.Args[0] != expression {
			return false
		}
		if ident, ok := parent.Fun.(*ast.Ident); ok {
			return ident.Name == "len" && ident.Obj == nil && len(parent.Args) == 1
		}
		selector, ok := parent.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Obj != nil {
			return false
		}
		importPath := flow.imports[qualifier.Name]
		if parseFuncs[importPath][selector.Sel.Name] {
			return true
		}
		// A pure path transform yields again a location value, judged by its
		// own uses; filepath.Abs is the working directory joined to it.
		if (importPath == "path/filepath" || importPath == "path") && pathTransforms[selector.Sel.Name] {
			if selector.Sel.Name == "Join" && plainJoin(parent.Args[1:]) {
				return flow.valueSafe(parent, sourceBounded)
			}
			return flow.valueSafe(parent, sourceString)
		}
		if importPath == "path/filepath" && selector.Sel.Name == "Abs" && len(parent.Args) == 1 {
			return flow.resultSafe(parent, 0, sourceString)
		}
	}
	return false
}

// boundedSafe lists the uses of a location joined to plain literal
// components that rule (c) bounds: the literal components are path tokens
// of the file, which name every path the join can reach at any depth
// (AFP-V0-2203, AFP-V0-2240), so a read at or below it, a further plain
// join and a flag use are safe. Anything that could widen it (filepath.Dir,
// a return, a store, an argument to another function) is not.
func (flow sourceFlow) boundedSafe(expression ast.Expr, parent ast.Node) bool {
	switch parent := parent.(type) {
	case *ast.BinaryExpr:
		switch parent.Op {
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			return true
		case token.ADD:
			return parent.X == expression && plainSuffix(parent.Y) && flow.valueSafe(parent, sourceBounded)
		}
	case *ast.CallExpr:
		if len(parent.Args) == 0 || parent.Args[0] != expression {
			return false
		}
		selector, ok := parent.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Obj != nil {
			return false
		}
		importPath := flow.imports[qualifier.Name]
		if (importPath == "path/filepath" || importPath == "path") && selector.Sel.Name == "Join" && plainJoin(parent.Args[1:]) {
			return flow.valueSafe(parent, sourceBounded)
		}
		return boundedReads[importPath][selector.Sel.Name]
	}
	return false
}

// boundedReads are the calls that read at or below their first argument and
// return no path above it: import path to function names.
var boundedReads = map[string]map[string]bool{
	"os":            {"ReadFile": true, "Open": true, "OpenFile": true, "Stat": true, "Lstat": true, "ReadDir": true},
	"path/filepath": {"WalkDir": true, "Walk": true},
}

// plainSuffix reports a string literal that spells a separator followed by
// plain components (plainComponents).
func plainSuffix(expression ast.Expr) bool {
	value, ok := locationLiteral(expression)
	rest, separated := strings.CutPrefix(value, "/")
	return ok && separated && plainComponents([]string{rest})
}

// plainJoin reports join arguments that are all string literals of plain
// components (plainComponents).
func plainJoin(arguments []ast.Expr) bool {
	values := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		value, ok := locationLiteral(argument)
		if !ok {
			return false
		}
		values = append(values, value)
	}
	return len(values) != 0 && plainComponents(values)
}

// plainComponents reports `/`-separated components that are none of empty,
// `.` or `..`, hold no backslash or glob character, and of which at least
// one is made only of path token characters, so it is a whole component a
// rule (c) token of the file names.
func plainComponents(values []string) bool {
	exact := false
	for _, value := range values {
		for component := range strings.SplitSeq(value, "/") {
			if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `\*?[`) {
				return false
			}
			whole := true
			for i := range len(component) {
				whole = whole && pathCharacter(component[i])
			}
			exact = exact || whole
		}
	}
	return exact
}

// locationLiteral returns the value of a string literal.
func locationLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

// pathTransforms are the path and path/filepath functions that return one
// string computed from their arguments' text alone, with no file system access.
var pathTransforms = map[string]bool{"Dir": true, "Join": true, "Clean": true, "FromSlash": true, "ToSlash": true}

// commandDirectory reports, and records, a location value stored as the Dir
// of a command: the assignment `name.Dir = value` where name is a local
// variable whose only assignment is its definition by one os/exec Command or
// CommandContext call, and that is never named with Args. Whether that
// command reads only an import graph is the planner's go list proof
// (AFP-V0-2200, AFP-V0-3664), so the use is recorded, not judged.
func (flow sourceFlow) commandDirectory(expression ast.Expr, assignment *ast.AssignStmt) bool {
	if assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || assignment.Rhs[0] != expression {
		return false
	}
	target, ok := assignment.Lhs[0].(*ast.SelectorExpr)
	if !ok || target.Sel.Name != "Dir" {
		return false
	}
	name, ok := target.X.(*ast.Ident)
	if !ok || name.Obj == nil || name.Obj.Kind != ast.Var || flow.packageLevel[name.Obj] {
		return false
	}
	definition, ok := name.Obj.Decl.(*ast.AssignStmt)
	if !ok || definition.Tok != token.DEFINE || len(definition.Lhs) != 1 || len(definition.Rhs) != 1 {
		return false
	}
	call, ok := definition.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext") {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil || flow.imports[qualifier.Name] != "os/exec" {
		return false
	}
	for _, use := range flow.uses[name.Obj] {
		if use == definition.Lhs[0] {
			continue
		}
		if flow.written(use) {
			return false
		}
		if member, ok := flow.parents[use].(*ast.SelectorExpr); ok && member.X == use && member.Sel.Name == "Args" {
			return false
		}
	}
	*flow.commands = append(*flow.commands, int(selector.Pos()))
	return true
}

// environmentEntry reports whether prefix, the left operand of `+ value`,
// ends a NAME=value entry's name: the literal "NAME=" or any expression
// followed by a literal ending in "=" (key + "=").
func environmentEntry(prefix ast.Expr) bool {
	if sum, ok := prefix.(*ast.BinaryExpr); ok && sum.Op == token.ADD {
		literal, ok := sum.Y.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return false
		}
		value, err := strconv.Unquote(literal.Value)
		return err == nil && strings.HasSuffix(value, "=")
	}
	literal, ok := prefix.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && environmentEntryPrefix.MatchString(value)
}

// listElement reports whether expression is an element of a composite
// literal (not a key or a keyed value) or a variadic argument of the builtin
// append, so it is one entry of a string slice.
func (flow sourceFlow) listElement(expression ast.Expr) bool {
	switch parent := flow.parents[expression].(type) {
	case *ast.CompositeLit:
		for _, element := range parent.Elts {
			if element == expression {
				return true
			}
		}
	case *ast.CallExpr:
		if !builtinAppend(parent) {
			return false
		}
		for _, argument := range parent.Args[1:] {
			if argument == expression {
				return true
			}
		}
	}
	return false
}

// environmentSafe allows the whole environment only as a child process's
// environment: the value of an Env key or of an assignment to an Env field,
// directly or through the builtin append, slices.Clip or slices.Clone, whose
// result is again judged as an environment.
func (flow sourceFlow) environmentSafe(expression ast.Expr, parent ast.Node) bool {
	switch parent := parent.(type) {
	case *ast.CallExpr:
		// append(env, entries...), slices.Clip(env) and slices.Clone(env)
		// are again an environment list.
		if len(parent.Args) == 0 || parent.Args[0] != expression {
			return false
		}
		if builtinAppend(parent) {
			return flow.valueSafe(parent, sourceEnvironment)
		}
		selector, ok := parent.Fun.(*ast.SelectorExpr)
		if !ok || len(parent.Args) != 1 || (selector.Sel.Name != "Clip" && selector.Sel.Name != "Clone") {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		return ok && qualifier.Obj == nil && flow.imports[qualifier.Name] == "slices" && flow.valueSafe(parent, sourceEnvironment)
	case *ast.KeyValueExpr:
		key, ok := parent.Key.(*ast.Ident)
		return ok && key.Name == "Env" && parent.Value == expression
	case *ast.AssignStmt:
		if len(parent.Lhs) != 1 || len(parent.Rhs) != 1 || parent.Rhs[0] != expression || parent.Tok != token.ASSIGN {
			return false
		}
		selector, ok := parent.Lhs[0].(*ast.SelectorExpr)
		return ok && selector.Sel.Name == "Env"
	}
	return false
}

// memberSafe allows a frame value only as the receiver of a member in safe,
// or of the method yielding, whose call's result 0 is then a value of kind.
func (flow sourceFlow) memberSafe(expression ast.Expr, parent ast.Node, safe map[string]bool, yielding string, kind locationSourceKind) bool {
	selector, ok := parent.(*ast.SelectorExpr)
	if !ok || selector.X != expression {
		return false
	}
	if safe[selector.Sel.Name] {
		return true
	}
	call, ok := flow.parents[selector].(*ast.CallExpr)
	return ok && call.Fun == selector && selector.Sel.Name == yielding && flow.resultSafe(call, 0, kind)
}

// builtinAppend reports a call of the builtin append with arguments.
func builtinAppend(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "append" && ident.Obj == nil && len(call.Args) > 0
}
