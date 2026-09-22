package mutate

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
)

// Operator names. The set is closed and deterministic: every mutant records the
// one operator that produced it.
const (
	operatorNegateCondition = "negate-condition"
	operatorSwapBinary      = "swap-binary"
	operatorReplaceLiteral  = "replace-literal"
	operatorDeleteStatement = "delete-statement"
)

// binarySwaps is the closed operator table for binary expressions.
var binarySwaps = map[token.Token]token.Token{
	token.EQL:  token.NEQ,
	token.NEQ:  token.EQL,
	token.LSS:  token.GEQ,
	token.GEQ:  token.LSS,
	token.GTR:  token.LEQ,
	token.LEQ:  token.GTR,
	token.ADD:  token.SUB,
	token.SUB:  token.ADD,
	token.LAND: token.LOR,
	token.LOR:  token.LAND,
}

// site is one candidate mutation bound to one parse of the source. apply edits
// that parse in place; the caller re-parses for every mutant so a site index is
// stable across renderings.
type site struct {
	Operator string
	Line     int
	Start    int
	End      int
	offset   token.Pos
	apply    func()
}

// mutant is one rendered mutation of the changed file.
type mutant struct {
	Operator string
	Line     int
	Start    int
	End      int
	Source   []byte
}

// planMutations lists every candidate mutation in source order. Only top-level
// function bodies are touched: declarations, signatures, and imports are left
// alone so a mutant stays a plausible version of the same program.
func planMutations(fset *token.FileSet, file *ast.File) []site {
	sites := make([]site, 0, 16)
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Body == nil {
			continue
		}
		sites = append(sites, planBody(fset, function.Body)...)
	}
	sort.SliceStable(sites, func(left, right int) bool { return sites[left].offset < sites[right].offset })
	return sites
}

func planBody(fset *token.FileSet, body *ast.BlockStmt) []site {
	sites := make([]site, 0, 8)
	record := func(operator string, start, end token.Pos, apply func()) {
		sites = append(sites, site{
			Operator: operator, Line: fset.Position(start).Line,
			Start: fset.Position(start).Offset, End: fset.Position(end).Offset,
			offset: start, apply: apply,
		})
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt:
			planCondition(record, typed.Cond, func(negated ast.Expr) { typed.Cond = negated })
		case *ast.ForStmt:
			planCondition(record, typed.Cond, func(negated ast.Expr) { typed.Cond = negated })
		case *ast.BinaryExpr:
			planBinary(record, typed)
		case *ast.ReturnStmt:
			planReturn(record, typed)
		case *ast.BlockStmt:
			planStatements(record, typed)
		}
		return true
	})
	return sites
}

type recorder func(operator string, start, end token.Pos, apply func())

func planCondition(record recorder, condition ast.Expr, replace func(ast.Expr)) {
	if condition == nil {
		return
	}
	record(operatorNegateCondition, condition.Pos(), condition.End(), func() {
		replace(&ast.UnaryExpr{Op: token.NOT, X: &ast.ParenExpr{X: condition}})
	})
}

func planBinary(record recorder, expression *ast.BinaryExpr) {
	swapped, mutable := binarySwaps[expression.Op]
	if !mutable {
		return
	}
	record(operatorSwapBinary, expression.OpPos, expression.OpPos+token.Pos(len(expression.Op.String())), func() { expression.Op = swapped })
}

func planReturn(record recorder, statement *ast.ReturnStmt) {
	for _, result := range statement.Results {
		literal, isLiteral := result.(*ast.BasicLit)
		if !isLiteral {
			continue
		}
		replacement, mutable := literalReplacement(literal)
		if !mutable {
			continue
		}
		record(operatorReplaceLiteral, literal.ValuePos, literal.End(), func() { literal.Value = replacement })
	}
}

func literalReplacement(literal *ast.BasicLit) (string, bool) {
	switch literal.Kind {
	case token.INT:
		if literal.Value == "0" {
			return "1", true
		}
		return "0", true
	case token.STRING:
		if literal.Value == `""` {
			return `"mutant"`, true
		}
		return `""`, true
	default:
		return "", false
	}
}

// planStatements deletes expression statements and plain assignments. A `:=`
// definition is never deleted: the deletion almost always leaves an unused or
// undeclared name, which is a build failure, not a behavioural mutant.
func planStatements(record recorder, block *ast.BlockStmt) {
	for index, statement := range block.List {
		if !deletableStatement(statement) {
			continue
		}
		record(operatorDeleteStatement, statement.Pos(), statement.End(), func() {
			block.List = append(block.List[:index:index], block.List[index+1:]...)
		})
	}
}

func deletableStatement(statement ast.Stmt) bool {
	switch typed := statement.(type) {
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		return typed.Tok != token.DEFINE
	}
	return false
}

// generateMutants renders one mutant per planned site, in source order, keeping
// only those that still parse and stopping at maxMutants. When spans are
// given, only sites whose line lies inside one of them are rendered, so a
// claim about a change is judged on the change.
func generateMutants(source []byte, maxMutants int, spans []LineSpan) ([]mutant, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	plan := planMutations(fset, file)
	mutants := make([]mutant, 0, len(plan))
	for index := range plan {
		if !insideSpans(plan[index].Line, spans) {
			continue
		}
		rendered, parses := renderMutant(source, index)
		if !parses {
			continue
		}
		mutants = append(mutants, mutant{
			Operator: plan[index].Operator, Line: plan[index].Line,
			Start: plan[index].Start, End: plan[index].End, Source: rendered,
		})
		if len(mutants) == maxMutants {
			break
		}
	}
	return mutants, nil
}

// insideSpans reports whether line lies in one of the spans; no spans admits
// every line.
func insideSpans(line int, spans []LineSpan) bool {
	if len(spans) == 0 {
		return true
	}
	for _, span := range spans {
		if line >= span.Start && line <= span.End {
			return true
		}
	}
	return false
}

// renderMutant re-parses the original source and applies exactly the site at
// index, so no mutation can leak into another.
func renderMutant(source []byte, index int) ([]byte, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "source.go", source, parser.ParseComments)
	if err != nil {
		return nil, false
	}
	plan := planMutations(fset, file)
	if index >= len(plan) {
		return nil, false
	}
	plan[index].apply()
	printed := &bytes.Buffer{}
	configuration := &printer.Config{Mode: printer.TabIndent, Tabwidth: 8}
	if err := configuration.Fprint(printed, fset, file); err != nil {
		return nil, false
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "source.go", printed.Bytes(), parser.SkipObjectResolution); err != nil {
		return nil, false
	}
	return printed.Bytes(), true
}
