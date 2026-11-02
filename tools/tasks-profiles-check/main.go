// SPDX-License-Identifier: Apache-2.0
// tasks-profiles-check fails, naming the profile and its file, when a
// taskman-NAME/N profile literal in the non-test sources under internal/tasks
// is not in exactly one of cli.LiveFormats, outputOnlyProfiles and
// adoptOnlyProfiles (internal/tasks/cli/live_upgrade_test.go). It is the
// static twin of TestCALV0131_LiveFormatsCoverEveryDecodedProfile, which runs
// only inside the long internal/tasks/cli package, so an unregistered profile
// fails in doc-gates in about a second (TCL-V0-815).
//
// LiveFormats is read from its source: a string literal entry counts as
// itself, and an identifier or package selector counts as the string literal
// its constant or variable is declared with. An entry this cannot resolve,
// such as a computed value, is left to the runtime test.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const module = "github.com/Beamfall/corvint/"

var profileLiteral = regexp.MustCompile(`^taskman-[a-z0-9-]+/[0-9]+$`)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tasks-profiles-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	flags.Usage = func() {
		fmt.Fprint(stderr, `usage: tasks-profiles-check [--root DIR]

Fails, naming each profile and file, when a taskman-NAME/N literal in the
non-test sources under internal/tasks is not in exactly one of cli.LiveFormats,
outputOnlyProfiles and adoptOnlyProfiles (TCL-V0-815). Writes nothing.

Example: go run ./tools/tasks-profiles-check
`)
	}
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		if err == flag.ErrHelp {
			return 0
		}
		flags.Usage()
		return 2
	}
	problems, err := check(*root)
	if err != nil {
		fmt.Fprintln(stderr, "tasks-profiles-check:", err)
		return 2
	}
	for _, p := range problems {
		fmt.Fprintln(stderr, "tasks-profiles-check:", p)
	}
	if len(problems) > 0 {
		return 1
	}
	fmt.Fprintln(stdout, "tasks-profiles-check: ok")
	return 0
}

// reference is a LiveFormats entry naming a constant or variable of the
// package in dir.
type reference struct{ dir, name string }

func check(root string) ([]string, error) {
	tasks := filepath.Join(root, "internal", "tasks")
	named := map[string]string{}       // profile -> file naming it
	declared := map[reference]string{} // declaration -> its literal profile
	live := map[string]bool{}          // profiles LiveFormats names directly
	var references []reference         // LiveFormats entries to resolve
	foundLiveFormats := false
	fset := token.NewFileSet()
	err := filepath.WalkDir(tasks, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		dir := path.Dir(rel)
		var body *ast.BlockStmt
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if dir == "internal/tasks/cli" && decl.Recv == nil && decl.Name.Name == "LiveFormats" {
					body, foundLiveFormats = decl.Body, true
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range value.Names {
						if i < len(value.Values) {
							if profile, ok := profileOf(value.Values[i]); ok {
								declared[reference{dir, name.Name}] = profile
							}
						}
					}
				}
			}
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			importPath, _ := strconv.Unquote(spec.Path.Value)
			name := path.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if strings.HasPrefix(importPath, module) {
				imports[name] = strings.TrimPrefix(importPath, module)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if body != nil && n == body {
				// LiveFormats' own literals do not count as a source naming them.
				collect(body, dir, imports, live, &references)
				return false
			}
			if profile, ok := profileOf(n); ok {
				named[profile] = rel
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !foundLiveFormats {
		return nil, fmt.Errorf("internal/tasks/cli declares no LiveFormats function")
	}
	for _, ref := range references {
		if profile, ok := declared[ref]; ok {
			live[profile] = true
		}
	}
	output, adopt, err := registered(filepath.Join(tasks, "cli", "live_upgrade_test.go"), fset)
	if err != nil {
		return nil, err
	}
	var problems []string
	for profile, file := range named {
		lists := 0
		for _, in := range []bool{live[profile], slices.Contains(output, profile), slices.Contains(adopt, profile)} {
			if in {
				lists++
			}
		}
		if lists != 1 {
			problems = append(problems, fmt.Sprintf("%s (%s) must be in exactly one of LiveFormats, outputOnlyProfiles and adoptOnlyProfiles", profile, file))
		}
	}
	sort.Strings(problems)
	return problems, nil
}

// collect records each LiveFormats entry: a profile literal directly, and an
// identifier or a selector of a module package as a reference to resolve.
func collect(body *ast.BlockStmt, dir string, imports map[string]string, live map[string]bool, references *[]reference) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BasicLit:
			if profile, ok := profileOf(n); ok {
				live[profile] = true
			}
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok {
				if target, ok := imports[pkg.Name]; ok {
					*references = append(*references, reference{target, n.Sel.Name})
				}
			}
			return false
		case *ast.Ident:
			*references = append(*references, reference{dir, n.Name})
		}
		return true
	})
}

// registered reads the string literals of the outputOnlyProfiles and
// adoptOnlyProfiles variables.
func registered(file string, fset *token.FileSet) (output, adopt []string, err error) {
	parsed, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	lists := map[string]*[]string{"outputOnlyProfiles": &output, "adoptOnlyProfiles": &adopt}
	found := 0
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				list, ok := lists[name.Name]
				if !ok || i >= len(value.Values) {
					continue
				}
				found++
				ast.Inspect(value.Values[i], func(n ast.Node) bool {
					if profile, ok := profileOf(n); ok {
						*list = append(*list, profile)
					}
					return true
				})
			}
		}
	}
	if found != len(lists) {
		return nil, nil, fmt.Errorf("%s: want the outputOnlyProfiles and adoptOnlyProfiles variables", filepath.ToSlash(file))
	}
	return output, adopt, nil
}

func profileOf(n ast.Node) (string, bool) {
	lit, ok := n.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || !profileLiteral.MatchString(value) {
		return "", false
	}
	return value, true
}
