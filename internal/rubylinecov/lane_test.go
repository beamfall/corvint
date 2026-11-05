// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

func laneProject(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "test"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test", "case_test.rb"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	return root
}

// LPCV-V0-7605: isolation divergence and unobservable children preserve fallback.
func TestIsolationAndChildren(t *testing.T) {
	requireRuby(t, "minitest")
	cases := []struct{ name, body, want string }{
		{"order", "def test_a; $state = true; assert true; end\ndef test_b; assert $state; end", "isolation-divergent"},
		{"cleared", "def test_child; assert system({'RUBYOPT'=>nil}, 'ruby', '-e', 'exit 0'); end", "uninstrumented-child"},
		{"nonruby", "def test_child; assert system('/bin/true'); end", "uninstrumented-child"},
		{"abrupt-fork", "def test_child; pid = fork { exit! 0 }; Process.wait(pid); assert true; end", "uninstrumented-child"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := laneProject(t, "require 'minitest/autorun'\nclass LaneTest < Minitest::Test\n i_suck_and_my_tests_are_order_dependent!\n"+c.body+"\nend\n")
			run, err := Collect(context.Background(), Options{Root: root, Work: t.TempDir(), Argv: []string{"ruby", "test/case_test.rb"}, Timeout: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, o := range run.Outcomes {
				if strings.Contains(o.Reason, c.want) && !o.Recorded {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s: %+v", c.want, run.Outcomes)
			}
			head := git(t, root, "rev-parse", "HEAD")
			oids, err := linecov.OIDs(context.Background(), root, head)
			if err != nil {
				t.Fatal(err)
			}
			ix, err := Index(".", head, run, oids)
			if err != nil {
				t.Fatal(err)
			}
			p, _ := Plan(ix, []string{"test/case_test.rb"}, linecov.Effect{}, linecov.Effect{})
			if p["test/case_test.rb"].Mode != linecov.Whole {
				t.Fatalf("unsafe plan %+v", p)
			}
		})
	}
}

// LPCV-V0-7605: a syntax edit within a never-called method is conservative.
func TestUncalledMethodFallback(t *testing.T) {
	ix := &linecov.Index{Format: linecov.Format, Module: ".", Commit: "c", Runner: "ruby", Files: []linecov.File{{Path: "lib/a.rb", OID: "oid", Blocks: []linecov.Block{{Start: 1, StartCol: 1, End: 5, EndCol: 4}}}}}
	ix, err := linecov.Merge(ix)
	if err != nil {
		t.Fatal(err)
	}
	e := Analyze(ix, []linecov.FileDiff{{Path: "lib/a.rb", OldOID: "oid", NewOID: "edit", Hunks: []linecov.Hunk{{Start: 3, End: 3}}}})
	if e.Fallback == "" {
		t.Fatal("uncalled method edit excluded all tests")
	}
}

func TestLiveRSpecIsolation(t *testing.T) {
	requireRuby(t, "rspec-core")
	root := t.TempDir()
	head := project(t, root, "rspec")
	run, ix := collect(t, root, head, Options{Argv: []string{"ruby", "-e", `require "rspec/core"; exit RSpec::Core::Runner.run(ARGV)`, "spec"}})
	if run.ExitCode() != 1 || len(run.Outcomes) != 5 || strings.Join(ix.Unordered, ",") != "spec/calc_spec.rb" {
		t.Fatalf("outcomes %+v unordered %v", run.Outcomes, ix.Unordered)
	}
	checkCalc(t, ix, sorted("./spec/calc_spec.rb[1:1]", "./spec/other_spec.rb[1:1]"), "./spec/calc_spec.rb[1:2]", "./spec/calc_spec.rb[1:3]")
}

func TestDataDrivenTestUnit(t *testing.T) {
	requireRuby(t, "test-unit")
	root := laneProject(t, "require 'test/unit'\nclass LaneTest < Test::Unit::TestCase\n data('one'=>1, 'two'=>2)\n def test_value(value); assert(value > 0); end\nend\n")
	run, err := Collect(context.Background(), Options{Root: root, Work: t.TempDir(), Argv: []string{"ruby", "test/case_test.rb"}, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Outcomes) != 2 || run.Outcomes[0].Name == run.Outcomes[1].Name {
		t.Fatalf("identities %+v", run.Outcomes)
	}
	head := git(t, root, "rev-parse", "HEAD")
	oids, err := linecov.OIDs(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Index(".", head, run, oids); err != nil {
		t.Fatal(err)
	}
	for _, o := range run.Outcomes {
		if !o.Recorded {
			t.Fatalf("data test not isolated %+v", o)
		}
	}
}

func TestLoadedEvalAndNewTestFallback(t *testing.T) {
	requireRuby(t, "minitest")
	root := laneProject(t, "require 'minitest/autorun'\nrequire_relative '../lib'\nclass LaneTest < Minitest::Test\n def test_load; load File.expand_path('../lib.rb',__dir__); assert_equal 1, Lib.value; end\n def test_eval; eval('Lib.value'); assert true; end\nend\n")
	os.WriteFile(filepath.Join(root, "lib.rb"), []byte("class Lib\n def self.value\n  1\n end\nend\n"), 0600)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "lib")
	head := git(t, root, "rev-parse", "HEAD")
	run, ix := collect(t, root, head, Options{Argv: []string{"ruby", "test/case_test.rb"}})
	if run.ExitCode() != 0 {
		t.Fatalf("outcomes %+v", run.Outcomes)
	}
	effect := Analyze(ix, edit(ix, "lib.rb", 3, false))
	if effect.Fallback == "" {
		t.Fatal("load/eval code narrowed")
	}
	// An added head test in an existing file keeps the complete current file,
	// even though the index has no record of its new name.
	e := Analyze(ix, []linecov.FileDiff{{Path: "test/case_test.rb", OldOID: "base", NewOID: "head", Hunks: []linecov.Hunk{{Start: 4, End: 3}}}})
	p, _ := Plan(ix, []string{"test/case_test.rb"}, linecov.Effect{}, e)
	if p["test/case_test.rb"].Mode != linecov.Whole {
		t.Fatal("new head test omitted")
	}
}
