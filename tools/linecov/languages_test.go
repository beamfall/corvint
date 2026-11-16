// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// LPCV-V0-7604/7606: both commands produce the common validated index and
// report the same analyzer's candidate decisions while qualification is pending.
func TestLanguageCommands(t *testing.T) {
	for _, lang := range []string{"ruby", "js"} {
		t.Run(lang, func(t *testing.T) {
			exeName := "ruby"
			if lang == "js" {
				exeName = "node"
			}
			exe, err := exec.LookPath(exeName)
			if err != nil {
				t.Skip(exeName + " unavailable")
			}
			exe, _ = filepath.Abs(exe)
			root := t.TempDir()
			file := "test/a.test.js"
			source := "lib.js"
			body := "exports.a = function a() {\n return 1;\n};\nexports.b = function b() {\n return 2;\n};\n"
			test := `const {test} = require('node:test'); const assert=require('node:assert'); const lib=require('../lib.js');
test('a',()=>assert.equal(lib.a(),1)); test('b',()=>assert.equal(lib.b(),2));`
			if lang == "ruby" {
				if err := exec.Command(exe, "-e", "gem 'minitest'").Run(); err != nil {
					t.Skip("minitest unavailable")
				}
				file = "test/a_test.rb"
				source = "lib.rb"
				body = "class Lib\n def self.a\n  1\n end\n def self.b\n  2\n end\nend\n"
				test = "require 'minitest/autorun'\nrequire_relative '../lib'\nclass LibTest < Minitest::Test\n def test_a; assert_equal 1, Lib.a; end\n def test_b; assert_equal 2, Lib.b; end\nend\n"
			}
			write(t, root, source, body)
			write(t, root, file, test)
			run(t, root, "git", "init", "-q")
			run(t, root, "git", "add", ".")
			run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "base")
			base, err := output(root, "git", "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			index := filepath.Join(t.TempDir(), "idx")
			args := []string{"-root", root, "-out", index}
			if lang == "ruby" {
				argv, _ := json.Marshal([]string{exe, file})
				args = append(args, "-command", string(argv))
			} else {
				args = append(args, "-runner", "node-test", "-executable", exe, "-files", file)
			}
			if err = languageCollect(lang, args); err != nil {
				t.Fatal(err)
			}
			ix, err := linecov.Load(index)
			if err != nil {
				t.Fatal(err)
			}
			if ix.Language != lang || len(ix.Tests) < 2 {
				t.Fatalf("index %+v", ix)
			}
			write(t, root, source, strings.Replace(body, "return 1", "return 3", 1))
			if lang == "ruby" {
				write(t, root, source, strings.Replace(body, "  1", "  3", 1))
			}
			run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qam", "edit")
			p, err := languageDecide(context.Background(), root, ix, lang, base, []string{file}, selectOptions{ruby: exe})
			if err != nil {
				t.Fatal(err)
			}
			if p.CoverageSelected != 1 || p.Excluded != 1 || p.StaticFallback != 0 || p.Candidate[file].Mode != linecov.Tests || p.Effective[file].Mode != linecov.Whole || p.GateEligible {
				t.Fatalf("plan %+v", p)
			}
		})
	}
}

// LPCV-V0-7605/7608: a Ruby plan executes its line-level candidate only when
// the family is admitted; an unexecuted-line edit then selects and runs no
// test, a load-time edit stays whole, and every refusal keeps the static plan
// with coverage-collector-unqualified and the refusal reported.
func TestLanguageDecideRubyAdmission(t *testing.T) {
	exe, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby unavailable")
	}
	exe, _ = filepath.Abs(exe)
	if err := exec.Command(exe, "-e", "gem 'minitest'").Run(); err != nil {
		t.Skip("minitest unavailable")
	}
	root := t.TempDir()
	file := "test/a_test.rb"
	body := "class Lib\n  def self.a\n    1\n  end\n\n  def self.unused\n    :never\n  end\nend\n"
	write(t, root, "lib.rb", body)
	write(t, root, file, "require 'minitest/autorun'\nrequire_relative '../lib'\nclass LibTest < Minitest::Test\n def test_a; assert_equal 1, Lib.a; end\n def test_b; assert true; end\nend\n")
	commit := func(msg string) string {
		run(t, root, "git", "add", ".")
		run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", msg)
		head, err := output(root, "git", "rev-parse", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		return head
	}
	run(t, root, "git", "init", "-q")
	base := commit("base")
	index := filepath.Join(t.TempDir(), "idx")
	argv, _ := json.Marshal([]string{exe, file})
	if err := languageCollect("ruby", []string{"-root", root, "-out", index, "-command", string(argv)}); err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Load(index)
	if err != nil {
		t.Fatal(err)
	}
	saved := rubyAdmit
	t.Cleanup(func() { rubyAdmit = saved })
	decide := func(edited string, admit error) LanguagePlan {
		t.Helper()
		run(t, root, "git", "checkout", "-q", "--detach", base)
		write(t, root, "lib.rb", edited)
		commit("edit")
		rubyAdmit = func(context.Context, string, string) error { return admit }
		p, err := languageDecide(context.Background(), root, ix, "ruby", base, []string{file}, selectOptions{ruby: exe})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	refused := errors.New("ruby qualification record unavailable")
	for _, tc := range []struct {
		name, edited string
		admit        error
		effective    string
		reason       string
		gate         bool
	}{
		{"covered line admitted", strings.Replace(body, "    1", "    3", 1), nil, "tests:LibTest#test_a", "coverage-qualified", true},
		{"covered line refused", strings.Replace(body, "    1", "    3", 1), refused, "whole", "coverage-collector-unqualified", false},
		{"unexecuted line admitted", strings.Replace(body, ":never", ":other", 1), nil, "exclude", "coverage-qualified", true},
		{"unexecuted line refused", strings.Replace(body, ":never", ":other", 1), refused, "whole", "coverage-collector-unqualified", false},
		{"load-time line admitted", strings.Replace(body, "class Lib", "class Lib # x\n  X = 1", 1), nil, "whole", "coverage-qualified", true},
		{"unparsable admitted", strings.Replace(body, ":never", ":never +", 1), nil, "whole", "coverage-qualified", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := decide(tc.edited, tc.admit)
			e := p.Effective[file]
			got := string(e.Mode)
			if e.Mode == linecov.Tests {
				got += ":" + strings.Join(e.Tests, ",")
			}
			if got != tc.effective || p.Reason != tc.reason || p.GateEligible != tc.gate || p.Runner != ix.Runner {
				t.Fatalf("effective %s reason %q gate %v admission %q fallback %q", got, p.Reason, p.GateEligible, p.Admission, p.Fallback)
			}
			if tc.admit != nil && p.Admission != tc.admit.Error() {
				t.Fatalf("refusal not reported: %q", p.Admission)
			}
			if tc.effective == "whole" && tc.admit == nil && p.Fallback == "" {
				t.Fatal("fallback reason not reported")
			}
		})
	}
}

// LPCV-V0-7607: saves replan dirty contents against the recorded commit and
// stream both failures and repairs as user-only observations.
func TestWatchWorkingTree(t *testing.T) {
	root := module(t)
	write(t, root, "p/p.go", "package p\nfunc F() int {\n return 1\n}\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "body")
	base, _ := output(root, "git", "rev-parse", "HEAD")
	oids, err := linecov.OIDs(context.Background(), root, base)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := runner(root, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	// A hand-built index needs an in-order record, else its package is
	// unordered and runs whole (GLS-V0-008), and records observed under an
	// environment, else it binds none (GLS-V0-017); this one reached no block.
	ix, err := linecov.Build("example.com/m", base, id, oids, []linecov.Record{{Package: "p", Name: "TestPass", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 1\n")}, {Package: "p", Name: "TestFail", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")}, {Package: "p", Name: linecov.InOrder, Union: true, EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")}})
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "p/p.go", "package p\nfunc F() int {\n return 2\n}\n")
	saves := make(chan struct{}, 2)
	saves <- struct{}{}
	saves <- struct{}{}
	close(saves)
	var stream bytes.Buffer
	calls := 0
	err = watchSaves(context.Background(), root, ix, []string{"p"}, saves, &stream, func(_ context.Context, p Plan) error {
		calls++
		if len(p.Run) != 1 || p.Run[0].Run != "^(TestPass)$" {
			t.Fatalf("plan %+v", p)
		}
		if calls == 1 {
			write(t, root, "p/p.go", "package p\nfunc F() int {\n return 3\n}\n")
			return os.ErrInvalid
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls %d", calls)
	}
	for _, l := range strings.Split(strings.TrimSpace(stream.String()), "\n") {
		var e WatchEvent
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatal(err)
		}
		if e.Scope != "user" || e.GateEligible || e.Commit != base {
			t.Fatalf("unsafe event %+v", e)
		}
	}
	if !strings.Contains(stream.String(), `"ExitCode":1`) {
		t.Fatal("test failure was not streamed")
	}
}

func TestWatchIgnoredGoSource(t *testing.T) {
	root := module(t)
	write(t, root, ".gitignore", "p/hidden.go\n")
	run(t, root, "git", "add", ".gitignore")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "ignore")
	base, _ := output(root, "git", "rev-parse", "HEAD")
	write(t, root, "p/hidden.go", "package p\nfunc init(){panic(\"changed\")}\n")
	diffs, err := linecov.WorkingDiff(context.Background(), root, base)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range diffs {
		if d.Path == "p/hidden.go" && d.OldOID == "" && d.NewOID != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ignored compiled input missed: %+v", diffs)
	}
}

func TestCommandTreeSnapshot(t *testing.T) {
	var b bytes.Buffer
	if err := help(&b, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("command-tree.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, b.Bytes()) {
		t.Fatal("command tree drift: run make capability-catalog")
	}
}
