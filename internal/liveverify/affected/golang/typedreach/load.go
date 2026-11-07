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
	"go/build/constraint"
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
	Fset          *token.FileSet
	Module        string
	Root          string
	Context       build.Context
	Target        *Package
	XTest         *Package
	byPath        map[string]*Package
	loading       map[string]bool
	order         []*Package
	overrides     map[string][]byte
	overridesRead map[string]bool
	config        *Config
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
// to root) for the host platform. With no executor configuration, explicit
// module build constraints refuse narrowing: local and CI runner tags may
// select different source. Cgo is enabled for discovery so cgo source cannot
// disappear before refusal. LoadFor evaluates the constraints instead.
// Every module package keeps its function
// bodies; standard-library packages are checked from GOROOT source without
// them. A module package with cgo, assembly or a //go:linkname directive, or
// any type error, is an error wrapping ErrUnmodelled or ErrTypeCheck.
func Load(root, module, dir string) (*Program, error) {
	return load(root, module, dir, nil, nil)
}

// LoadFor is Load under the executor's build configuration (AFP-V0-2372):
// build constraints select exactly the source the run compiles, so
// constrained source is modelled rather than refused. The caller binds
// config.Identity() into the analysis identity.
func LoadFor(config Config, root, module, dir string) (*Program, error) {
	return load(root, module, dir, nil, &config)
}

// LoadBase checks the changed files at base without writing the worktree
// (AFP-V0-2376). Keys are root-relative paths; nil means absent at base.
// Incompatible head tests, changed build constraints and deleted files fail
// closed rather than pretending this overlay is a complete historical tree.
func LoadBase(root, module, dir string, files map[string][]byte) (*Program, error) {
	return loadBase(root, module, dir, files, nil)
}

// LoadBaseFor is LoadBase under the executor's build configuration. A base
// file whose build constraint differs from its head file's fails closed: the
// two trees could select different source.
func LoadBaseFor(config Config, root, module, dir string, files map[string][]byte) (*Program, error) {
	return loadBase(root, module, dir, files, &config)
}

func loadBase(root, module, dir string, files map[string][]byte, config *Config) (*Program, error) {
	for name, source := range files {
		if config == nil && hasBuildConstraint(source) {
			return nil, fmt.Errorf("%w: constrained base file %s", ErrUnmodelled, name)
		}
		if source != nil {
			head, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				return nil, fmt.Errorf("%w: base file %s is absent at head", ErrUnmodelled, name)
			}
			if config != nil && !slices.Equal(constraintLines(source), constraintLines(head)) {
				return nil, fmt.Errorf("%w: base file %s changes its build constraint", ErrUnmodelled, name)
			}
		}
	}
	program, err := load(root, module, dir, files, config)
	if err != nil {
		return nil, err
	}
	for name, source := range files {
		if source == nil || program.overridesRead[name] {
			continue
		}
		for _, pkg := range program.Packages() {
			if filepath.Clean(pkg.Dir) == filepath.Join(program.Root, filepath.FromSlash(path.Dir(name))) {
				return nil, fmt.Errorf("%w: base file %s was not loaded", ErrUnmodelled, name)
			}
		}
	}
	return program, nil
}

func load(root, module, dir string, overrides map[string][]byte, config *Config) (*Program, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmodelled, err)
	}
	ctx := build.Default
	ctx.CgoEnabled = true
	ctx.GOOS, ctx.GOARCH = runtime.GOOS, runtime.GOARCH
	if config != nil {
		ctx.GOROOT, ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled, ctx.Compiler, ctx.UseAllFiles = config.GOROOT, config.GOOS, config.GOARCH, config.CgoEnabled, config.Compiler, config.UseAllFiles
		ctx.BuildTags, ctx.ToolTags, ctx.ReleaseTags = slices.Clone(config.BuildTags), slices.Clone(config.ToolTags), slices.Clone(config.ReleaseTags)
	}
	ctx.Dir = root
	program := &Program{Fset: token.NewFileSet(), Module: module, Root: root, Context: ctx, byPath: map[string]*Package{}, loading: map[string]bool{}, overrides: overrides, overridesRead: map[string]bool{}, config: config}
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
	if err := program.runnerSourceParity(bp); err != nil {
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

// runnerSourceParity refuses, when the program carries no executor
// configuration, even currently ignored constrained files: without the
// executor's tags (including CI's -race), their absence from this graph
// cannot prove their absence from the test binary. Under a configuration the
// build context already selected exactly the files the run compiles.
func (program *Program) runnerSourceParity(bp *build.Package) error {
	if program.config != nil {
		return nil
	}
	for _, names := range [][]string{bp.GoFiles, bp.TestGoFiles, bp.XTestGoFiles, bp.IgnoredGoFiles} {
		for _, name := range names {
			source, err := os.ReadFile(filepath.Join(bp.Dir, name))
			if err != nil {
				return fmt.Errorf("%w: runner source %s: %v", ErrUnmodelled, name, err)
			}
			if hasBuildConstraint(source) {
				return fmt.Errorf("%w: build-constrained source %s: runner configuration parity unproved", ErrUnmodelled, filepath.Join(bp.Dir, name))
			}
		}
	}
	return nil
}

func hasBuildConstraint(source []byte) bool {
	return len(constraintLines(source)) > 0
}

// constraintLines returns the //go:build and // +build lines of source.
func constraintLines(source []byte) []string {
	var out []string
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) {
			out = append(out, line)
		}
	}
	return out
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
		if err := program.runnerSourceParity(bp); err != nil {
			return nil, err
		}
		pkg, err := program.check(importPath, bp.Dir, bp.Name, bp.GoFiles, nil, true)
		if err != nil {
			return nil, err
		}
		return pkg.Types, nil
	}
	// GOROOT is used only for signatures and is governed by the standard
	// library effect allow-list. Its generated cgo definitions are not source
	// inputs to go/types; module discovery above still enables cgo to refuse it.
	stdlib := program.Context
	stdlib.CgoEnabled = false
	bp, err := stdlib.Import(importPath, dir, 0)
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
		relative, _ := filepath.Rel(program.Root, full)
		key := filepath.ToSlash(relative)
		source, overridden := program.overrides[key]
		var err error
		if module && overridden {
			program.overridesRead[key] = true
			if source == nil {
				continue
			}
		} else {
			source, err = os.ReadFile(full)
		}
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
	if len(pkg.Files) == 0 {
		return nil, fmt.Errorf("%w: %s has no source at base", ErrTypeCheck, importPath)
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
