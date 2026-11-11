// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"slices"
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
	if source == builtinSource {
		return "has " + source + ", which hides a builtin the location-source scan trusts"
	}
	return "calls " + source + ", whose value may name a repository path"
}

// linknameSource and cgoSource are the two ways a file can call a location
// source without naming it: a //go:linkname directive binds a local name to
// any function, and C code reads the environment and the working directory
// itself (AFP-V0-3660). builtinSource is a package-level len or append: the
// scan reads an unresolved len or append as the builtin, and a declaration in
// another file of the package is unresolved in this one, so the file that
// declares it holds the package instead.
const (
	linknameSource = "a //go:linkname directive"
	cgoSource      = `cgo (import "C")`
	builtinSource  = "a package-level declaration of len or append"
)

// trustedBuiltins are the predeclared functions sourceFlow recognises by an
// unresolved name.
var trustedBuiltins = map[string]bool{"len": true, "append": true}

// redeclaresBuiltin reports whether body declares a package-level len or
// append, or may and does not parse. A token scan finds each candidate at
// brace depth 0 that is not plainly a call (`len(`); a candidate is confirmed
// on the parsed declarations.
func redeclaresBuiltin(body []byte) bool {
	if !bytes.Contains(body, []byte("len")) && !bytes.Contains(body, []byte("append")) {
		return false
	}
	var lexer scanner.Scanner
	file := token.NewFileSet().AddFile("", -1, len(body))
	lexer.Init(file, body, nil, 0)
	depth, candidate := 0, false
	previous := token.ILLEGAL
	for !candidate {
		_, kind, text := lexer.Scan()
		if kind == token.EOF {
			break
		}
		switch kind {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
		case token.IDENT:
			if depth == 0 && trustedBuiltins[text] {
				_, next, _ := lexer.Scan()
				switch previous {
				case token.FUNC, token.VAR, token.CONST, token.TYPE, token.COMMA, token.LPAREN, token.SEMICOLON:
					candidate = true
				}
				candidate = candidate || next != token.LPAREN
				previous = next
				continue
			}
		}
		previous = kind
	}
	if !candidate {
		return false
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "", body, parser.SkipObjectResolution)
	if err != nil {
		parsed, err = parser.ParseFile(token.NewFileSet(), "", append([]byte("package locationsources\n"), body...), parser.SkipObjectResolution)
	}
	if err != nil {
		return true
	}
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Recv == nil && trustedBuiltins[declaration.Name.Name] {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if trustedBuiltins[name.Name] {
							return true
						}
					}
				case *ast.TypeSpec:
					if trustedBuiltins[spec.Name.Name] {
						return true
					}
				}
			}
		}
	}
	return false
}

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
	sourceString      locationSourceKind = iota // one string result: a path or a computed location
	sourceEnvValue                              // one raw environment value, as os.Getenv returns it
	sourceLookup                                // (string, bool): result 0 is a raw environment value
	sourceEnvironment                           // []string of NAME=value entries
	sourceCallerFile                            // runtime.Caller: result 1 is a file path
	sourceFunc                                  // *runtime.Func: FileLine result 0 is a file path
	sourceFrames                                // *runtime.Frames: Next result 0 is a frame
	sourceFrame                                 // runtime.Frame: its File is a file path
	sourceBounded                               // a location joined to plain literal components
	sourceAlways                                // a working directory: no use is safe
	sourceAbsolute                              // filepath.Abs: the working directory joined to its argument
)

// locationSourceFuncs maps an import path to its location sources.
// runtime.Callers yields program counters only; a path needs FuncForPC or
// CallersFrames, which are listed. filepath.Abs of a relative path is the
// working directory joined to it.
var locationSourceFuncs = map[string]map[string]locationSourceKind{
	"os":            {"Getenv": sourceEnvValue, "LookupEnv": sourceLookup, "Environ": sourceEnvironment, "ExpandEnv": sourceString},
	"syscall":       {"Getenv": sourceLookup, "Environ": sourceEnvironment, "Getwd": sourceAlways, "Getcwd": sourceAlways},
	"runtime":       {"Caller": sourceCallerFile, "FuncForPC": sourceFunc, "CallersFrames": sourceFrames},
	"path/filepath": {"Abs": sourceAbsolute},
}

// parseFuncs are the calls whose result is a number, a boolean or a duration
// whatever string they parse: import path to function names.
var parseFuncs = map[string]map[string]bool{
	"strconv": {"Atoi": true, "ParseBool": true, "ParseInt": true, "ParseUint": true, "ParseFloat": true},
	"time":    {"ParseDuration": true},
}

// environmentName is the name of an environment variable.
var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
	if redeclaresBuiltin(body) {
		found[builtinSource] = nil
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
	envName      string // the literal variable name of the environment source being followed, or ""
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
	case sourceEnvValue:
		flow.envName = environmentVariable(call)
	case sourceLookup:
		flow.envName = environmentVariable(call)
		return flow.resultSafe(call, 0, sourceEnvValue)
	case sourceCallerFile:
		return flow.resultSafe(call, 1, sourceString)
	case sourceAbsolute:
		// An absolute literal names itself; a plain relative literal is the
		// working directory joined to components rule (c) reads; anything
		// else may be the working directory itself (AFP-V0-3660).
		if len(call.Args) != 1 {
			return false
		}
		value, literal := locationLiteral(call.Args[0])
		switch {
		case literal && strings.HasPrefix(value, "/"):
			return true
		case literal && plainComponents([]string{value}):
			return flow.resultSafe(call, 0, sourceBounded)
		}
		return flow.resultSafe(call, 0, sourceString)
	}
	return flow.valueSafe(call, kind)
}

// environmentVariable returns the literal variable name a Getenv or
// LookupEnv call reads, or "" when the name is computed.
func environmentVariable(call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return ""
	}
	name, ok := locationLiteral(call.Args[0])
	if !ok || !environmentName.MatchString(name) {
		return ""
	}
	return name
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
	case sourceString, sourceEnvValue:
		return flow.stringSafe(expression, parent, kind)
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
// boolean or duration parser, and, for a raw environment value only, the
// value of an entry of its own name in a child process's environment
// (commandEnvironmentEntry).
func (flow sourceFlow) stringSafe(expression ast.Expr, parent ast.Node, kind locationSourceKind) bool {
	switch parent := parent.(type) {
	case *ast.BinaryExpr:
		switch parent.Op {
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			return true
		case token.ADD:
			if kind == sourceEnvValue && parent.Y == expression && flow.envName != "" && entryName(parent.X) == flow.envName && flow.commandEnvironmentEntry(parent) {
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
// return no path: import path to function names. os.Open and os.OpenFile
// return a file whose Name is the path, and filepath.Walk and WalkDir hand
// their callback full paths below it, either of which filepath.Dir can climb
// above the join, so they are not listed.
var boundedReads = map[string]map[string]bool{
	"os": {"ReadFile": true, "Stat": true, "Lstat": true, "ReadDir": true},
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
// of a command: the assignment `name.Dir = value` where name is a proven
// command local (execCommandLocal) built by os/exec Command or
// CommandContext, never named with Args or Path, and whose Dir is never read.
// Whether that command reads only an import graph is the planner's go list
// proof (AFP-V0-2200, AFP-V0-3664), so the use is recorded, not judged.
func (flow sourceFlow) commandDirectory(expression ast.Expr, assignment *ast.AssignStmt) bool {
	if assignment.Tok != token.ASSIGN || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || assignment.Rhs[0] != expression {
		return false
	}
	target, ok := assignment.Lhs[0].(*ast.SelectorExpr)
	if !ok || target.Sel.Name != "Dir" {
		return false
	}
	name, ok := target.X.(*ast.Ident)
	if !ok {
		return false
	}
	built, ok := flow.execCommandLocal(name, false, func(member *ast.SelectorExpr) bool {
		switch member.Sel.Name {
		case "Args", "Path":
			return false
		case "Dir":
			return flow.memberWritten(member)
		}
		return true
	})
	if !ok {
		return false
	}
	*flow.commands = append(*flow.commands, int(built))
	return true
}

// execCommandLocal reports whether name is a proven command local: a local
// variable whose only assignment is its definition `name := exec.Command(...)`
// or `exec.CommandContext(...)` (or, when literals, `exec.Cmd{...}` or
// `&exec.Cmd{...}`), and whose every other use is the operand of a member
// selector member allows. Such a command is never passed, returned, stored,
// copied, reassigned or address-taken, so no code but these selectors sees
// it. It returns the position of the os/exec selector that builds it.
func (flow sourceFlow) execCommandLocal(name *ast.Ident, literals bool, member func(*ast.SelectorExpr) bool) (token.Pos, bool) {
	if name.Obj == nil || name.Obj.Kind != ast.Var || flow.packageLevel[name.Obj] {
		return token.NoPos, false
	}
	definition, ok := name.Obj.Decl.(*ast.AssignStmt)
	if !ok || definition.Tok != token.DEFINE || len(definition.Lhs) != 1 || len(definition.Rhs) != 1 {
		return token.NoPos, false
	}
	built := flow.execConstructor(definition.Rhs[0], literals)
	if built == token.NoPos {
		return token.NoPos, false
	}
	for _, use := range flow.uses[name.Obj] {
		if use == definition.Lhs[0] {
			continue
		}
		selector, ok := flow.parents[use].(*ast.SelectorExpr)
		if !ok || selector.X != use || !member(selector) {
			return token.NoPos, false
		}
	}
	return built, true
}

// execConstructor returns the position of the os/exec selector of a Command
// or CommandContext call, or, when literals, of the type of an exec.Cmd
// composite literal or its address; otherwise token.NoPos.
func (flow sourceFlow) execConstructor(expression ast.Expr, literals bool) token.Pos {
	if address, ok := expression.(*ast.UnaryExpr); ok && address.Op == token.AND && literals {
		expression = address.X
	}
	var selector *ast.SelectorExpr
	switch expression := expression.(type) {
	case *ast.CallExpr:
		selector, _ = expression.Fun.(*ast.SelectorExpr)
		if selector == nil || selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext" {
			return token.NoPos
		}
	case *ast.CompositeLit:
		selector, _ = expression.Type.(*ast.SelectorExpr)
		if !literals || selector == nil || selector.Sel.Name != "Cmd" {
			return token.NoPos
		}
	default:
		return token.NoPos
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil || flow.imports[qualifier.Name] != "os/exec" {
		return token.NoPos
	}
	return selector.Pos()
}

// memberWritten reports whether member is a target of a plain assignment,
// which replaces the field and reads nothing.
func (flow sourceFlow) memberWritten(member *ast.SelectorExpr) bool {
	assignment, ok := flow.parents[member].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.ASSIGN {
		return false
	}
	for _, target := range assignment.Lhs {
		if target == member {
			return true
		}
	}
	return false
}

// environmentMember allows every member of a command but a read of its
// environment: Env only as a plain assignment target, never Environ.
func (flow sourceFlow) environmentMember(member *ast.SelectorExpr) bool {
	switch member.Sel.Name {
	case "Env":
		return flow.memberWritten(member)
	case "Environ":
		return false
	}
	return true
}

// entryName returns NAME for the string literal "NAME=", or "".
func entryName(prefix ast.Expr) string {
	value, ok := locationLiteral(prefix)
	name, entry := strings.CutSuffix(value, "=")
	if !ok || !entry || !environmentName.MatchString(name) {
		return ""
	}
	return name
}

// commandEnvironmentEntry reports whether entry, a `"NAME=" + value` sum, is
// an element of a string slice literal, or a variadic argument of the builtin
// append, whose value is used safely as an environment list
// (environmentSafe): it reaches only the Env of a proven command local.
func (flow sourceFlow) commandEnvironmentEntry(entry ast.Expr) bool {
	switch container := flow.parents[entry].(type) {
	case *ast.CompositeLit:
		list, ok := container.Type.(*ast.ArrayType)
		if !ok || list.Len != nil || !slices.Contains(container.Elts, entry) {
			return false
		}
		return flow.valueSafe(container, sourceEnvironment)
	case *ast.CallExpr:
		if !builtinAppend(container) || !slices.Contains(container.Args[1:], entry) {
			return false
		}
		return flow.valueSafe(container, sourceEnvironment)
	}
	return false
}

// environmentSafe allows an environment list only as a child process's
// environment: the value of the Env key of an exec.Cmd literal or of an
// assignment to the Env field of a proven command local whose environment is
// never read back (execCommandLocal, environmentMember), directly or through
// the builtin append, slices.Clip or slices.Clone, whose result is again
// judged as an environment list; or the argument of len.
func (flow sourceFlow) environmentSafe(expression ast.Expr, parent ast.Node) bool {
	switch parent := parent.(type) {
	case *ast.CallExpr:
		// Its length is a number; append(env, entries...), slices.Clip(env)
		// and slices.Clone(env) are again an environment list.
		if len(parent.Args) == 0 || parent.Args[0] != expression {
			return false
		}
		if ident, ok := parent.Fun.(*ast.Ident); ok && ident.Name == "len" && ident.Obj == nil && len(parent.Args) == 1 {
			return true
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
		if !ok || key.Name != "Env" || parent.Value != expression {
			return false
		}
		literal, ok := flow.parents[parent].(*ast.CompositeLit)
		if !ok {
			return false
		}
		var bound ast.Node = literal
		if address, ok := flow.parents[literal].(*ast.UnaryExpr); ok && address.Op == token.AND {
			bound = address
		}
		definition, ok := flow.parents[bound].(*ast.AssignStmt)
		if !ok || len(definition.Lhs) != 1 || len(definition.Rhs) != 1 || definition.Rhs[0] != bound {
			return false
		}
		name, ok := definition.Lhs[0].(*ast.Ident)
		if !ok {
			return false
		}
		_, proven := flow.execCommandLocal(name, true, flow.environmentMember)
		return proven
	case *ast.AssignStmt:
		if len(parent.Lhs) != 1 || len(parent.Rhs) != 1 || parent.Rhs[0] != expression || parent.Tok != token.ASSIGN {
			return false
		}
		selector, ok := parent.Lhs[0].(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Env" {
			return false
		}
		name, ok := selector.X.(*ast.Ident)
		if !ok {
			return false
		}
		_, proven := flow.execCommandLocal(name, true, flow.environmentMember)
		return proven
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
