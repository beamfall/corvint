// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
)

// comparisonOnlyLocators proves the small non-reading case in AZU-V0-9930.
// Offsets refer to body, which may start after the imports. The synthetic
// package clause lets the same proof operate on either scanner's input.
// Parse failures and unresolved or escaping bindings grant no exemptions.
func comparisonOnlyLocators(body []byte, aliases map[string]string) map[int]bool {
	if !bytes.Contains(body, []byte("Getwd")) {
		return nil
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
		return nil
	}
	safe := map[int]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 2 || len(assignment.Rhs) != 1 {
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
		if !ok || value.Name == "_" || value.Obj == nil || value.Obj.Decl != assignment {
			return true
		}
		// The second result must also be a simple local binding or blank.
		if _, ok := assignment.Lhs[1].(*ast.Ident); !ok {
			return true
		}
		bounded := true
		var parent ast.Node
		var visit func(ast.Node) bool
		visit = func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if use, ok := n.(*ast.Ident); ok && use != value && use.Obj == value.Obj {
				comparison, ok := parent.(*ast.BinaryExpr)
				if !ok || (comparison.Op != token.EQL && comparison.Op != token.NEQ) {
					bounded = false
					return false
				}
				other := comparison.X
				if other == use {
					other = comparison.Y
				}
				literal, ok := other.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					bounded = false
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
		}
		return true
	})
	return safe
}
