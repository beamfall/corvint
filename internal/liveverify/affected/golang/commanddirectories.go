// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/ast"
	"go/token"
	"path"
	"strings"
)

// commandDirectoryProven admits only an unchanged directory ("" or "."), a
// fresh testing TempDir, or the module root derived from this file's
// runtime.Caller(0) filename by filepath.Dir/Clean/Join. Unknown origins fail
// closed. The root expression may use single-definition top-level locals,
// but every use of those locals must belong to this derivation: no mutation,
// address, alias, shadowing or escape. This preserves AFP-V0-2200's root
// targets without trusting a variable's name or an arbitrary Dir assignment.
func commandDirectoryProven(value ast.Expr, outer ast.Node, file *ast.File, names map[string]string, tests map[string]bool, relative, moduleDir string, rootTargetsMatch bool) bool {
	if literalIs(value, "") || literalIs(value, ".") {
		return true
	}
	counts := map[string]int{}
	for _, name := range declaredNames(file) {
		counts[name]++
	}
	member := func(call *ast.CallExpr, pkg, method string) bool {
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != method || call.Ellipsis.IsValid() {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		return ok && names[qualifier.Name] == pkg
	}
	if call, ok := value.(*ast.CallExpr); ok && !call.Ellipsis.IsValid() && len(call.Args) == 0 {
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "TempDir" {
			if receiver, ok := selector.X.(*ast.Ident); ok && tests[receiver.Name] && counts[receiver.Name] == 1 {
				// Every use must be the receiver of a direct method call;
				// assignments, addresses, aliases and unknown uses refuse.
				accounted := map[*ast.Ident]bool{}
				ast.Inspect(outer, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
							if ident, ok := selector.X.(*ast.Ident); ok {
								accounted[ident] = true
							}
						}
					}
					return true
				})
				valid := true
				ast.Inspect(outer, func(node ast.Node) bool {
					if ident, ok := node.(*ast.Ident); ok && ident.Name == receiver.Name && !accounted[ident] {
						valid = false
					}
					return valid
				})
				return valid
			}
		}
	}
	body, ok := outer.(*ast.BlockStmt)
	if !ok {
		return false
	}
	definitions := map[string]*ast.AssignStmt{}
	for _, statement := range body.List {
		if assignment, ok := statement.(*ast.AssignStmt); ok && assignment.Tok == token.DEFINE && len(assignment.Rhs) == 1 {
			for _, target := range assignment.Lhs {
				if ident, ok := target.(*ast.Ident); ok && counts[ident.Name] == 1 {
					definitions[ident.Name] = assignment
				}
			}
		}
	}
	used := map[string]bool{}
	accounted := map[*ast.Ident]bool{}
	var resolve func(ast.Expr) (string, bool, bool)
	resolve = func(expression ast.Expr) (string, bool, bool) {
		if text, ok := literalString(expression); ok {
			return text, false, !strings.ContainsAny(text, `\:`) && !strings.HasPrefix(text, "/")
		}
		if ident, ok := expression.(*ast.Ident); ok {
			definition := definitions[ident.Name]
			if definition == nil || definition.End() >= ident.Pos() || used[ident.Name] {
				return "", false, false
			}
			used[ident.Name], accounted[ident] = true, true
			for _, target := range definition.Lhs {
				if declared, ok := target.(*ast.Ident); ok && declared.Name == ident.Name {
					accounted[declared] = true
				}
			}
			if filename, ok := definition.Lhs[min(1, len(definition.Lhs)-1)].(*ast.Ident); ok && len(definition.Lhs) == 4 && filename.Name == ident.Name {
				if call, ok := definition.Rhs[0].(*ast.CallExpr); ok && member(call, "runtime", "Caller") && len(call.Args) == 1 {
					if skip, ok := call.Args[0].(*ast.BasicLit); ok && skip.Kind == token.INT && skip.Value == "0" {
						return relative, true, true
					}
				}
			}
			if len(definition.Lhs) != 1 {
				return "", false, false
			}
			return resolve(definition.Rhs[0])
		}
		call, ok := expression.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return "", false, false
		}
		if len(call.Args) == 1 && (member(call, "path/filepath", "Dir") || member(call, "path/filepath", "Clean")) {
			text, anchored, ok := resolve(call.Args[0])
			if member(call, "path/filepath", "Dir") {
				return path.Dir(text), anchored, ok
			}
			return path.Clean(text), anchored, ok
		}
		if !member(call, "path/filepath", "Join") {
			return "", false, false
		}
		text, anchored, ok := resolve(call.Args[0])
		if !ok {
			return "", false, false
		}
		for _, argument := range call.Args[1:] {
			part, rooted, ok := resolve(argument)
			if !ok || rooted {
				return "", false, false
			}
			text = path.Join(text, part)
		}
		return text, anchored, true
	}
	directory, anchored, valid := resolve(value)
	if !valid || !anchored || !rootTargetsMatch || path.Clean(directory) != path.Clean(moduleDir) {
		return false
	}
	ast.Inspect(outer, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && used[ident.Name] && !accounted[ident] {
			valid = false
		}
		return valid
	})
	return valid
}
