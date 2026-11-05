// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
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
			p, err := languageDecide(context.Background(), root, ix, lang, base, []string{file})
			if err != nil {
				t.Fatal(err)
			}
			if p.CoverageSelected != 1 || p.Excluded != 1 || p.StaticFallback != 0 || p.Candidate[file].Mode != linecov.Tests || p.Effective[file].Mode != linecov.Whole || p.GateEligible {
				t.Fatalf("plan %+v", p)
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
	id, _, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/m", base, id, oids, []linecov.Record{{Package: "p", Name: "TestPass", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 1\n")}, {Package: "p", Name: "TestFail", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")}})
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
