// SPDX-License-Identifier: Apache-2.0

package mutate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// SelectionMutant is a plugin-produced candidate, not a trial or a kill. The
// core qualification harness samples the complete population and runs FULL.
type SelectionMutant struct {
	Class, Operator  string
	Line, Start, End int
	Source           []byte
}

// SelectionOperators reuses the Go mutation piece's body operators and its
// literal replacement for package declarations. File operators belong to core.
func SelectionOperators() []string {
	return []string{"declaration-replace-literal", operatorDeleteStatement, operatorNegateCondition, operatorReplaceLiteral, operatorSwapBinary}
}

// SelectionPopulation renders a deterministic complete candidate population.
// A cap refuses the ENTIRE population: a truncated prefix cannot be used as a
// uniform sampling population (RSP-V0-033, RSP-V0-035). No tests are executed.
func SelectionPopulation(source []byte, cap int) ([]SelectionMutant, error) {
	return selectionPopulation(source, cap, renderMutant)
}

func selectionPopulation(source []byte, cap int, render func([]byte, int) ([]byte, bool)) ([]SelectionMutant, error) {
	if cap < 1 || cap > 4096 || len(source) > 1<<20 {
		return nil, fmt.Errorf("go-mutation-cap-invalid")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	sites := planMutations(fset, file)
	if len(sites) > cap {
		return nil, fmt.Errorf("go-mutation-population-capped")
	}
	// Bound rendered storage/work before invoking the existing renderer.
	if (len(sites)+1)*len(source) > 32<<20 {
		return nil, fmt.Errorf("go-mutation-bytes-capped")
	}
	out := make([]SelectionMutant, 0, len(sites))
	var failures []string
	for index, m := range sites {
		rendered, parses := render(source, index)
		if !parses {
			failures = append(failures, fmt.Sprintf("go-mutation-unparseable:%s:%d:%d:%d", m.Operator, m.Line, m.Start, m.End))
			continue
		}
		out = append(out, SelectionMutant{Class: "body-edit", Operator: m.Operator, Line: m.Line, Start: m.Start, End: m.End, Source: rendered})
	}
	if len(failures) != 0 {
		// Never offer a smaller population after silently dropping candidates.
		return nil, fmt.Errorf("%s", strings.Join(failures, ";"))
	}
	capped := false
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		ast.Inspect(gen, func(node ast.Node) bool {
			if capped {
				return false
			}
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			literal, ok := node.(*ast.BasicLit)
			if !ok {
				return true
			}
			replacement, ok := literalReplacement(literal)
			if !ok {
				return true
			}
			if len(out) >= cap || (len(out)+2)*len(source) > 32<<20 {
				capped = true
				return false
			}
			start, end := fset.PositionFor(literal.Pos(), false), fset.PositionFor(literal.End(), false)
			mutant := append([]byte{}, source[:start.Offset]...)
			mutant = append(mutant, replacement...)
			mutant = append(mutant, source[end.Offset:]...)
			out = append(out, SelectionMutant{Class: "declaration-edit", Operator: "declaration-replace-literal", Line: start.Line, Start: start.Offset, End: end.Offset, Source: mutant})
			return true
		})
		if capped || len(out) > cap || (len(out)+1)*len(source) > 32<<20 {
			return nil, fmt.Errorf("go-mutation-population-capped")
		}
	}
	return out, nil
}
