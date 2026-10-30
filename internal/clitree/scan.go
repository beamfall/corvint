package clitree

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Literal is a string a binary's dispatch code matches an argument against,
// with the first place it does.
type Literal struct {
	Value string
	At    string
}

// Scan is the code-derived dispatch inventory of a set of source directories
// (CAP-V0-041): every word a subcommand switch, comparison or table lookup
// matches at a fixed argument position, and every option a flag set declares
// or an argument loop matches.
type Scan struct {
	Words []Literal
	Flags []Literal
}

// argumentSlice reports whether a name holds command-line arguments.
func argumentSlice(name string) bool {
	lowered := strings.ToLower(name)
	return strings.Contains(lowered, "arg") || lowered == "rest" || lowered == "tokens" || lowered == "tail" ||
		lowered == "remaining" || lowered == "words" || lowered == "cmd"
}

// childArgv reports whether an identifier names a child-process argument
// vector rather than this binary's command line (CAP-V0-047, CAP-V0-200).
type childArgv func(*ast.Ident) bool

func sliceName(expression ast.Expr, child childArgv) (string, bool) {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name, argumentSlice(value.Name) && !child(value)
	case *ast.SelectorExpr:
		return value.Sel.Name, value.Sel.Name == "Args" || argumentSlice(value.Sel.Name)
	case *ast.SliceExpr:
		return sliceName(value.X, child)
	}
	return "", false
}

// rootIdent returns the variable an argument expression reads: NAME, NAME[i],
// NAME[i:j] or a parenthesized form of them.
func rootIdent(expression ast.Expr) *ast.Ident {
	switch value := expression.(type) {
	case *ast.Ident:
		return value
	case *ast.IndexExpr:
		return rootIdent(value.X)
	case *ast.SliceExpr:
		return rootIdent(value.X)
	case *ast.ParenExpr:
		return rootIdent(value.X)
	}
	return nil
}

// childScope is one function body and the slice names it uses as a
// child-process argv there.
type childScope struct {
	start, end token.Pos
	names      map[string]bool
}

// childScopes finds, per function body, the slices that are a child process's
// argv, not this binary's arguments: a slice passed to exec.Command,
// exec.CommandContext or syscall.Exec, assigned to a Cmd's Args field, or
// bound to a []string composite literal it builds (V1-3153). A function that
// both dispatches on a parameter and execs it keeps it as a child argv, so its
// dispatch words are not scanned; os.Args, a selector's Args field and a name
// the function binds from one of them or from an Args() call (flag.Args(),
// set.Args()) are never excluded, so a binary that re-executes itself with its
// own arguments is still scanned.
func childScopes(file *ast.File) []childScope {
	var scopes []childScope
	ast.Inspect(file, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		}
		if body == nil {
			return true
		}
		names, own := map[string]bool{}, map[string]bool{}
		var mark func(ast.Expr)
		mark = func(expression ast.Expr) {
			// append(prefix, argv...) builds the child argv from its arguments.
			if call, ok := expression.(*ast.CallExpr); ok {
				if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" {
					for _, argument := range call.Args {
						mark(argument)
					}
				}
				return
			}
			if id := rootIdent(expression); id != nil {
				names[id.Name] = true
			}
		}
		bindOwn := func(name string, right ast.Expr) {
			if commandLine(right) {
				own[name] = true
			}
		}
		ast.Inspect(body, func(m ast.Node) bool {
			switch node := m.(type) {
			case *ast.CallExpr:
				selector, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch {
				case pkg.Name == "exec" && (selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext"),
					pkg.Name == "syscall" && selector.Sel.Name == "Exec":
					for _, argument := range node.Args {
						mark(argument)
					}
				}
			case *ast.AssignStmt:
				if len(node.Lhs) != len(node.Rhs) {
					return true
				}
				for index, left := range node.Lhs {
					if selector, ok := left.(*ast.SelectorExpr); ok && selector.Sel.Name == "Args" {
						mark(node.Rhs[index])
					}
					if id, ok := left.(*ast.Ident); ok {
						bindOwn(id.Name, node.Rhs[index])
						if stringSliceLiteral(node.Rhs[index]) {
							names[id.Name] = true
						}
					}
				}
			case *ast.ValueSpec:
				for index, name := range node.Names {
					if index < len(node.Values) {
						bindOwn(name.Name, node.Values[index])
						if stringSliceLiteral(node.Values[index]) {
							names[name.Name] = true
						}
					}
				}
			case *ast.CompositeLit:
				// exec.Cmd{Args: argv}
				for _, element := range node.Elts {
					if pair, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "Args" {
							mark(pair.Value)
						}
					}
				}
			}
			return true
		})
		for name := range own {
			delete(names, name)
		}
		if len(names) > 0 {
			scopes = append(scopes, childScope{body.Pos(), body.End(), names})
		}
		return true
	})
	return scopes
}

// commandLine reports whether an expression reads this binary's own command
// line: os.Args (or a selector's Args field), an Args() call, or a slice or
// element of one.
func commandLine(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name == "Args"
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && selector.Sel.Name == "Args" && len(value.Args) == 0
	case *ast.SliceExpr:
		return commandLine(value.X)
	case *ast.IndexExpr:
		return commandLine(value.X)
	case *ast.ParenExpr:
		return commandLine(value.X)
	}
	return false
}

// stringSliceLiteral reports whether an expression is a []string{...} literal.
func stringSliceLiteral(expression ast.Expr) bool {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return false
	}
	array, ok := literal.Type.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	element, ok := array.Elt.(*ast.Ident)
	return ok && element.Name == "string"
}

// position classifies an expression: fixed when it indexes an argument slice
// at a constant, any when it is an argument element at a computed index.
func position(expression ast.Expr, fixed, element map[string]bool, child childArgv) (isFixed, isElement bool) {
	switch value := expression.(type) {
	case *ast.IndexExpr:
		if _, ok := sliceName(value.X, child); !ok {
			return false, false
		}
		_, constant := value.Index.(*ast.BasicLit)
		return constant, true
	case *ast.Ident:
		return fixed[value.Name], fixed[value.Name] || element[value.Name]
	case *ast.CallExpr:
		// strings.TrimSpace(args[0]), strings.ToLower(sub) and the like.
		if selector, ok := value.Fun.(*ast.SelectorExpr); ok && len(value.Args) == 1 {
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "strings" {
				return position(value.Args[0], fixed, element, child)
			}
		}
	}
	return false, false
}

func stringLiteral(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

var flagSetMethods = map[string]int{
	"String": 0, "Bool": 0, "Int": 0, "Int64": 0, "Uint": 0, "Uint64": 0, "Float64": 0, "Duration": 0,
	"Func": 0, "BoolFunc": 0,
	"StringVar": 1, "BoolVar": 1, "IntVar": 1, "Int64Var": 1, "UintVar": 1, "Uint64Var": 1,
	"Float64Var": 1, "DurationVar": 1, "Var": 1, "TextVar": 1,
}

// ScanDirs reads the non-test Go files of each directory (not recursively)
// and returns their dispatch literals. It reads only those files.
func ScanDirs(dirs ...string) (Scan, error) {
	words, flags := map[string]string{}, map[string]string{}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return Scan{}, err
		}
		var files []*ast.File
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return Scan{}, err
			}
			files = append(files, file)
		}
		scanPackage(fset, files, words, flags)
	}
	return Scan{Words: literals(words), Flags: literals(flags)}, nil
}

func literals(found map[string]string) []Literal {
	out := make([]Literal, 0, len(found))
	for value, at := range found {
		out = append(out, Literal{Value: value, At: at})
	}
	slices.SortFunc(out, func(a, b Literal) int { return strings.Compare(a.Value, b.Value) })
	return out
}

func scanPackage(fset *token.FileSet, files []*ast.File, words, flags map[string]string) {
	// Package-level and local composite literals by name, for table lookups.
	tables := map[string][]ast.Expr{}
	looked := map[string]bool{}
	registered := map[string][]string{}
	record := func(found map[string]string, value string, at token.Pos) {
		if _, seen := found[value]; !seen {
			position := fset.Position(at)
			found[value] = filepath.Base(position.Filename) + ":" + strconv.Itoa(position.Line)
		}
	}
	add := func(value string, at token.Pos, fixed bool) {
		switch {
		case strings.HasPrefix(value, "-") && len(value) > 1 && value != "--":
			name, _, _ := strings.Cut(value, "=")
			record(flags, name, at)
		case fixed && OperationWord(value):
			record(words, value, at)
		}
	}
	for _, file := range files {
		fixed, element, flagSets := map[string]bool{}, map[string]bool{}, map[string]bool{}
		scopes := childScopes(file)
		child := func(id *ast.Ident) bool {
			for _, scope := range scopes {
				if scope.names[id.Name] && id.Pos() >= scope.start && id.Pos() < scope.end {
					return true
				}
			}
			return false
		}
		// bind records what NAME = RIGHT (an assignment or a var declaration) makes NAME: a
		// table, an argument at a fixed or computed position, or a flag set.
		bind := func(name string, right ast.Expr) {
			tables[name] = append(tables[name], right)
			if isFixed, isElement := position(right, fixed, element, child); isElement {
				fixed[name] = fixed[name] || isFixed
				element[name] = true
			}
			if call, ok := right.(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewFlagSet" {
					flagSets[name] = true
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				for index, name := range node.Names {
					if index < len(node.Values) {
						bind(name.Name, node.Values[index])
					}
				}
			case *ast.AssignStmt:
				if len(node.Lhs) != len(node.Rhs) {
					return true
				}
				for index, left := range node.Lhs {
					if indexed, ok := left.(*ast.IndexExpr); ok {
						// NAME["word"] = handler registers a table entry.
						if table, ok := indexed.X.(*ast.Ident); ok {
							if key, ok := stringLiteral(indexed.Index); ok {
								registered[table.Name] = append(registered[table.Name], key)
							}
						}
						continue
					}
					identifier, ok := left.(*ast.Ident)
					if !ok {
						continue
					}
					bind(identifier.Name, node.Rhs[index])
				}
			case *ast.RangeStmt:
				if _, ok := sliceName(node.X, child); ok {
					if value, ok := node.Value.(*ast.Ident); ok {
						element[value.Name] = true
					}
				}
			case *ast.FuncDecl:
				for _, field := range node.Type.Params.List {
					if star, ok := field.Type.(*ast.StarExpr); ok {
						if selector, ok := star.X.(*ast.SelectorExpr); ok && selector.Sel.Name == "FlagSet" {
							for _, name := range field.Names {
								flagSets[name.Name] = true
							}
						}
					}
				}
			case *ast.SwitchStmt:
				if node.Tag == nil {
					return true
				}
				if isFixed, isElement := position(node.Tag, fixed, element, child); isElement {
					for _, statement := range node.Body.List {
						for _, label := range statement.(*ast.CaseClause).List {
							if value, ok := stringLiteral(label); ok {
								add(value, label.Pos(), isFixed)
							}
						}
					}
				}
			case *ast.BinaryExpr:
				if node.Op != token.EQL && node.Op != token.NEQ {
					return true
				}
				for _, pair := range [][2]ast.Expr{{node.X, node.Y}, {node.Y, node.X}} {
					if value, ok := stringLiteral(pair[1]); ok {
						if isFixed, isElement := position(pair[0], fixed, element, child); isElement {
							add(value, pair[1].Pos(), isFixed)
						}
					}
				}
			case *ast.IndexExpr:
				if table, ok := node.X.(*ast.Ident); ok {
					if isFixed, _ := position(node.Index, fixed, element, child); isFixed {
						looked[table.Name] = true
					}
				}
			case *ast.CallExpr:
				selector, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				receiver, ok := selector.X.(*ast.Ident)
				argument, method := flagSetMethods[selector.Sel.Name]
				if !ok || !method || !(receiver.Name == "flag" || flagSets[receiver.Name]) || argument >= len(node.Args) {
					return true
				}
				if name, ok := stringLiteral(node.Args[argument]); ok && name != "" {
					record(flags, "--"+name, node.Args[argument].Pos())
				}
			}
			return true
		})
	}
	for table := range looked {
		for _, value := range tables[table] {
			for _, key := range compositeKeys(value) {
				add(key.value, key.at, true)
			}
		}
		for _, key := range registered[table] {
			if _, seen := words[key]; !seen && OperationWord(key) {
				words[key] = table + "[" + strconv.Quote(key) + "]"
			}
		}
	}
}

type keyAt struct {
	value string
	at    token.Pos
}

// compositeKeys returns the string keys of a map literal, or the string
// elements of a slice literal.
func compositeKeys(expression ast.Expr) []keyAt {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var keys []keyAt
	for _, element := range literal.Elts {
		candidate := element
		if pair, ok := element.(*ast.KeyValueExpr); ok {
			candidate = pair.Key
		}
		if value, ok := stringLiteral(candidate); ok {
			keys = append(keys, keyAt{value, candidate.Pos()})
		}
	}
	return keys
}
