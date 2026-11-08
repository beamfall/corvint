// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/Beamfall/corvint/internal/testevidence"
)

// GoBodies exposes the Go parser's executable function bodies to the RSP
// adapter. Like Analyze/enclosingFunc, declarations and braces are excluded.
// Nested function literals have an explicit parent; package initializers are
// not function bodies. Positions are physical byte positions, ignoring //line.
func GoBodies(source []byte) ([]testevidence.Body, error) {
	if len(source) > 8<<20 {
		return nil, fmt.Errorf("go-regions-source-cap")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	bodies := []testevidence.Body{}
	var walk func(ast.Node, string)
	walk = func(node ast.Node, parent string) {
		ast.Inspect(node, func(n ast.Node) bool {
			var block *ast.BlockStmt
			switch v := n.(type) {
			case *ast.FuncDecl:
				block = v.Body
			case *ast.FuncLit:
				block = v.Body
			default:
				return true
			}
			if block == nil {
				return false
			}
			start, end := fset.PositionFor(block.Lbrace+1, false), fset.PositionFor(block.Rbrace, false)
			if start.Offset >= end.Offset {
				return false
			}
			id := fmt.Sprintf("body:%d", start.Offset)
			bodies = append(bodies, testevidence.Body{ID: id, Parent: parent, Range: testevidence.Range{Start: int32(start.Line), StartColumn: int32(start.Column), End: int32(end.Line), EndColumn: int32(end.Column)}})
			walk(block, id)
			return false
		})
	}
	walk(file, "")
	return bodies, nil
}
