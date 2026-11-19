// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"context"
	"github.com/Beamfall/corvint/internal/linecov"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testinventory"
)

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull}, args...)
	out, err := exec.Command("git", argv...).CombinedOutput()
	if err != nil {
		t.Fatalf("git: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLEVV1005NativeCollectors(t *testing.T) {
	for _, runner := range []string{"vitest", "jest", "rspec", "minitest"} {
		t.Run(runner, func(t *testing.T) {
			modules := os.Getenv("JSLINECOV_NODE_MODULES")
			if runner == "vitest" || runner == "jest" {
				if modules == "" {
					t.Skip("JSLINECOV_NODE_MODULES not set")
				}
			} else {
				if _, err := exec.LookPath("ruby"); err != nil {
					t.Skip("Ruby unavailable")
				}
				if runner == "rspec" {
					if _, err := exec.LookPath("rspec"); err != nil {
						t.Skip("RSpec unavailable")
					}
				}
			}
			root := t.TempDir()
			files := map[string]string{".gitignore": "node_modules/\ncoverage/\n"}
			source := ""
			switch runner {
			case "vitest":
				files["package.json"] = `{"type":"module"}`
				files["vitest.config.mjs"] = `export default {};`
				files["calc.js"] = "export function add(a,b){return a+b;}\n"
				files["calc.test.js"] = "import {it,expect} from 'vitest'; import {add} from './calc.js'; it('adds',()=>expect(add(1,2)).toBe(3));\n"
				source = "calc.js"
			case "jest":
				files["package.json"] = `{"name":"lev-fixture","private":true}`
				files["jest.config.cjs"] = `module.exports={testEnvironment:'node'};`
				files["calc.js"] = "exports.add=(a,b)=>a+b;\n"
				files["calc.test.js"] = "const {add}=require('./calc'); test('adds',()=>expect(add(1,2)).toBe(3));\n"
				source = "calc.js"
			case "rspec":
				files[".rspec"] = "--require ./spec/spec_helper.rb\n"
				files["spec/spec_helper.rb"] = "require 'rspec'\n"
				files["lib/calc.rb"] = "def add(a,b)\n a+b\nend\n"
				files["spec/calc_spec.rb"] = "require_relative '../lib/calc'\nRSpec.describe 'Calc' do\n it('adds'){expect(add(1,2)).to eq(3)}\nend\n"
				source = "lib/calc.rb"
			case "minitest":
				files["test/test_helper.rb"] = "require 'minitest/autorun'\n"
				files["lib/calc.rb"] = "def add(a,b)\n a+b\nend\n"
				files["test/calc_test.rb"] = "require_relative 'test_helper'\nrequire_relative '../lib/calc'\nclass CalcTest < Minitest::Test\n def test_add\n assert_equal 3, add(1,2)\n end\nend\n"
				files["test/test_other.rb"] = "require_relative 'test_helper'\nrequire_relative '../lib/calc'\nclass OtherTest < Minitest::Test\n def test_other\n assert_equal 5, add(2,3)\n end\nend\n"
				source = "lib/calc.rb"
			}
			for name, body := range files {
				full := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if modules != "" && (runner == "vitest" || runner == "jest") {
				if err := os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
					t.Fatal(err)
				}
			}
			fixtureGit(t, root, "init", "-q")
			fixtureGit(t, root, "add", ".")
			fixtureGit(t, root, "commit", "-qm", "fixture")
			ctx := context.Background()
			git := testinventory.Git{Executable: "git", Root: root}
			refreshed, err := Refresh(ctx, Options{Git: git, Runner: runner})
			if err != nil {
				t.Fatal(err)
			}
			if len(refreshed) != 1 {
				t.Fatal(refreshed)
			}
			if runner == "minitest" {
				f, err := os.Open(refreshed[0].Path)
				if err != nil {
					t.Fatal(err)
				}
				ix, err := linecov.Decode(f)
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"minitest:test/calc_test.rb::CalcTest::test_add", "minitest:test/test_other.rb::OtherTest::test_other"} {
					if !slices.ContainsFunc(ix.Tests, func(test linecov.Test) bool { return test.Name == id }) {
						t.Fatalf("missing %s: %+v", id, ix.Tests)
					}
				}
			}
			tree, err := testinventory.NewWorkTree(ctx, git)
			if err != nil {
				t.Fatal(err)
			}
			blobs, _ := tree.Files()
			declared := testinventory.Detect(blobs, tree.Read)
			var d testinventory.Declared
			for _, found := range declared {
				if found.Runner == runner {
					d = found
				}
			}
			native, err := NativeIdentity(ctx, root, d)
			if err != nil {
				t.Fatal(err)
			}
			command := CommandForTree(d, blobs, declared)
			body, err := os.ReadFile(filepath.Join(root, source))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, source), []byte(strings.Replace(string(body), "a+b", "a+b+1", 1)), 0600); err != nil {
				t.Fatal(err)
			}
			edited, err := testinventory.NewWorkTree(ctx, git)
			if err != nil {
				t.Fatal(err)
			}
			current, _ := edited.Files()
			got := Read(Input{Path: refreshed[0].Path, Instance: d, Runner: Binding(d, blobs, command, native), Command: command, BaseFiles: blobs, CurrentFiles: current, Changed: []string{source}, Ancestor: func(string) bool { return true }})
			if got.Tier != File || got.Reason != Unqualified || len(got.Rank) == 0 {
				t.Fatalf("%+v", got)
			}
		})
	}
}

// Candidate admission has one owner. An unknown pattern widens to every Ruby
// candidate; the collector must not silently apply its own narrower allowlist.
func TestLEVV1005MinitestCandidateCommand(t *testing.T) {
	d := testinventory.Declared{Key: "minitest@ruby", Runner: "minitest", Root: "ruby", Config: "ruby/test/test_helper.rb"}
	files := map[string]string{"ruby/test/calc_test.rb": "a", "ruby/test/test_other.rb": "b", "ruby/unusual.rb": "c", "ruby/test/test_helper.rb": "d"}
	for _, unknown := range []bool{false, true} {
		if unknown {
			d.Pattern, d.Extensions = "future-pattern", []string{".rb"}
		}
		all := []testinventory.Declared{d}
		command := CommandForTree(d, files, all)
		want := []string{"test/calc_test.rb", "test/test_other.rb"}
		if unknown {
			want = append(want, "unusual.rb")
		}
		if !slices.Equal(command[len(Command(d)):], want) {
			t.Fatalf("unknown=%t: %q", unknown, command)
		}
	}
}
