// SPDX-License-Identifier: Apache-2.0

package clitree

import (
	"go/ast"
	"go/token"
)

// CAP-V0-200 (V1-3153): a slice named like arguments is not always this
// binary's command line. childArgvOf finds, per function, the names that are
// not: an argument-named []string parameter that every call in the package
// fills from a non-command-line value (wholeGoTest(invocation.Argv)), and a
// slice the function hands whole, sliced or spread to a child process
// (exec.Command, exec.CommandContext, syscall.Exec, a Cmd's Args) or builds
// from a []string literal. A name the function binds from its own command line
// (os.Args, a selector's Args, an Args() call, or an append of one) is never
// excluded. A name resolves in its innermost enclosing function only, so a
// closure's parameter never hides the same name in the function around it.

// funcScope is one function body and the names excluded inside it.
type funcScope struct {
	start, end token.Pos
	excluded   map[string]bool
}

// childArgvOf returns the predicate the scan consults for an identifier.
func childArgvOf(files []*ast.File) childArgv {
	nonCLI := nonCommandLineParams(files)
	var scopes []funcScope
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			var body *ast.BlockStmt
			excluded := map[string]bool{}
			switch fn := n.(type) {
			case *ast.FuncDecl:
				body = fn.Body
				for name := range nonCLI[fn] {
					excluded[name] = true
				}
			case *ast.FuncLit:
				body = fn.Body
			}
			if body == nil {
				return true
			}
			for name := range childNames(body) {
				excluded[name] = true
			}
			scopes = append(scopes, funcScope{body.Pos(), body.End(), excluded})
			return true
		})
	}
	return func(id *ast.Ident) bool {
		var inner *funcScope
		for index := range scopes {
			scope := &scopes[index]
			if id.Pos() >= scope.start && id.Pos() < scope.end && (inner == nil || scope.start >= inner.start) {
				inner = scope
			}
		}
		return inner != nil && inner.excluded[id.Name]
	}
}

// inspectOwn walks a function body without entering nested function literals,
// which are scopes of their own.
func inspectOwn(body *ast.BlockStmt, visit func(ast.Node)) {
	ast.Inspect(body, func(n ast.Node) bool {
		if _, nested := n.(*ast.FuncLit); nested {
			return false
		}
		if n != nil {
			visit(n)
		}
		return true
	})
}

// childNames returns the slices one body hands to a child process or builds
// from a literal, less the ones it binds from its own command line.
func childNames(body *ast.BlockStmt) map[string]bool {
	names, own := map[string]bool{}, map[string]bool{}
	var mark func(ast.Expr)
	mark = func(expression ast.Expr) {
		switch value := expression.(type) {
		case *ast.Ident:
			names[value.Name] = true
		case *ast.SliceExpr:
			mark(value.X)
		case *ast.ParenExpr:
			mark(value.X)
		case *ast.CallExpr:
			// append(prefix, argv...) builds the child argv from its arguments.
			if fn, ok := value.Fun.(*ast.Ident); ok && fn.Name == "append" {
				for _, argument := range value.Args {
					mark(argument)
				}
			}
		}
		// A single element (exec.Command("xdg-open", args[1])) is one string, not an argv.
	}
	bind := func(name string, right ast.Expr) {
		if commandLine(right) {
			own[name] = true
		}
		if stringSliceLiteral(right) {
			names[name] = true
		}
	}
	inspectOwn(body, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return
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
				return
			}
			for index, left := range node.Lhs {
				if selector, ok := left.(*ast.SelectorExpr); ok && selector.Sel.Name == "Args" {
					mark(node.Rhs[index])
				}
				if id, ok := left.(*ast.Ident); ok {
					bind(id.Name, node.Rhs[index])
				}
			}
		case *ast.ValueSpec:
			for index, name := range node.Names {
				if index < len(node.Values) {
					bind(name.Name, node.Values[index])
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
	})
	for name := range own {
		delete(names, name)
	}
	return names
}

// commandLine reports whether an expression reads this binary's own command
// line: os.Args (or a selector's Args field), an Args() call, a slice or
// element of one, or an append that spreads one in.
func commandLine(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.SelectorExpr:
		return value.Sel.Name == "Args"
	case *ast.CallExpr:
		if fn, ok := value.Fun.(*ast.Ident); ok && fn.Name == "append" {
			for _, argument := range value.Args {
				if commandLine(argument) {
					return true
				}
			}
			return false
		}
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
	return ok && isStringSlice(literal.Type)
}

func isStringSlice(expression ast.Expr) bool {
	array, ok := expression.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	element, ok := array.Elt.(*ast.Ident)
	return ok && element.Name == "string"
}

// callSite is one call of a package function, with the function it is in.
type callSite struct {
	caller *ast.FuncDecl
	args   []ast.Expr
}

// nonCommandLineParams returns, per function, its argument-named []string
// parameters that never receive this binary's command line. A function is a
// possible entry, so all its parameters stay command line, when it is exported,
// main or init, has no call in the package, or is named anywhere other than as
// the callee of a call (a handler table, a method value). Otherwise a parameter
// is command line when some call passes, at its position, anything but a
// struct field other than Args, a literal, an append of those, or a caller
// parameter that is itself not command line; this is iterated to a fixed point.
func nonCommandLineParams(files []*ast.File) map[*ast.FuncDecl]map[string]bool {
	byName := map[string][]*ast.FuncDecl{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Body != nil {
				byName[fn.Name.Name] = append(byName[fn.Name.Name], fn)
			}
		}
	}
	// params[fn][i] is the name of fn's i-th parameter when it is an argument-named []string.
	params := map[*ast.FuncDecl][]string{}
	for _, fns := range byName {
		for _, fn := range fns {
			var names []string
			for _, field := range fn.Type.Params.List {
				count := max(1, len(field.Names))
				for index := range count {
					name := ""
					if index < len(field.Names) && isStringSlice(field.Type) && argumentSlice(field.Names[index].Name) {
						name = field.Names[index].Name
					}
					names = append(names, name)
				}
			}
			params[fn] = names
		}
	}
	sites := map[string][]callSite{}
	escaped := map[string]bool{}
	for _, file := range files {
		var current *ast.FuncDecl
		callees := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				current = node
				callees[node.Name] = true // the declaration itself is no reference
			case *ast.CallExpr:
				var callee *ast.Ident
				switch fun := node.Fun.(type) {
				case *ast.Ident:
					callee = fun
				case *ast.SelectorExpr:
					callee = fun.Sel
				}
				if callee != nil && byName[callee.Name] != nil {
					callees[callee] = true
					sites[callee.Name] = append(sites[callee.Name], callSite{current, node.Args})
				}
			case *ast.Ident:
				if byName[node.Name] != nil && !callees[node] {
					escaped[node.Name] = true
				}
			}
			return true
		})
	}
	cli := map[*ast.FuncDecl]map[int]bool{}
	for name, fns := range byName {
		entry := ast.IsExported(name) || name == "main" || name == "init" || len(sites[name]) == 0 || escaped[name]
		for _, fn := range fns {
			cli[fn] = map[int]bool{}
			for index, param := range params[fn] {
				if param != "" && entry {
					cli[fn][index] = true
				}
			}
		}
	}
	paramIndex := func(fn *ast.FuncDecl, name string) (int, bool) {
		if fn == nil {
			return 0, false
		}
		for index, param := range params[fn] {
			if param == name {
				return index, true
			}
		}
		// Any other parameter of the caller is not an argument slice.
		for _, field := range fn.Type.Params.List {
			for _, id := range field.Names {
				if id.Name == name {
					return -1, true
				}
			}
		}
		return 0, false
	}
	// fromCommandLine is conservative: only a struct field other than Args (a
	// recorded invocation), a literal, or a caller parameter not yet shown to
	// receive the command line counts as not this binary's arguments.
	var fromCommandLine func(ast.Expr, *ast.FuncDecl) bool
	fromCommandLine = func(expression ast.Expr, caller *ast.FuncDecl) bool {
		switch value := expression.(type) {
		case *ast.Ident:
			if index, isParam := paramIndex(caller, value.Name); isParam {
				return index >= 0 && cli[caller][index]
			}
			return true
		case *ast.SelectorExpr:
			return value.Sel.Name == "Args"
		case *ast.CompositeLit, *ast.BasicLit:
			return false
		case *ast.SliceExpr:
			return fromCommandLine(value.X, caller)
		case *ast.ParenExpr:
			return fromCommandLine(value.X, caller)
		case *ast.CallExpr:
			if fn, ok := value.Fun.(*ast.Ident); ok && fn.Name == "append" {
				for _, argument := range value.Args {
					if fromCommandLine(argument, caller) {
						return true
					}
				}
				return false
			}
		}
		return true
	}
	for changed := true; changed; {
		changed = false
		for name, fns := range byName {
			for _, site := range sites[name] {
				for index, argument := range site.args {
					if !fromCommandLine(argument, site.caller) {
						continue
					}
					for _, fn := range fns {
						if index < len(params[fn]) && params[fn][index] != "" && !cli[fn][index] {
							cli[fn][index] = true
							changed = true
						}
					}
				}
			}
		}
	}
	out := map[*ast.FuncDecl]map[string]bool{}
	for fn, names := range params {
		for index, name := range names {
			if name != "" && !cli[fn][index] {
				if out[fn] == nil {
					out[fn] = map[string]bool{}
				}
				out[fn][name] = true
			}
		}
	}
	return out
}
