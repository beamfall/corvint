// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func fixtureGitEnvironment() []string {
	return testsupport.WithoutGitIdentity(os.Environ())
}

func fixtureGitCommand(arguments ...string) *exec.Cmd {
	command := exec.Command("git", arguments...)
	command.Env = fixtureGitEnvironment()
	return command
}

func fixtureGitCommandContext(ctx context.Context, arguments ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Env = fixtureGitEnvironment()
	return command
}

// CCF-V1-9800: ambient identity and dates cannot change the frozen fixture SHA.
func TestFixtureGitIdentityIsHermetic(t *testing.T) {
	variables := []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE",
		"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE"}
	for _, name := range variables {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	root := evidenceSummaryRepository(t)
	want := gitFixture(t, root, "rev-parse", "HEAD")
	for _, name := range variables {
		value := "Foreign Fixture Caller"
		if strings.HasSuffix(name, "EMAIL") {
			value = "foreign@example.invalid"
		} else if strings.HasSuffix(name, "DATE") {
			value = "2033-04-05T06:07:08+0300"
		}
		t.Setenv(name, value)
	}
	root = evidenceSummaryRepository(t)
	if got := gitFixture(t, root, "rev-parse", "HEAD"); got != want {
		t.Fatalf("foreign identity changed fixture commit: got %s, want %s", got, want)
	}
	t.Run("context-golden", TestContextDefaultWireIsTheGolden)
	t.Run("lsp-golden", TestContextLSPOffKeepsTheGoldenAndOnDegrades)
	t.Run("core-profiles", TestCoreVerbsEmitTheFrozenProfiles)
}

// CCF-V1-007, CCF-V1-9800: N-1 exceptions must survive nested fixture replay.
func TestFixtureGitIdentityIsHermeticWithN1Replay(t *testing.T) {
	if os.Getenv("CORVINT_CORE_N1_BINARY") == "" {
		t.Skip("set CORVINT_CORE_N1_BINARY to enable nested N-1 replay")
	}
	t.Run("nested", TestFixtureGitIdentityIsHermetic)
}

// CCF-V1-9800: all direct Git commands must use the shared fixture environment,
// and assignments must not replace it with the unfiltered process environment.
func TestFixtureGitCommandsUseHermeticEnvironment(t *testing.T) {
	paths, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || path == "git_fixture_test.go" && (function.Name.Name == "fixtureGitCommand" || function.Name.Name == "fixtureGitCommandContext") {
				continue
			}
			gitCommands := map[string]bool{}
			ast.Inspect(function, func(node ast.Node) bool {
				assignment, ok := node.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for index, value := range assignment.Rhs {
					call, ok := value.(*ast.CallExpr)
					if !ok {
						continue
					}
					name, ok := call.Fun.(*ast.Ident)
					if !ok || (name.Name != "fixtureGitCommand" && name.Name != "fixtureGitCommandContext") || index >= len(assignment.Lhs) {
						continue
					}
					if target, ok := assignment.Lhs[index].(*ast.Ident); ok {
						gitCommands[target.Name] = true
					}
				}
				return true
			})
			ast.Inspect(function, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				owner, ok := selector.X.(*ast.Ident)
				if !ok || owner.Name != "exec" || (selector.Sel.Name != "Command" && selector.Sel.Name != "CommandContext") {
					return true
				}
				index := 0
				if selector.Sel.Name == "CommandContext" {
					index = 1
				}
				if len(call.Args) > index {
					if literal, ok := call.Args[index].(*ast.BasicLit); ok && literal.Value == `"git"` {
						t.Errorf("%s: use fixtureGitCommand", fset.Position(call.Pos()))
					}
				}
				return true
			})
			if len(gitCommands) == 0 {
				continue
			}
			ast.Inspect(function, func(node ast.Node) bool {
				assignment, ok := node.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, target := range assignment.Lhs {
					selector, ok := target.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "Env" {
						continue
					}
					owner, ok := selector.X.(*ast.Ident)
					if !ok || !gitCommands[owner.Name] {
						continue
					}
					for _, value := range assignment.Rhs {
						ast.Inspect(value, func(node ast.Node) bool {
							call, ok := node.(*ast.CallExpr)
							if !ok {
								return true
							}
							if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Environ" {
								if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "os" {
									t.Errorf("%s: use fixtureGitEnvironment for Git environments", fset.Position(call.Pos()))
								}
							}
							return true
						})
					}
				}
				return true
			})
		}
	}
}
