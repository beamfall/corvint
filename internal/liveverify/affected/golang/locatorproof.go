// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// comparisonOnlyLocators proves the small non-reading cases of AZU-V0-9930 and
// AFP-V0-2950: an `os.Getwd` result bound to a local variable whose every
// other use compares it with a string literal or joins it, as the first
// argument of `filepath.Join` or `path.Join`, with literal plain components,
// into a path only a read at or below it consumes (boundedJoinConsumed).
// Each bounded join contributes the root-anchored token of its literal
// components (`/docs/guide.md`), returned sorted and unique in joined, so a
// change at or below the joined path selects the package as a rule (c) reader
// (AFP-V0-2203) and nothing else does. Offsets refer to body, which may start
// after the imports. The synthetic package clause lets the same proof operate
// on either scanner's input. aliases maps the file's local qualifiers to the
// import paths of rootLocatorFuncs; joins maps them to pathJoinFuncs. Parse
// failures and unresolved or escaping bindings grant no exemptions.
func comparisonOnlyLocators(body []byte, aliases, joins map[string]string) (safe map[int]bool, joined []string) {
	if !bytes.Contains(body, []byte("Getwd")) {
		return nil, nil
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "", body, 0)
	shift := 0
	if err != nil {
		const prefix = "package locatorproof\n"
		positions = token.NewFileSet()
		file, err = parser.ParseFile(positions, "", append([]byte(prefix), body...), 0)
		shift = len(prefix)
	}
	if err != nil {
		return nil, nil
	}
	safe = map[int]bool{}
	anchored := map[string]bool{}
	// parents maps every node to its enclosing node, so a bounded join's
	// consumers can be followed (boundedJoinConsumed).
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) != 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	packageLevel := map[*ast.ValueSpec]bool{}
	for _, declaration := range file.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok && general.Tok == token.VAR {
			for _, spec := range general.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					packageLevel[value] = true
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || (assignment.Tok != token.DEFINE && assignment.Tok != token.ASSIGN) || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Getwd" {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Obj != nil || aliases[qualifier.Name] != "os" {
			return true
		}
		value, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok || value.Name == "_" || value.Obj == nil || value.Obj.Kind != ast.Var || !localDeclaration(value.Obj.Decl, packageLevel) {
			return true
		}
		// The second result must also be a simple local binding or blank.
		if _, ok := assignment.Lhs[1].(*ast.Ident); !ok {
			return true
		}
		bounded := true
		var paths []string
		var parent ast.Node
		var visit func(ast.Node) bool
		visit = func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if use, ok := n.(*ast.Ident); ok && use != value && use.Obj == value.Obj {
				switch expression := parent.(type) {
				case *ast.BinaryExpr:
					if expression.Op != token.EQL && expression.Op != token.NEQ {
						bounded = false
						return false
					}
					other := expression.X
					if other == use {
						other = expression.Y
					}
					literal, ok := other.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						bounded = false
					}
				case *ast.CallExpr:
					// AFP-V0-2950: the root is the first argument of a join
					// whose other arguments are literal plain components,
					// and only a read at or below the joined path consumes
					// the result: a `filepath.Dir`, a further join, a return
					// or any other consumer could widen it.
					if joinedPath, ok := boundedJoin(expression, use, joins); ok && boundedJoinConsumed(file, expression, parents, aliases, packageLevel) {
						paths = append(paths, joinedPath)
					} else {
						bounded = false
					}
				case *ast.AssignStmt:
					// Certify every write, including the original declaration.
					// Only Getwd and a local empty-string initializer preserve
					// the root premise; arbitrary initializers may survive a
					// conditional Getwd fallback.
					if expression != assignment && !sameLocatorBinding(expression, assignment, aliases) && !emptyLocatorDeclaration(expression, use) {
						bounded = false
					}
				case *ast.ValueSpec:
					// Only an uninitialized or empty local string is known.
					if !emptyLocatorDeclaration(expression, use) {
						bounded = false
					}
				default:
					bounded = false
				}
				if !bounded {
					return false
				}
			}
			previous := parent
			parent = n
			ast.Inspect(n, func(child ast.Node) bool {
				if child == n {
					return true
				}
				visit(child)
				return false
			})
			parent = previous
			return false
		}
		visit(file)
		if bounded {
			safe[positions.Position(selector.Sel.Pos()).Offset-shift] = true
			for _, joinedPath := range paths {
				anchored[joinedPath] = true
			}
		}
		return true
	})
	for joinedPath := range anchored {
		joined = append(joined, joinedPath)
	}
	sort.Strings(joined)
	return safe, joined
}

// pathJoinFuncs names the calls that join a located root with components.
var pathJoinFuncs = map[string]map[string]bool{
	"path/filepath": {"Join": true},
	"path":          {"Join": true},
}

// boundedReadFuncs names the `os` calls whose one path argument they read at
// or below and whose results carry no path that could climb above it: file
// contents, a FileInfo (its Name is a base name) and DirEntry base names.
// `os.Open` is not one (the File's Name returns the path), nor is a walk,
// whose callback receives the path.
var boundedReadFuncs = map[string]bool{"ReadFile": true, "Stat": true, "Lstat": true, "ReadDir": true}

// boundedJoinConsumed reports a bounded join whose result only a read at or
// below the joined path consumes (AFP-V0-2950): it is the one argument of a
// boundedReadFuncs call (under the local name the file's import gives `os`),
// or the value of a `:=` or `var` binding of one local variable, never rebound
// or addressed, whose every use is such an argument or an operand of `==` or
// `!=` against a string literal. Any other consumer may widen the path
// (`filepath.Dir(p)`, `filepath.Join(p, "..")`, a return, a call, a field), so
// the proof refuses it and the package stays a root locator.
func boundedJoinConsumed(file *ast.File, call *ast.CallExpr, parents map[ast.Node]ast.Node, aliases map[string]string, packageLevel map[*ast.ValueSpec]bool) bool {
	consumer := parents[call]
	if boundedRead(consumer, call, aliases) {
		return true
	}
	var variable *ast.Ident
	switch binding := consumer.(type) {
	case *ast.AssignStmt:
		if binding.Tok != token.DEFINE || len(binding.Lhs) != 1 || len(binding.Rhs) != 1 || binding.Rhs[0] != call {
			return false
		}
		variable, _ = binding.Lhs[0].(*ast.Ident)
	case *ast.ValueSpec:
		if packageLevel[binding] || len(binding.Names) != 1 || len(binding.Values) != 1 || binding.Values[0] != call {
			return false
		}
		variable = binding.Names[0]
	default:
		return false
	}
	if variable == nil || variable.Name == "_" || variable.Obj == nil || variable.Obj.Decl != consumer {
		return false
	}
	consumed := true
	ast.Inspect(file, func(node ast.Node) bool {
		use, ok := node.(*ast.Ident)
		if !consumed || !ok || use == variable || use.Obj != variable.Obj {
			return consumed
		}
		consumed = boundedRead(parents[use], use, aliases) || literalComparison(parents[use], use)
		return consumed
	})
	return consumed
}

// boundedRead reports a call of a boundedReadFuncs function whose only
// argument is argument.
func boundedRead(consumer ast.Node, argument ast.Expr, aliases map[string]string) bool {
	call, ok := consumer.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Args[0] != argument || call.Ellipsis.IsValid() {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && qualifier.Obj == nil && aliases[qualifier.Name] == "os" && boundedReadFuncs[selector.Sel.Name]
}

// literalComparison reports an `==` or `!=` comparison of use with a string
// literal.
func literalComparison(consumer ast.Node, use ast.Expr) bool {
	expression, ok := consumer.(*ast.BinaryExpr)
	if !ok || expression.Op != token.EQL && expression.Op != token.NEQ {
		return false
	}
	other := expression.X
	if other == use {
		other = expression.Y
	}
	literal, ok := other.(*ast.BasicLit)
	return ok && literal.Kind == token.STRING
}

// localDeclaration reports a variable declared by a short variable
// declaration or a `var` statement inside a function, never a package-level
// value (packageLevel) another file could read, a parameter or a result.
func localDeclaration(declaration any, packageLevel map[*ast.ValueSpec]bool) bool {
	switch spec := declaration.(type) {
	case *ast.AssignStmt:
		return true
	case *ast.ValueSpec:
		return !packageLevel[spec]
	}
	return false
}

// emptyLocatorDeclaration recognizes only a single local zero-string
// declaration. Any other initializer or write remains unproven (AFP-V0-2950).
func emptyLocatorDeclaration(node ast.Node, use *ast.Ident) bool {
	if use.Obj == nil || use.Obj.Decl != node {
		return false
	}
	var initial ast.Expr
	switch declaration := node.(type) {
	case *ast.AssignStmt:
		if declaration.Tok != token.DEFINE || len(declaration.Lhs) != 1 || len(declaration.Rhs) != 1 || declaration.Lhs[0] != use {
			return false
		}
		initial = declaration.Rhs[0]
	case *ast.ValueSpec:
		if len(declaration.Names) != 1 || declaration.Names[0] != use {
			return false
		}
		if len(declaration.Values) == 0 {
			typ, ok := declaration.Type.(*ast.Ident)
			return ok && typ.Obj == nil && typ.Name == "string"
		}
		if len(declaration.Values) != 1 {
			return false
		}
		initial = declaration.Values[0]
	default:
		return false
	}
	literal, ok := initial.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && value == ""
}

// sameLocatorBinding reports an assignment that rebinds the proof's variable
// from another `os.Getwd()` call with the same two-result shape (a `root, err
// = os.Getwd()` after a `root := ""`), which the proof inspects on its own.
func sameLocatorBinding(candidate, original *ast.AssignStmt, aliases map[string]string) bool {
	if (candidate.Tok != token.DEFINE && candidate.Tok != token.ASSIGN) || len(candidate.Lhs) != 2 || len(candidate.Rhs) != 1 {
		return false
	}
	call, ok := candidate.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Getwd" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil || aliases[qualifier.Name] != "os" {
		return false
	}
	left, ok := candidate.Lhs[0].(*ast.Ident)
	second, simple := candidate.Lhs[1].(*ast.Ident)
	first, _ := original.Lhs[0].(*ast.Ident)
	return ok && simple && second.Obj != left.Obj && first != nil && left.Obj == first.Obj
}

// boundedJoin reports a call of filepath.Join or path.Join (under the local
// name joins gives the package) with use as its first argument and every
// other argument a string literal whose components are plain names, and
// returns the root-anchored token of the joined literals.
func boundedJoin(call *ast.CallExpr, use *ast.Ident, joins map[string]string) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || call.Ellipsis.IsValid() || len(call.Args) < 2 || call.Args[0] != use {
		return "", false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil || !pathJoinFuncs[joins[qualifier.Name]][selector.Sel.Name] {
		return "", false
	}
	components := make([]string, 0, len(call.Args)-1)
	for _, argument := range call.Args[1:] {
		value, ok := boundedJoinArgument(argument)
		if !ok {
			return "", false
		}
		components = append(components, value)
	}
	return "/" + path.Join(components...), true
}

// boundedJoinArgument returns the value of a string literal whose every path
// component is a plain name: not empty, `.`, `..` or a glob, so the join stays
// below the root at a path the literals spell.
func boundedJoinArgument(argument ast.Expr) (string, bool) {
	literal, ok := argument.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	value = strings.Trim(value, "/")
	if value == "" {
		return "", false
	}
	for component := range strings.SplitSeq(value, "/") {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, "*?[\\") {
			return "", false
		}
	}
	return value, true
}
