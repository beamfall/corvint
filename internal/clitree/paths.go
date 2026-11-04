// SPDX-License-Identifier: Apache-2.0
package clitree

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

// dispatchPaths keeps branch context for nested fixed-position dispatch. A
// flat vocabulary alone would miss a new route made of already-known words.
func dispatchPaths(fset *token.FileSet, files []*ast.File, found map[string]string) {
	type binding struct {
		name  string
		index int
	}
	argument := func(e ast.Expr) (binding, bool) {
		v, ok := e.(*ast.IndexExpr)
		if !ok {
			return binding{}, false
		}
		id, ok := v.X.(*ast.Ident)
		if !ok || !argumentSlice(id.Name) {
			return binding{}, false
		}
		n, ok := v.Index.(*ast.BasicLit)
		if !ok || n.Kind != token.INT {
			return binding{}, false
		}
		index, err := strconv.Atoi(n.Value)
		return binding{id.Name, index}, err == nil && index >= 0 && index < 16
	}
	var walk func(ast.Node, map[binding]string)
	walk = func(n ast.Node, known map[binding]string) {
		if n == nil {
			return
		}
		extend := func(b binding, word string) map[binding]string {
			next := map[binding]string{}
			for k, v := range known {
				next[k] = v
			}
			next[b] = word
			var route []string
			for i := 0; i <= b.index; i++ {
				v, ok := next[binding{b.name, i}]
				if !ok || !OperationWord(v) {
					return next
				}
				route = append(route, v)
			}
			if len(route) > 1 {
				key := strings.Join(route, " ")
				if _, ok := found[key]; !ok {
					p := fset.Position(n.Pos())
					found[key] = filepath.Base(p.Filename) + ":" + strconv.Itoa(p.Line)
				}
			}
			return next
		}
		switch node := n.(type) {
		case *ast.SwitchStmt:
			if b, ok := argument(node.Tag); ok {
				for _, stmt := range node.Body.List {
					c := stmt.(*ast.CaseClause)
					for _, expr := range c.List {
						if word, ok := stringLiteral(expr); ok && OperationWord(word) {
							next := extend(b, word)
							for _, s := range c.Body {
								walk(s, next)
							}
						}
					}
				}
				return
			}
		case *ast.IfStmt:
			if test, ok := node.Cond.(*ast.BinaryExpr); ok && test.Op == token.EQL {
				if b, ok := argument(test.X); ok {
					if word, ok := stringLiteral(test.Y); ok && OperationWord(word) {
						walk(node.Body, extend(b, word))
						walk(node.Else, known)
						return
					}
				}
			}
		}
		// Visit only direct AST children, keeping this branch's constraints.
		first := true
		ast.Inspect(n, func(child ast.Node) bool {
			if first {
				first = false
				return true
			}
			if child != nil {
				walk(child, known)
			}
			return false
		})
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				walk(fn.Body, map[binding]string{})
			}
		}
	}
}
