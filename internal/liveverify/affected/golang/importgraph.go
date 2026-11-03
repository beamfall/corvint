package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file finds the import-graph readers of AFP-V0-2200: a package whose
// code or tests run `go list` over packages of the repository observes those
// packages' import graph, not their built behaviour. A literal that is only
// an argument of such a call names no binary the package runs (AFP-V0-037),
// so it adds no command edge; the package gets an import-graph edge instead,
// which a plan follows only when the change alters an import set
// (AFP-V0-2201).

// importGraphFields are the `go list` template fields whose values depend
// only on the import graph of the listed packages: their import paths, the
// package clause, and the dependency closure. A template naming any other
// field (GoFiles, Stale, Export, ...) observes more than the import graph.
var importGraphFields = map[string]bool{
	"ImportPath": true, "Imports": true, "Deps": true, "Name": true, "Standard": true, "DepOnly": true,
}

// templateField matches a field reference of a text/template action.
var templateField = regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_]*)`)

// bareDot matches a dot that starts no field name.
var bareDot = regexp.MustCompile(`\.([^A-Za-z_]|$)`)

// goListFile is what one file's `go list` calls show (AFP-V0-2200): the
// string literals that are only arguments of a proven call, the repository
// directories those calls list, and whether some call lists packages it does
// not name by a literal or observes more than the import graph (all).
type goListFile struct {
	listed  map[token.Pos]bool
	targets map[string]bool
	all     bool
	reader  bool
}

// goListCandidate reports a file whose bytes can hold a `go list` call, so
// only those are parsed in full.
func goListCandidate(body []byte) bool {
	return bytes.Contains(body, []byte(`"list"`)) && bytes.Contains(body, []byte(`"go"`))
}

// goListScanFile parses one file and classifies each `go list` call it makes
// through os/exec (AFP-V0-2200). A call is proven when the command is the
// literal "go", the first argument the literal "list", every flag one of
// -deps, -e or -f with a literal template whose fields are importGraphFields,
// and every other argument a package pattern: a literal, or a concatenation
// whose first operand is a literal or a constant that does not start with
// "-". Its literals are recorded as listed and its literal patterns as
// targets; a pattern that is not a literal, or that matches more than one
// directory, lists the whole module (all). A call that is not proven makes
// the file a reader of the whole module, and its literals stay ordinary uses.
// The arguments are the call's own, or a local []string variable that is
// only initialized by a composite literal, extended by append and passed as
// the call's variadic argument.
func goListScanFile(relative string, body []byte, directory string, owner module) (goListFile, error) {
	result := goListFile{listed: map[token.Pos]bool{}, targets: map[string]bool{}}
	file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.SkipObjectResolution)
	if err != nil {
		return result, err
	}
	names := unshadowedImports(file)
	exec := ""
	for local, value := range names {
		if value == "os/exec" {
			exec = local
		}
	}
	if exec == "" {
		return result, nil
	}
	constants := stringConstants(file)
	var functions []ast.Node
	signatures := map[ast.Node]*ast.FuncType{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Body != nil {
				functions = append(functions, node.Body)
				signatures[node.Body] = node.Type
			}
		case *ast.FuncLit:
			functions = append(functions, node.Body)
			signatures[node.Body] = node.Type
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Name != exec {
			return true
		}
		arguments := call.Args
		switch selector.Sel.Name {
		case "Command":
		case "CommandContext":
			if len(arguments) == 0 {
				return true
			}
			arguments = arguments[1:]
		default:
			return true
		}
		if len(arguments) == 0 || !literalIs(arguments[0], "go") {
			return true
		}
		elements, variadic := arguments[1:], call.Ellipsis.IsValid()
		if variadic {
			if len(elements) != 1 {
				return true
			}
			ident, ok := elements[0].(*ast.Ident)
			if !ok {
				return true
			}
			bodies := enclosingBodies(functions, call)
			proven := false
			for _, body := range bodies {
				if elements, proven = sliceElements(body, ident.Name, call); proven {
					break
				}
			}
			if !proven {
				// Not provably a `go list` call; a function that holds a
				// "list" literal still makes the file a whole-module reader.
				if len(bodies) != 0 && mentionsList(bodies[len(bodies)-1]) {
					result.reader, result.all = true, true
				}
				return true
			}
		}
		if len(elements) == 0 || !literalIs(elements[0], "list") {
			return true
		}
		result.reader = true
		targets, all, proven := goListArguments(elements[1:], constants, directory, owner)
		if bodies := enclosingBodies(functions, call); proven && len(bodies) != 0 {
			outer := bodies[len(bodies)-1]
			tests := map[string]bool{}
			for _, body := range bodies {
				maps.Copy(tests, testingParameters(signatures[body], names))
			}
			proven = isolatedList(outer, exec, call) && !escapes(outer, call, names, tests)
		}
		if !proven {
			result.all = true
			return true
		}
		result.all = result.all || all
		for target := range targets {
			result.targets[target] = true
		}
		for _, element := range elements {
			ast.Inspect(element, func(node ast.Node) bool {
				if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
					result.listed[literal.Pos()] = true
				}
				return true
			})
		}
		return true
	})
	return result, nil
}

// isolatedList reports that body, the outermost function holding a `go list`
// call, cannot change what the call lists or carry its output into another
// command: no assignment to a field named Args (a replaced argument list) and
// none to a field named Env other than a literal environment
// (literalEnvironment), which cannot carry GOFLAGS=-json; and every other os/exec
// call in body passes only string literals after its context, so no listed
// name becomes another command's argument.
func isolatedList(body ast.Node, exec string, list *ast.CallExpr) bool {
	isolated := true
	ast.Inspect(body, func(node ast.Node) bool {
		if !isolated {
			return false
		}
		switch node := node.(type) {
		case *ast.AssignStmt:
			for index, target := range node.Lhs {
				selector, ok := target.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Env" && selector.Sel.Name != "Args" {
					continue
				}
				if selector.Sel.Name == "Args" || len(node.Rhs) != len(node.Lhs) || !literalEnvironment(node.Rhs[index]) {
					isolated = false
				}
			}
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok && (key.Name == "Env" || key.Name == "Args") {
				isolated = false
			}
		case *ast.CallExpr:
			if node == list {
				return true
			}
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok || qualifier.Name != exec || selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext" {
				return true
			}
			arguments := node.Args
			if selector.Sel.Name == "CommandContext" && len(arguments) != 0 {
				arguments = arguments[1:]
			}
			if node.Ellipsis.IsValid() {
				isolated = false
			}
			for _, argument := range arguments {
				if _, literal := literalString(argument); !literal {
					isolated = false
				}
			}
		}
		return true
	})
	return isolated
}

// escapes reports that the `go list` call list, the command it builds, its
// output or a value derived from them can leave body, the outermost function
// holding the call, where a caller or helper could build or run the listed
// packages: returned, sent on a channel, stored anywhere but a local of body,
// or passed to a call this scan does not prove harmless. A harmless call is a
// builtin or conversion, a function of a standard-library import, or a method
// of a derived value or of a *testing.T, B or F parameter of the function;
// a value such a call reaches through an address argument becomes derived. A
// package helper, a function literal, a non-standard import or any other
// method escapes. Names are matched without object resolution, so a shadowed
// name can only add derived values.
func escapes(body ast.Node, list *ast.CallExpr, imports map[string]string, testing map[string]bool) bool {
	local := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, target := range node.Lhs {
					if ident, ok := target.(*ast.Ident); ok {
						local[ident.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, name := range node.Names {
				local[name.Name] = true
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				for _, target := range []ast.Expr{node.Key, node.Value} {
					if ident, ok := target.(*ast.Ident); ok {
						local[ident.Name] = true
					}
				}
			}
		case *ast.FuncLit:
			for _, field := range node.Type.Params.List {
				for _, name := range field.Names {
					local[name.Name] = true
				}
			}
		}
		return true
	})
	derived := map[string]bool{}
	holds := func(expression ast.Node) bool {
		found := false
		ast.Inspect(expression, func(node ast.Node) bool {
			if node == list {
				found = true
			} else if ident, ok := node.(*ast.Ident); ok && derived[ident.Name] {
				found = true
			}
			return !found
		})
		return found
	}
	escaped := false
	// derive marks the local that roots target as derived and reports a new
	// mark; a target rooted anywhere else escapes.
	derive := func(target ast.Expr) bool {
		root := target
		for {
			if selector, ok := root.(*ast.SelectorExpr); ok {
				root = selector.X
			} else if index, ok := root.(*ast.IndexExpr); ok {
				root = index.X
			} else if star, ok := root.(*ast.StarExpr); ok {
				root = star.X
			} else if paren, ok := root.(*ast.ParenExpr); ok {
				root = paren.X
			} else {
				break
			}
		}
		ident, ok := root.(*ast.Ident)
		if !ok || ident.Name != "_" && !local[ident.Name] {
			escaped = true
			return false
		}
		if ident.Name == "_" || derived[ident.Name] {
			return false
		}
		derived[ident.Name] = true
		return true
	}
	harmless := func(call *ast.CallExpr) bool {
		switch function := call.Fun.(type) {
		case *ast.Ident:
			switch function.Name {
			case "string", "len", "cap", "append", "delete", "min", "max":
				return true
			}
		case *ast.ArrayType, *ast.MapType:
			return true
		case *ast.SelectorExpr:
			if qualifier, ok := function.X.(*ast.Ident); ok {
				if path, imported := imports[qualifier.Name]; imported {
					first, _, _ := strings.Cut(path, "/")
					return !strings.Contains(first, ".")
				}
				return derived[qualifier.Name] || testing[qualifier.Name]
			}
			return holds(function.X)
		}
		return false
	}
	for changed := true; changed && !escaped; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			if escaped {
				return false
			}
			switch node := node.(type) {
			case *ast.AssignStmt:
				for index, target := range node.Lhs {
					value := node.Rhs[0]
					if len(node.Rhs) == len(node.Lhs) {
						value = node.Rhs[index]
					}
					if holds(value) {
						changed = derive(target) || changed
					}
				}
			case *ast.ValueSpec:
				for _, value := range node.Values {
					if holds(value) {
						for _, name := range node.Names {
							changed = derive(name) || changed
						}
					}
				}
			case *ast.RangeStmt:
				if holds(node.X) {
					for _, target := range []ast.Expr{node.Key, node.Value} {
						if target != nil {
							changed = derive(target) || changed
						}
					}
				}
			case *ast.ReturnStmt:
				for _, result := range node.Results {
					escaped = escaped || holds(result)
				}
			case *ast.SendStmt:
				escaped = escaped || holds(node.Value)
			case *ast.CallExpr:
				if node == list {
					return true
				}
				reached := false
				for _, argument := range node.Args {
					reached = reached || holds(argument)
				}
				if !reached {
					return true
				}
				if !harmless(node) {
					escaped = true
					return false
				}
				for _, argument := range node.Args {
					if address, ok := argument.(*ast.UnaryExpr); ok && address.Op == token.AND {
						changed = derive(address.X) || changed
					}
				}
			}
			return !escaped
		})
	}
	return escaped
}

// testingParameters names the parameters of signature whose type is
// *testing.T, *testing.B, *testing.F or testing.TB.
func testingParameters(signature *ast.FuncType, imports map[string]string) map[string]bool {
	names := map[string]bool{}
	if signature == nil || signature.Params == nil {
		return names
	}
	for _, field := range signature.Params.List {
		kind := field.Type
		if star, ok := kind.(*ast.StarExpr); ok {
			kind = star.X
		}
		selector, ok := kind.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || imports[qualifier.Name] != "testing" {
			continue
		}
		switch selector.Sel.Name {
		case "T", "B", "F", "TB":
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	return names
}

// literalEnvironment reports an environment `append(os.Environ(), ...)` or
// `[]string{...}` whose added entries are string literals, none setting
// GOFLAGS or GOENV, which change what `go list` prints.
func literalEnvironment(expression ast.Expr) bool {
	var entries []ast.Expr
	switch node := expression.(type) {
	case *ast.CallExpr:
		function, ok := node.Fun.(*ast.Ident)
		if !ok || function.Name != "append" || node.Ellipsis.IsValid() || len(node.Args) == 0 {
			return false
		}
		environ, ok := node.Args[0].(*ast.CallExpr)
		if !ok || len(environ.Args) != 0 {
			return false
		}
		selector, ok := environ.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Environ" {
			return false
		}
		if qualifier, ok := selector.X.(*ast.Ident); !ok || qualifier.Name != "os" {
			return false
		}
		entries = node.Args[1:]
	case *ast.CompositeLit:
		if _, ok := stringSliceLiteral(node); !ok {
			return false
		}
		entries = node.Elts
	default:
		return false
	}
	for _, entry := range entries {
		value, ok := literalString(entry)
		if !ok || strings.HasPrefix(value, "GOFLAGS=") || strings.HasPrefix(value, "GOENV=") {
			return false
		}
	}
	return true
}

// goListArguments classifies the arguments after "list": the directories the
// literal patterns name, whether some pattern names the whole module or is
// not a literal, and whether every argument is a known flag or a pattern.
func goListArguments(elements []ast.Expr, constants map[string]string, directory string, owner module) (map[string]bool, bool, bool) {
	targets := map[string]bool{}
	all := false
	for index := 0; index < len(elements); index++ {
		value, literal := literalString(elements[index])
		if !literal {
			lead, ok := leadingString(elements[index], constants)
			if !ok || strings.HasPrefix(lead, "-") {
				return nil, false, false
			}
			all = true
			continue
		}
		switch {
		case value == "-deps" || value == "-e":
		case value == "-f":
			if index+1 >= len(elements) {
				return nil, false, false
			}
			index++
			template, ok := literalString(elements[index])
			if !ok || !importGraphTemplate(template) {
				return nil, false, false
			}
		case strings.HasPrefix(value, "-f="):
			if !importGraphTemplate(strings.TrimPrefix(value, "-f=")) {
				return nil, false, false
			}
		case strings.HasPrefix(value, "-"):
			return nil, false, false
		default:
			target, whole := listPattern(value, directory, owner)
			if whole {
				all = true
			} else if target != "" {
				targets[target] = true
			}
		}
	}
	return targets, all, true
}

// importGraphTemplate reports a `go list -f` template whose every field
// reference is an importGraphFields member. A bare dot (`{{.}}`, `{{printf
// "%v" .}}`) or a variable (`$`) can print the whole package, so either
// refuses.
func importGraphTemplate(template string) bool {
	if strings.Contains(template, "$") || bareDot.MatchString(template) {
		return false
	}
	for _, match := range templateField.FindAllStringSubmatch(template, -1) {
		if !importGraphFields[match[1]] {
			return false
		}
	}
	return true
}

// listPattern resolves one literal package pattern to the repository-relative
// directory it names, as commandDirectory resolves a command literal. A
// pattern with "..." or the "all" pattern lists the whole module; "std" and
// "cmd" list no repository package. A directory the caller cannot match to
// an observed package is resolved by the caller to the whole module.
func listPattern(value, directory string, owner module) (string, bool) {
	switch {
	case value == "std" || value == "cmd":
		return "", false
	case value == "all" || strings.Contains(value, "..."):
		return "", true
	}
	name := rootAnchored(value, owner.path)
	if strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, ".")
	}
	if name == "." {
		return directory, false
	}
	target, ok := commandDirectory(name, directory, owner.dir)
	if !ok {
		return "", true
	}
	return path.Clean(target), false
}

// literalIs reports a string literal whose value is want.
func literalIs(expression ast.Expr, want string) bool {
	value, ok := literalString(expression)
	return ok && value == want
}

func literalString(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

// leadingString returns the first operand of a + concatenation when it is a
// string literal or a string constant of the file: what the argument starts
// with, so a pattern cannot be a flag in disguise.
func leadingString(expression ast.Expr, constants map[string]string) (string, bool) {
	for {
		switch node := expression.(type) {
		case *ast.BinaryExpr:
			if node.Op != token.ADD {
				return "", false
			}
			expression = node.X
			continue
		case *ast.ParenExpr:
			expression = node.X
			continue
		case *ast.Ident:
			value, ok := constants[node.Name]
			return value, ok && value != ""
		}
		value, ok := literalString(expression)
		return value, ok && value != ""
	}
}

// stringConstants maps each name the file declares exactly once, as a
// constant whose value is a single string literal, to that value. A name
// declared again anywhere in the file (a variable, a field, a parameter,
// another constant) is left out, so no scope can rebind it.
func stringConstants(file *ast.File) map[string]string {
	constants := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		general, ok := node.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			return true
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != len(value.Values) {
				continue
			}
			for index, name := range value.Names {
				if text, literal := literalString(value.Values[index]); literal {
					constants[name.Name] = text
				}
			}
		}
		return true
	})
	counts := map[string]int{}
	for _, name := range declaredNames(file) {
		counts[name]++
	}
	for name := range constants {
		if counts[name] != 1 {
			delete(constants, name)
		}
	}
	return constants
}

// enclosingBodies returns the function bodies in functions that hold node,
// innermost first.
func enclosingBodies(functions []ast.Node, node ast.Node) []ast.Node {
	var bodies []ast.Node
	for _, body := range functions {
		if body.Pos() <= node.Pos() && node.End() <= body.End() {
			bodies = append(bodies, body)
		}
	}
	slices.SortFunc(bodies, func(left, right ast.Node) int {
		return int(right.Pos()) - int(left.Pos())
	})
	return bodies
}

// sliceElements returns the elements a local []string variable named name
// holds when body passes it to call: the composite literal that declares it,
// then every argument of each `name = append(name, ...)`. Any other use of
// the name in body (another assignment, an index, an address, a second
// declaration, a call argument other than call's) proves nothing.
func sliceElements(body ast.Node, name string, call *ast.CallExpr) ([]ast.Expr, bool) {
	if body == nil {
		return nil, false
	}
	var elements []ast.Expr
	declared, proven := false, true
	accounted := map[*ast.Ident]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		if !proven {
			return false
		}
		switch node := node.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			target, ok := node.Lhs[0].(*ast.Ident)
			if !ok || target.Name != name {
				return true
			}
			accounted[target] = true
			switch {
			case node.Tok == token.DEFINE && !declared:
				composite, ok := stringSliceLiteral(node.Rhs[0])
				if !ok {
					proven = false
					return false
				}
				declared = true
				elements = append(elements, composite.Elts...)
			case node.Tok == token.ASSIGN && declared:
				extra, ok := appendTo(node.Rhs[0], name, accounted)
				if !ok {
					proven = false
					return false
				}
				elements = append(elements, extra...)
			default:
				proven = false
				return false
			}
		case *ast.ValueSpec:
			for index, ident := range node.Names {
				if ident.Name != name {
					continue
				}
				accounted[ident] = true
				if declared || len(node.Values) != len(node.Names) {
					proven = false
					return false
				}
				composite, ok := stringSliceLiteral(node.Values[index])
				if !ok {
					proven = false
					return false
				}
				declared = true
				elements = append(elements, composite.Elts...)
			}
		case *ast.CallExpr:
			if node == call {
				if ident, ok := node.Args[len(node.Args)-1].(*ast.Ident); ok {
					accounted[ident] = true
				}
			}
		}
		return true
	})
	if !proven || !declared {
		return nil, false
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && ident.Name == name && !accounted[ident] {
			proven = false
		}
		return proven
	})
	return elements, proven
}

// stringSliceLiteral returns a `[]string{...}` composite literal with no
// keyed elements.
func stringSliceLiteral(expression ast.Expr) (*ast.CompositeLit, bool) {
	composite, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	array, ok := composite.Type.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return nil, false
	}
	if element, ok := array.Elt.(*ast.Ident); !ok || element.Name != "string" {
		return nil, false
	}
	for _, element := range composite.Elts {
		if _, keyed := element.(*ast.KeyValueExpr); keyed {
			return nil, false
		}
	}
	return composite, true
}

// appendTo returns the appended arguments of `append(name, ...)`, marking the
// first argument accounted for; a spread argument proves nothing.
func appendTo(expression ast.Expr, name string, accounted map[*ast.Ident]bool) ([]ast.Expr, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok || call.Ellipsis.IsValid() || len(call.Args) < 2 {
		return nil, false
	}
	function, ok := call.Fun.(*ast.Ident)
	if !ok || function.Name != "append" {
		return nil, false
	}
	first, ok := call.Args[0].(*ast.Ident)
	if !ok || first.Name != name {
		return nil, false
	}
	accounted[first] = true
	return call.Args[1:], true
}

// mentionsList reports a "list" literal within body.
func mentionsList(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && literal.Value == `"list"` {
			found = true
		}
		return true
	})
	return found
}

// goListUnits sets each unit's import-graph edges (AFP-V0-2200) from its
// files' `go list` calls, and returns, by unit index and file, the scans
// whose listed literals literalExecs then does not count as command uses. A
// target directory that holds no observed package makes the unit a reader of
// the whole module.
func goListUnits(root *affected.Source, units []affected.Unit, directories []string, owners map[string]module) []map[string]goListFile {
	byDirectory := make(map[string]string, len(units))
	for index, directory := range directories {
		byDirectory[directory] = units[index].ID
	}
	scans := make([]map[string]goListFile, len(units))
	for index := range units {
		owner := owners[directories[index]]
		reads := map[string]bool{}
		all, reader, environment := false, false, false
		files := append(append([]string(nil), units[index].Sources...), units[index].Tests...)
		bodies := make(map[string][]byte, len(files))
		for _, relative := range files {
			if body, err := root.Read(relative); err == nil {
				bodies[relative] = body
				// A package that names the go command's environment can
				// set GOFLAGS (or a GOENV file holding it) for every call.
				environment = environment || bytes.Contains(body, []byte("GOFLAGS")) || bytes.Contains(body, []byte("GOENV"))
			}
		}
		for _, relative := range files {
			body, read := bodies[relative]
			if !read || !goListCandidate(body) {
				continue
			}
			scan, err := goListScanFile(relative, body, directories[index], owner)
			if err != nil {
				continue
			}
			if !scan.reader {
				continue
			}
			if environment {
				scan = goListFile{reader: true, all: true}
			}
			if scans[index] == nil {
				scans[index] = map[string]goListFile{}
			}
			scans[index][relative] = scan
			reader = true
			all = all || scan.all
			for target := range scan.targets {
				id, observed := byDirectory[target]
				if !observed {
					all = true
					continue
				}
				reads[id] = true
			}
		}
		if !reader {
			continue
		}
		units[index].ImportGraphReadsAll = all
		if !all {
			units[index].ImportGraphReads = sortedKeys(reads)
		}
	}
	return scans
}
