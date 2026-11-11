// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const runnernamesPath = "github.com/Beamfall/corvint/internal/langplugin/runnernames"

// pluginChoiceBranches reports each branch of a non-test function in the
// given package sources that names a runner, through a runnernames constant
// or a registered runner literal, and chooses a plugin type: it builds a type
// of the package that has a Pieces method, or calls a package function that
// returns one or a RunnerPlugin (AFP-V0-9986): a switch case or an if whose
// body or else does it. A family factory's admission guard, which refuses an
// unsupported runner before building its one plugin type, chooses nothing.
func pluginChoiceBranches(t *testing.T, sources map[string]string, runners []string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	registered := map[string]bool{}
	for _, runner := range runners {
		registered[runner] = true
	}
	files := map[string]*ast.File{}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	pluginTypes := map[string]bool{"RunnerPlugin": true}
	for _, name := range names {
		file, err := parser.ParseFile(fileSet, name, sources[name], parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = file
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == "Pieces" && len(fn.Recv.List) == 1 {
				if base := baseTypeName(fn.Recv.List[0].Type); base != "" {
					pluginTypes[base] = true
				}
			}
		}
	}
	pluginFuncs := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Type.Results != nil {
				for _, result := range fn.Type.Results.List {
					if pluginTypes[baseTypeName(result.Type)] {
						pluginFuncs[fn.Name.Name] = true
					}
				}
			}
		}
	}
	var findings []string
	for _, name := range names {
		file := files[name]
		alias := ""
		for _, spec := range file.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == runnernamesPath {
				alias = "runnernames"
				if spec.Name != nil {
					alias = spec.Name.Name
				}
			}
		}
		namesRunner := func(node ast.Node) string {
			found := ""
			ast.Inspect(node, func(n ast.Node) bool {
				switch typed := n.(type) {
				case *ast.SelectorExpr:
					if ident, ok := typed.X.(*ast.Ident); ok && alias != "" && ident.Name == alias {
						found = alias + "." + typed.Sel.Name
					}
				case *ast.BasicLit:
					if value, err := strconv.Unquote(typed.Value); err == nil && typed.Kind == token.STRING && registered[value] {
						found = typed.Value
					}
				}
				return found == ""
			})
			return found
		}
		choosesPlugin := func(nodes ...ast.Node) bool {
			chosen := false
			for _, node := range nodes {
				if node == nil || reflect.ValueOf(node).IsNil() {
					continue
				}
				ast.Inspect(node, func(n ast.Node) bool {
					switch typed := n.(type) {
					case *ast.CompositeLit:
						chosen = chosen || pluginTypes[baseTypeName(typed.Type)]
					case *ast.CallExpr:
						if ident, ok := typed.Fun.(*ast.Ident); ok && pluginFuncs[ident.Name] {
							chosen = true
						}
					}
					return !chosen
				})
			}
			return chosen
		}
		report := func(node ast.Node, runner string) {
			findings = append(findings, fmt.Sprintf("%s: branch on %s chooses a plugin type", fileSet.Position(node.Pos()), runner))
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch typed := n.(type) {
				case *ast.CaseClause:
					for _, expr := range typed.List {
						if runner := namesRunner(expr); runner != "" && choosesPlugin(&ast.BlockStmt{List: typed.Body}) {
							report(typed, runner)
							break
						}
					}
				case *ast.IfStmt:
					if runner := namesRunner(typed.Cond); runner != "" && choosesPlugin(typed.Body, typed.Else) {
						report(typed, runner)
					}
				}
				return true
			})
		}
	}
	return findings
}

// baseTypeName is the name of a type expression's named type, through a
// pointer; "" for any other type expression.
func baseTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return baseTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	}
	return ""
}

// registrySources returns this package's non-test Go sources.
func registrySources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(raw)
	}
	return sources
}

// selectionSwitch5bfe17dc6 is SelectionPlugin as commit 5bfe17dc6 left it, with
// the Ruby plugin type it chose (V1-5351), and the test-unit identity branch of
// the Ruby plugin's Evidence, which names a runner without choosing a plugin.
const selectionSwitch5bfe17dc6 = `package registry

import "github.com/Beamfall/corvint/internal/langplugin/runnernames"

type RunnerPlugin interface{ Pieces() int }
type RubyPlugin struct{ runner string }
type absentPlugin struct{ runner string }

func (p *RubyPlugin) Pieces() int  { return 1 }
func (p absentPlugin) Pieces() int { return 0 }

func SelectionPlugin(runner string) (RunnerPlugin, bool) {
	switch runner {
	case runnernames.RunnerRspec, runnernames.RunnerMinitest, runnernames.RunnerTestUnit, runnernames.RunnerRailsTest:
		return &RubyPlugin{runner: runner}, true
	}
	return absentPlugin{runner}, true
}

func (p *RubyPlugin) identity(name string) string {
	if p.runner == runnernames.RunnerTestUnit {
		return "method(" + name + ")"
	}
	return name
}
`

// The same choice spelled with a literal, an if and a factory call, beside a
// family factory's admission guard, which chooses nothing.
const selectionLiterals = `package registry

type RunnerPlugin interface{ Pieces() int }
type RubyPlugin struct{ runner string }
type absentPlugin struct{ runner string }

func (p *RubyPlugin) Pieces() int  { return 1 }
func (p absentPlugin) Pieces() int { return 0 }
func newRuby(runner string) RunnerPlugin { return &RubyPlugin{runner: runner} }

func SelectionPlugin(runner string) RunnerPlugin {
	if runner == "rspec" {
		return newRuby(runner)
	}
	return absentPlugin{runner}
}

func NewRubyPlugin(runner string) (RunnerPlugin, bool) {
	if runner != "rspec" && runner != "minitest" {
		return nil, false
	}
	return &RubyPlugin{runner: runner}, true
}
`

// AFP-V0-9986 (V1-5351): no function of this package branches on a runner name
// to choose a plugin type; the baseline is empty. The 5bfe17dc6 switch and its
// literal if spelling are reported; a branch that only names a runner, and a
// family factory's admission guard, are not.
func TestRegistryChoosesNoPluginByRunnerName_AFPV09986(t *testing.T) {
	for _, finding := range pluginChoiceBranches(t, registrySources(t), registry.Runners()) {
		t.Errorf("%s: resolve the plugin from the registered family table (AFP-V0-9985)", finding)
	}
	findings := pluginChoiceBranches(t, map[string]string{"runner_plugin.go": selectionSwitch5bfe17dc6}, registry.Runners())
	if len(findings) != 1 || !strings.Contains(findings[0], "runner_plugin.go:14:") || !strings.Contains(findings[0], "runnernames.Runner") {
		t.Errorf("the 5bfe17dc6 switch: findings %q, want exactly the case at line 14", findings)
	}
	findings = pluginChoiceBranches(t, map[string]string{"literal.go": selectionLiterals}, registry.Runners())
	if len(findings) != 1 || !strings.Contains(findings[0], `branch on "rspec"`) {
		t.Errorf("the literal if choice: findings %q, want exactly the SelectionPlugin branch", findings)
	}
}

// AFP-V0-9985 (V1-5351): the table-driven lookup returns what the per-family
// switch returned for every registered runner: the unbound Ruby plugin for the
// four Ruby runners, the absent plugin for every other one, and nothing for an
// unregistered name.
func TestSelectionPluginParity_AFPV09985(t *testing.T) {
	ruby := map[string]bool{"rspec": true, "minitest": true, "test-unit": true, "rails-test": true}
	seen := 0
	for _, runner := range registry.Runners() {
		plugin, ok := registry.SelectionPlugin(runner)
		if !ok || plugin == nil {
			t.Fatalf("%s: registered runner has no selection plugin", runner)
		}
		_, isRuby := plugin.(*registry.RubyPlugin)
		switch {
		case ruby[runner]:
			seen++
			pieces := plugin.Pieces()
			if !isRuby || !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present {
				t.Errorf("%s: %T %+v, want the unbound Ruby plugin", runner, plugin, pieces)
			}
		case isRuby || !reflect.DeepEqual(plugin.Pieces(), tr.AbsentRunnerPieces(runner)):
			t.Errorf("%s: %T %+v, want the absent plugin", runner, plugin, plugin.Pieces())
		}
	}
	if seen != len(ruby) {
		t.Errorf("found %d of the %d Ruby runners in the registry", seen, len(ruby))
	}
	if plugin, ok := registry.SelectionPlugin("unregistered-runner"); ok || plugin != nil {
		t.Errorf("an unregistered runner got %T", plugin)
	}
}
