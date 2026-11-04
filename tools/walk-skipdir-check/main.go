// Command walk-skipdir-check refuses a filepath.WalkDir, fs.WalkDir or filepath.Walk callback
// in non-test Go code that returns SkipDir where its entry may not be a directory
// (docs/specs/lane-shell-portability-v0.md, ASD-V0-2476, V1-4511). For a file, SkipDir skips
// every remaining entry of the parent directory; in a linked worktree .git is a file, so a walk
// that skips ".git" that way stops checking the repository root. Run from the repository root,
// it parses every tracked non-test *.go file outside testdata. A callback is a function literal
// passed to one of those walks, directly or through a variable assigned in the same file. A
// `return fs.SkipDir` or `return filepath.SkipDir` in it is guarded when an enclosing if or
// tagless switch case condition has the entry's IsDir() call as an &&-conjunct, or the callback's
// error parameter compared non-nil (WalkDir reports an error only for the root or a directory),
// or when an earlier `if !ENTRY.IsDir() { ... return ... }` in an enclosing block has returned.
// Each unguarded return prints one FAIL line; it exits 1 on a finding, 2 when it cannot list or
// read the files, and 0 otherwise. It reads files only. SkipDir returned through another
// function is not followed.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func main() {
	os.Exit(run(".", os.Stdout, os.Stderr))
}

func run(root string, stdout, stderr io.Writer) int {
	list := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go")
	raw, err := list.Output()
	if err != nil {
		fmt.Fprintf(stderr, "walk-skipdir-check: cannot list Go files: %v\n", err)
		return 2
	}
	failed := false
	for _, path := range strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00") {
		if path == "" || strings.HasSuffix(path, "_test.go") || strings.Contains("/"+path, "/testdata/") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fmt.Fprintf(stderr, "walk-skipdir-check: %v\n", err)
			return 2
		}
		lines, err := check(path, source)
		if err != nil {
			fmt.Fprintf(stderr, "walk-skipdir-check: %v\n", err)
			return 2
		}
		for _, line := range lines {
			failed = true
			fmt.Fprintf(stderr, "walk-skipdir-check: FAIL %s:%d: the walk callback returns SkipDir where its entry may not be a directory, which skips the rest of the parent directory (a .git file in a linked worktree is the usual case); fix: return SkipDir only under ENTRY.IsDir() and nil for a file\n", path, line)
		}
	}
	if failed {
		return 1
	}
	fmt.Fprintln(stdout, "walk-skipdir-check: ok")
	return 0
}

// check returns the line of each unguarded SkipDir return in a walk callback of one file.
func check(name string, source []byte) ([]int, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, name, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	direct := map[*ast.FuncLit]bool{}
	named := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isWalk(call.Fun) || len(call.Args) == 0 {
			return true
		}
		switch callback := call.Args[len(call.Args)-1].(type) {
		case *ast.FuncLit:
			direct[callback] = true
		case *ast.Ident:
			named[callback.Name] = true
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		switch assign := node.(type) {
		case *ast.AssignStmt:
			for i, target := range assign.Lhs {
				if ident, ok := target.(*ast.Ident); ok && named[ident.Name] && i < len(assign.Rhs) {
					if literal, ok := assign.Rhs[i].(*ast.FuncLit); ok {
						direct[literal] = true
					}
				}
			}
		case *ast.ValueSpec:
			for i, ident := range assign.Names {
				if named[ident.Name] && i < len(assign.Values) {
					if literal, ok := assign.Values[i].(*ast.FuncLit); ok {
						direct[literal] = true
					}
				}
			}
		}
		return true
	})
	var lines []int
	for literal := range direct {
		entry, walkErr := callbackParams(literal)
		c := callback{entry: entry, err: walkErr}
		c.block(literal.Body.List, false)
		for _, position := range c.found {
			lines = append(lines, files.Position(position).Line)
		}
	}
	return sortedUnique(lines), nil
}

// isWalk reports whether fun names filepath.WalkDir, fs.WalkDir or filepath.Walk.
func isWalk(fun ast.Expr) bool {
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch pkg.Name + "." + selector.Sel.Name {
	case "filepath.WalkDir", "fs.WalkDir", "filepath.Walk":
		return true
	}
	return false
}

// callbackParams returns the names of a callback's second (entry) and third (error)
// parameters, empty when absent or blank.
func callbackParams(literal *ast.FuncLit) (string, string) {
	var names []string
	for _, field := range literal.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
		}
		for _, ident := range field.Names {
			names = append(names, ident.Name)
		}
	}
	name := func(i int) string {
		if i < len(names) && names[i] != "_" {
			return names[i]
		}
		return ""
	}
	return name(1), name(2)
}

type callback struct {
	entry, err string
	found      []token.Pos
}

// block visits statements in order; an early `if !ENTRY.IsDir() { ...return }` guards the rest.
func (c *callback) block(statements []ast.Stmt, guarded bool) {
	for _, statement := range statements {
		c.statement(statement, guarded)
		if branch, ok := statement.(*ast.IfStmt); ok && branch.Else == nil && c.negatedIsDir(branch.Cond) && returns(branch.Body) {
			guarded = true
		}
	}
}

func (c *callback) statement(statement ast.Stmt, guarded bool) {
	switch s := statement.(type) {
	case *ast.ReturnStmt:
		if !guarded && len(s.Results) == 1 && isSkipDir(s.Results[0]) {
			c.found = append(c.found, s.Pos())
		}
	case *ast.BlockStmt:
		c.block(s.List, guarded)
	case *ast.IfStmt:
		c.block(s.Body.List, guarded || c.guards(s.Cond))
		if s.Else != nil {
			c.statement(s.Else, guarded || c.negatedIsDir(s.Cond))
		}
	case *ast.SwitchStmt:
		for _, clause := range s.Body.List {
			caseClause := clause.(*ast.CaseClause)
			caseGuarded := guarded || s.Tag == nil && len(caseClause.List) == 1 && c.guards(caseClause.List[0])
			c.block(caseClause.Body, caseGuarded)
		}
	case *ast.TypeSwitchStmt:
		for _, clause := range s.Body.List {
			c.block(clause.(*ast.CaseClause).Body, guarded)
		}
	case *ast.SelectStmt:
		for _, clause := range s.Body.List {
			c.block(clause.(*ast.CommClause).Body, guarded)
		}
	case *ast.ForStmt:
		c.block(s.Body.List, guarded)
	case *ast.RangeStmt:
		c.block(s.Body.List, guarded)
	case *ast.LabeledStmt:
		c.statement(s.Stmt, guarded)
	}
}

// guards reports whether a condition has ENTRY.IsDir() or ERR != nil as an &&-conjunct.
func (c *callback) guards(condition ast.Expr) bool {
	switch e := condition.(type) {
	case *ast.ParenExpr:
		return c.guards(e.X)
	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			return c.guards(e.X) || c.guards(e.Y)
		case token.NEQ:
			return c.err != "" && (isIdent(e.X, c.err) && isIdent(e.Y, "nil") || isIdent(e.Y, c.err) && isIdent(e.X, "nil"))
		}
	case *ast.CallExpr:
		return c.isDir(e)
	}
	return false
}

func (c *callback) negatedIsDir(condition ast.Expr) bool {
	for {
		paren, ok := condition.(*ast.ParenExpr)
		if !ok {
			break
		}
		condition = paren.X
	}
	unary, ok := condition.(*ast.UnaryExpr)
	if !ok || unary.Op != token.NOT {
		return false
	}
	call, ok := unary.X.(*ast.CallExpr)
	return ok && c.isDir(call)
}

func (c *callback) isDir(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && c.entry != "" && len(call.Args) == 0 && selector.Sel.Name == "IsDir" && isIdent(selector.X, c.entry)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func isSkipDir(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "SkipDir" && (isIdent(selector.X, "fs") || isIdent(selector.X, "filepath"))
}

// returns reports whether a block ends in a return statement.
func returns(body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	_, ok := body.List[len(body.List)-1].(*ast.ReturnStmt)
	return ok
}

func sortedUnique(lines []int) []int {
	slices.Sort(lines)
	return slices.Compact(lines)
}
