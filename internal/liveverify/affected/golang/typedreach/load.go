// Package typedreach decides, for one Go test package, which of its test
// functions can reach a change, over a call graph built from the type-checked
// source of every module package its test binary links (AFP-V0-2370 to
// AFP-V0-2379). It is sound only within the model AFP-V0-2373 states: any
// code it cannot see keeps the package whole with a named reason.
package typedreach

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Package is one type-checked package of the test binary. Module packages
// carry their syntax and Info; standard-library packages are checked without
// function bodies and carry neither.
type Package struct {
	Path   string
	Dir    string
	Files  []*ast.File
	Names  []string
	Tests  map[*ast.File]bool
	Types  *types.Package
	Info   *types.Info
	Module bool
}

// Program is the test binary of one target package: the target with its
// internal test files, its external test package when it has one, and every
// package either imports.
type Program struct {
	Fset    *token.FileSet
	Module  string
	Root    string
	Context build.Context
	Target  *Package
	XTest   *Package
	byPath  map[string]*Package
	loading map[string]bool
	order   []*Package
}

// Packages returns the module packages of the program in load order.
func (program *Program) Packages() []*Package {
	var out []*Package
	for _, pkg := range program.order {
		if pkg.Module {
			out = append(out, pkg)
		}
	}
	return out
}

// Load fallback reasons (AFP-V0-2372).
var (
	ErrUnmodelled = errors.New("UNMODELLED_SOURCE")
	ErrTypeCheck  = errors.New("TYPE_CHECK_FAILED")
)

// Load type-checks the test binary of the package in directory dir (relative
// to root) for the host platform with cgo disabled, the configuration `go
// test` uses when CGO_ENABLED=0. Every module package keeps its function
// bodies; standard-library packages are checked from GOROOT source without
// them. A module package with cgo, assembly or a //go:linkname directive, or
// any type error, is an error wrapping ErrUnmodelled or ErrTypeCheck.
func Load(root, module, dir string) (*Program, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmodelled, err)
	}
	ctx := build.Default
	ctx.CgoEnabled = false
	ctx.GOOS, ctx.GOARCH = runtime.GOOS, runtime.GOARCH
	ctx.Dir = root
	program := &Program{Fset: token.NewFileSet(), Module: module, Root: root, Context: ctx, byPath: map[string]*Package{}, loading: map[string]bool{}}
	importPath := module
	if dir != "." && dir != "" {
		importPath = module + "/" + filepath.ToSlash(dir)
	}
	bp, err := ctx.ImportDir(filepath.Join(root, dir), 0)
	if err != nil {
		var none *build.NoGoError
		if !errors.As(err, &none) {
			return nil, fmt.Errorf("%w: %s: %v", ErrUnmodelled, dir, err)
		}
	}
	if err := moduleHazard(bp); err != nil {
		return nil, err
	}
	target, err := program.check(importPath, bp.Dir, bp.Name, append(slices.Clone(bp.GoFiles), bp.TestGoFiles...), bp.TestGoFiles, true)
	if err != nil {
		return nil, err
	}
	program.Target = target
	if len(bp.XTestGoFiles) > 0 {
		xtest, err := program.check(importPath+"_test", bp.Dir, bp.Name+"_test", bp.XTestGoFiles, bp.XTestGoFiles, true)
		if err != nil {
			return nil, err
		}
		program.XTest = xtest
	}
	return program, nil
}

// moduleHazard refuses a module package whose code the model cannot see.
func moduleHazard(bp *build.Package) error {
	switch {
	case len(bp.CgoFiles) > 0:
		return fmt.Errorf("%w: %s uses cgo", ErrUnmodelled, bp.ImportPath)
	case len(bp.SFiles)+len(bp.CFiles)+len(bp.SysoFiles)+len(bp.CXXFiles)+len(bp.MFiles)+len(bp.HFiles)+len(bp.FFiles)+len(bp.SwigFiles)+len(bp.SwigCXXFiles) > 0:
		return fmt.Errorf("%w: %s holds non-Go source", ErrUnmodelled, bp.Dir)
	}
	return nil
}

// Import implements types.Importer for the program's packages.
func (program *Program) Import(importPath string) (*types.Package, error) {
	return program.ImportFrom(importPath, program.Root, 0)
}

// ImportFrom implements types.ImporterFrom: module paths load from the module
// root, everything else from GOROOT (vendored standard-library paths
// resolved from the importing directory).
func (program *Program) ImportFrom(importPath, dir string, _ types.ImportMode) (*types.Package, error) {
	if importPath == "unsafe" {
		return types.Unsafe, nil
	}
	if importPath == "C" {
		return nil, fmt.Errorf("%w: import \"C\"", ErrUnmodelled)
	}
	if importPath == program.Module || strings.HasPrefix(importPath, program.Module+"/") {
		if pkg := program.byPath[importPath]; pkg != nil {
			return pkg.Types, nil
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(importPath, program.Module), "/")
		bp, err := program.Context.ImportDir(filepath.Join(program.Root, filepath.FromSlash(relative)), 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrUnmodelled, importPath, err)
		}
		if err := moduleHazard(bp); err != nil {
			return nil, err
		}
		pkg, err := program.check(importPath, bp.Dir, bp.Name, bp.GoFiles, nil, true)
		if err != nil {
			return nil, err
		}
		return pkg.Types, nil
	}
	bp, err := program.Context.Import(importPath, dir, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnmodelled, importPath, err)
	}
	if !bp.Goroot {
		return nil, fmt.Errorf("%w: %s is neither in the module nor in GOROOT", ErrUnmodelled, importPath)
	}
	if pkg := program.byPath[bp.ImportPath]; pkg != nil {
		return pkg.Types, nil
	}
	pkg, err := program.check(bp.ImportPath, bp.Dir, bp.Name, bp.GoFiles, nil, false)
	if err != nil {
		return nil, err
	}
	return pkg.Types, nil
}

func (program *Program) check(importPath, dir, name string, files, tests []string, module bool) (*Package, error) {
	if program.loading[importPath] {
		return nil, fmt.Errorf("%w: import cycle through %s", ErrTypeCheck, importPath)
	}
	program.loading[importPath] = true
	defer delete(program.loading, importPath)
	pkg := &Package{Path: importPath, Dir: dir, Module: module, Tests: map[*ast.File]bool{}}
	mode := parser.SkipObjectResolution
	if module {
		mode |= parser.ParseComments
	}
	for _, file := range files {
		full := filepath.Join(dir, file)
		source, err := os.ReadFile(full)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnmodelled, err)
		}
		parsed, err := parser.ParseFile(program.Fset, full, source, mode)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrTypeCheck, err)
		}
		if module && hasLinkname(parsed) {
			return nil, fmt.Errorf("%w: %s has a //go:linkname directive", ErrUnmodelled, full)
		}
		pkg.Files = append(pkg.Files, parsed)
		pkg.Names = append(pkg.Names, full)
		if slices.Contains(tests, file) {
			pkg.Tests[parsed] = true
		}
	}
	var first error
	config := types.Config{
		Importer:         program,
		IgnoreFuncBodies: !module,
		Error: func(err error) {
			if first == nil {
				first = err
			}
		},
		Sizes: types.SizesFor("gc", program.Context.GOARCH),
	}
	if module {
		pkg.Info = &types.Info{
			Types:      map[ast.Expr]types.TypeAndValue{},
			Defs:       map[*ast.Ident]types.Object{},
			Uses:       map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
			Instances:  map[*ast.Ident]types.Instance{},
			Implicits:  map[ast.Node]types.Object{},
		}
	}
	checked, _ := config.Check(importPath, program.Fset, pkg.Files, pkg.Info)
	if first != nil {
		if errors.Is(first, ErrUnmodelled) || errors.Is(first, ErrTypeCheck) {
			return nil, first
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrTypeCheck, importPath, first)
	}
	if checked.Name() != name && name != "" && !strings.HasSuffix(name, "_test") {
		return nil, fmt.Errorf("%w: %s declares package %s, want %s", ErrTypeCheck, importPath, checked.Name(), name)
	}
	pkg.Types = checked
	program.byPath[importPath] = pkg
	program.order = append(program.order, pkg)
	return pkg, nil
}

// hasLinkname reports a //go:linkname directive in a parsed file's comments.
func hasLinkname(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:linkname") {
				return true
			}
		}
	}
	return false
}

// relative is a module package's directory relative to the module root, in
// slash form.
func (program *Program) relative(pkg *Package) string {
	return path.Clean(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSuffix(pkg.Path, "_test"), program.Module), "/"))
}
