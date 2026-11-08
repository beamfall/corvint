// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

// standaloneToolNames checks descriptor declarations outside the offline registry
// providers. Name expressions have a closed allowlist: string literals and
// file-resolved constants initialized directly with one string literal. Everything
// else is unproven, including cross-file constants, variables and function calls.
// Parser object resolution respects local shadowing; a matching spelling alone
// cannot certify an identifier as a constant.
func standaloneToolNames(dir string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if err != nil {
				return false
			}
			object, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var name ast.Expr
			schema, descriptor := false, false
			switch typ := object.Type.(type) {
			case *ast.Ident:
				descriptor = typ.Name == "ToolDescriptor"
			case *ast.SelectorExpr:
				descriptor = typ.Sel.Name == "ToolDescriptor"
			}
			for _, element := range object.Elts {
				kv, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				var key string
				switch value := kv.Key.(type) {
				case *ast.Ident:
					key = value.Name
				case *ast.BasicLit:
					key, _ = strconv.Unquote(value.Value)
				}
				switch key {
				case "Name", "name":
					name = kv.Value
				case "InputSchema", "inputSchema":
					schema = true
				}
			}
			if !descriptor && !schema {
				return true
			}
			if identifier, ok := name.(*ast.Ident); ok && identifier.Obj != nil && identifier.Obj.Kind == ast.Con {
				if declaration, ok := identifier.Obj.Decl.(*ast.ValueSpec); ok && len(declaration.Names) == 1 && len(declaration.Values) == 1 {
					name = declaration.Values[0]
				}
			}
			if literal, ok := name.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil && value != "" {
					names = append(names, value)
					return true
				}
			}
			err = fmt.Errorf("%s: standalone MCP descriptor name is unproven", positions.Position(object.Pos()))
			return false
		})
		return err
	})
	return names, err
}
