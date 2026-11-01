// Package testhooks finds the tests of one Go package that write a package-level
// test hook, directly or through a setter, so a test that does so is not run in
// parallel with its peers (ASD-V0-956, V1-2426).
//
// A hook write is an assignment, increment or decrement, in a function declared
// in a _test.go file, whose target is a package-level variable of the package
// under test: a plain identifier in the package's own test files, or
// importName.Var in its external _test package. A setter is any such function or
// method, including one declared in export_test.go, whose body (function literals
// included) writes a hook or calls another setter. Production code is not read
// for writes: its package state (caches, counters) is not a test hook. A method call on a package-level variable (a keyed sync.Map
// seam, for example) is not a write. Calls are matched by name, so the analysis
// is syntactic: it needs no build and no type checking.
package testhooks

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// User is one TestXxx function that writes a hook.
type User struct {
	Test string // the test function name
	File string // the file's base name
	// Parallel reports that the test body itself calls t.Parallel().
	Parallel bool
	// Via names the first hook variable or setter the test reaches.
	Via string
}

// Report is the result for one package directory.
type Report struct {
	Hooks   []string // package-level variables a test-file function writes
	Setters []string // test-file functions and methods, besides tests, that write a hook
	Users   []User
}

// Violations returns the users that call t.Parallel().
func (r Report) Violations() []User {
	var out []User
	for _, u := range r.Users {
		if u.Parallel {
			out = append(out, u)
		}
	}
	return out
}

type funcKey struct {
	external bool // declared in the _test package
	name     string
}

type funcInfo struct {
	decl     *ast.FuncDecl
	file     string
	imported string // the local name of the package under test, external files only
	writes   []string
	calls    []funcKey
}

// Analyze reads every .go file directly in dir. importPath is the package's
// import path, which identifies it in the external test package's imports.
func Analyze(dir, importPath string) (Report, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Report{}, err
	}
	fset := token.NewFileSet()
	var internal, external []*ast.File
	names := map[*ast.File]string{}
	pkgName := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return Report{}, err
		}
		names[file] = entry.Name()
		if strings.HasSuffix(file.Name.Name, "_test") && strings.HasSuffix(entry.Name(), "_test.go") {
			external = append(external, file)
			continue
		}
		if pkgName != "" && pkgName != file.Name.Name {
			return Report{}, fmt.Errorf("testhooks: %s: package %s, want %s", entry.Name(), file.Name.Name, pkgName)
		}
		pkgName = file.Name.Name
		internal = append(internal, file)
	}
	vars := packageVars(internal)
	funcs := map[funcKey]*funcInfo{}
	for _, file := range internal {
		if strings.HasSuffix(names[file], "_test.go") { // production writes (caches, counters) are no test hook
			collect(funcs, file, names[file], false, "", vars)
		}
	}
	for _, file := range external {
		local := ""
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path != importPath {
				continue
			}
			local = pkgName
			if spec.Name != nil {
				local = spec.Name.Name
			}
		}
		collect(funcs, file, names[file], true, local, vars)
	}
	return closure(funcs), nil
}

// packageVars names the variables declared at the top level of the package's files.
func packageVars(files []*ast.File) map[string]bool {
	vars := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if name.Name != "_" {
						vars[name.Name] = true
					}
				}
			}
		}
	}
	return vars
}

func collect(funcs map[funcKey]*funcInfo, file *ast.File, name string, external bool, imported string, vars map[string]bool) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		info := &funcInfo{decl: fn, file: name, imported: imported}
		locals := localNames(fn)
		target := func(expr ast.Expr) string { // the hook an assignment target names, if any
			for {
				switch e := expr.(type) {
				case *ast.IndexExpr:
					expr = e.X
					continue
				case *ast.StarExpr:
					expr = e.X
					continue
				case *ast.SelectorExpr:
					if id, ok := e.X.(*ast.Ident); ok && external && imported != "" && id.Name == imported {
						if vars[e.Sel.Name] {
							return e.Sel.Name
						}
						return ""
					}
					expr = e.X
					continue
				case *ast.Ident:
					if !external && vars[e.Name] && !locals[e.Name] {
						return e.Name
					}
				}
				return ""
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					return true
				}
				for _, lhs := range s.Lhs {
					if hook := target(lhs); hook != "" {
						info.writes = append(info.writes, hook)
					}
				}
			case *ast.IncDecStmt:
				if hook := target(s.X); hook != "" {
					info.writes = append(info.writes, hook)
				}
			case *ast.CallExpr:
				switch f := s.Fun.(type) {
				case *ast.Ident:
					if !locals[f.Name] {
						info.calls = append(info.calls, funcKey{external, f.Name})
					}
				case *ast.SelectorExpr:
					if id, ok := f.X.(*ast.Ident); ok && external && imported != "" && id.Name == imported {
						info.calls = append(info.calls, funcKey{false, f.Sel.Name})
					} else {
						// A method call: matched by method name within the same package.
						info.calls = append(info.calls, funcKey{external, "." + f.Sel.Name})
					}
				}
			}
			return true
		})
		key := funcKey{external, fn.Name.Name}
		if fn.Recv != nil {
			key.name = "." + fn.Name.Name
		}
		if prior, ok := funcs[key]; ok { // two methods of one name: merge them, conservatively
			prior.writes = append(prior.writes, info.writes...)
			prior.calls = append(prior.calls, info.calls...)
			continue
		}
		funcs[key] = info
	}
}

// localNames is every name the function declares (parameters, results and
// variables in its body), which shadow a package variable of the same name.
// A shadowing name anywhere in the body counts for the whole body: a miss
// there under-reports a write, so a test that relies on it keeps its own review.
func localNames(fn *ast.FuncDecl) map[string]bool {
	locals := map[string]bool{}
	addFields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				locals[name.Name] = true
			}
		}
	}
	addFields(fn.Recv)
	addFields(fn.Type.Params)
	addFields(fn.Type.Results)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE {
				for _, lhs := range s.Lhs {
					if id, ok := lhs.(*ast.Ident); ok {
						locals[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			for _, name := range s.Names {
				locals[name.Name] = true
			}
		case *ast.FuncLit:
			addFields(s.Type.Params)
			addFields(s.Type.Results)
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{s.Key, s.Value} {
				if id, ok := e.(*ast.Ident); ok && s.Tok == token.DEFINE {
					locals[id.Name] = true
				}
			}
		}
		return true
	})
	return locals
}

func closure(funcs map[funcKey]*funcInfo) Report {
	via := map[funcKey]string{}
	for key, info := range funcs {
		if len(info.writes) > 0 {
			via[key] = info.writes[0]
		}
	}
	for changed := true; changed; {
		changed = false
		for key, info := range funcs {
			if _, ok := via[key]; ok {
				continue
			}
			for _, call := range info.calls {
				if _, ok := via[call]; ok {
					via[key] = strings.TrimPrefix(call.name, ".")
					changed = true
					break
				}
			}
		}
	}
	var report Report
	hooks := map[string]bool{}
	for _, info := range funcs {
		for _, hook := range info.writes {
			hooks[hook] = true
		}
	}
	for hook := range hooks {
		report.Hooks = append(report.Hooks, hook)
	}
	for key, info := range funcs {
		hook, ok := via[key]
		if !ok {
			continue
		}
		if isTest(info.decl) {
			report.Users = append(report.Users, User{Test: info.decl.Name.Name, File: info.file, Parallel: callsParallel(info.decl), Via: hook})
		} else {
			report.Setters = append(report.Setters, strings.TrimPrefix(key.name, "."))
		}
	}
	sort.Strings(report.Hooks)
	sort.Strings(report.Setters)
	report.Setters = dedupe(report.Setters)
	sort.Slice(report.Users, func(i, j int) bool { return report.Users[i].Test < report.Users[j].Test })
	return report
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// isTest reports a top-level func TestXxx(t *testing.T).
func isTest(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
		return false
	}
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 {
		return false
	}
	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "T"
}

// callsParallel reports a statement t.Parallel() directly in the test body.
func callsParallel(fn *ast.FuncDecl) bool {
	t := fn.Type.Params.List[0].Names[0].Name
	for _, stmt := range fn.Body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == t {
				return true
			}
		}
	}
	return false
}
