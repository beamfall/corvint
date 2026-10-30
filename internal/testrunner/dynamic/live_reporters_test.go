package dynamic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// These execute only test-owned source in a temporary directory. Optional runtimes
// are explicitly skipped; parsing fixtures alone does not qualify installation.
func TestProfileOwnedReportersLive(t *testing.T) {
	cases := []struct{ runner, executable, file, source string }{
		{"node-test", "node", "sample.test.mjs", `import test from 'node:test';import assert from 'node:assert/strict';test('pass',()=>assert.equal(1,1));test('fail',()=>assert.equal(1,2));test.skip('skip',()=>{});`},
		{"unittest", "python3", "test_sample.py", "import unittest\nclass Sample(unittest.TestCase):\n def test_pass(self): self.assertTrue(True)\n def test_fail(self): self.assertTrue(False)\n @unittest.skip('fixture')\n def test_skip(self): pass\n"},
		{"minitest", "ruby", "sample.rb", "require 'minitest/autorun'\nclass Sample < Minitest::Test\n def test_pass; assert true; end\n def test_fail; assert false; end\n def test_skip; skip 'fixture'; end\nend\n"},
		{"test-unit", "ruby", "sample.rb", "require 'test/unit'\nclass Sample < Test::Unit::TestCase\n def test_pass; assert true; end\n def test_fail; assert false; end\n def test_skip; omit 'fixture'; end\nend\n"},
	}
	for _, c := range cases {
		t.Run(c.runner, func(t *testing.T) {
			exe, e := exec.LookPath(c.executable)
			if e != nil {
				t.Skipf("runtime unavailable: %v", e)
			}
			if c.executable == "ruby" {
				library := "minitest"
				if c.runner == "test-unit" {
					library = "test/unit"
				}
				if e := exec.Command(exe, "-e", "require '"+library+"'").Run(); e != nil {
					t.Skipf("Ruby profile dependency unavailable: %v", e)
				}
			}
			dir := t.TempDir()
			art := filepath.Join(dir, "reports")
			if e = os.Mkdir(art, 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, c.file), []byte(c.source), 0600); e != nil {
				t.Fatal(e)
			}
			selector := c.file
			if c.runner == "unittest" {
				selector = "test_sample"
			}
			inv, e := Build(tr.Request{Runner: c.runner, Root: dir, Executable: exe, ReportDir: art, Selectors: []string{selector}})
			if e != nil {
				t.Fatal(e)
			}
			for name, body := range inv.Files {
				if e = os.WriteFile(filepath.Join(art, name), body, 0600); e != nil {
					t.Fatal(e)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, inv.Argv...)
			cmd.Dir = dir
			out, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			exit := 0
			if runErr != nil {
				ee, ok := runErr.(*exec.ExitError)
				if !ok {
					t.Fatal(runErr)
				}
				exit = ee.ExitCode()
			}
			if exit != 1 {
				t.Fatalf("expected assertion exit1 got%d: %s", exit, out)
			}
			in := tr.Input{Runner: c.runner, ExitCode: exit, Reports: map[string][]byte{}}
			for _, name := range inv.ReportPaths {
				b, e := os.ReadFile(filepath.Join(art, name))
				if e != nil {
					t.Fatalf("%v output=%s", e, out)
				}
				in.Reports[name] = b
			}
			obs, e := Parse(in)
			if e != nil || !obs.Complete || len(obs.Tests) != 3 {
				t.Fatalf("%+v %v output=%s", obs, e, out)
			}
		})
	}
}

// TRE-V0-110 (V1-1541): a Minitest test that raises is FAILED with cause
// UNKNOWN for that case only; the passing case keeps its outcome and the run
// stays complete.
func TestMinitestUnexpectedErrorFailsOnlyThatCaseLive(t *testing.T) {
	exe, e := exec.LookPath("ruby")
	if e != nil {
		t.Skipf("runtime unavailable: %v", e)
	}
	if e := exec.Command(exe, "-e", "require 'minitest'").Run(); e != nil {
		t.Skipf("Ruby profile dependency unavailable: %v", e)
	}
	dir := t.TempDir()
	art := filepath.Join(dir, "reports")
	if e = os.Mkdir(art, 0700); e != nil {
		t.Fatal(e)
	}
	src := "require 'minitest/autorun'\nclass Sample < Minitest::Test\n def test_pass; assert true; end\n def test_raise; raise ArgumentError, 'secret detail'; end\nend\n"
	if e = os.WriteFile(filepath.Join(dir, "sample.rb"), []byte(src), 0600); e != nil {
		t.Fatal(e)
	}
	inv, e := Build(tr.Request{Runner: "minitest", Root: dir, Executable: exe, ReportDir: art, Selectors: []string{"sample.rb"}})
	if e != nil {
		t.Fatal(e)
	}
	for name, body := range inv.Files {
		if e = os.WriteFile(filepath.Join(art, name), body, 0600); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, inv.Argv...)
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	ee, ok := runErr.(*exec.ExitError)
	if ctx.Err() != nil || !ok || ee.ExitCode() != 1 {
		t.Fatalf("want native exit 1, got %v: %s", runErr, out)
	}
	in := tr.Input{Runner: "minitest", ExitCode: 1, Reports: map[string][]byte{}}
	for _, name := range inv.ReportPaths {
		b, e := os.ReadFile(filepath.Join(art, name))
		if e != nil {
			t.Fatalf("%v output=%s", e, out)
		}
		in.Reports[name] = b
	}
	obs, e := Parse(in)
	if e != nil || !obs.Complete || len(obs.Tests) != 2 || len(obs.Problems) != 0 {
		t.Fatalf("%+v %v output=%s", obs, e, out)
	}
	got := map[string]tr.Test{}
	for _, c := range obs.Tests {
		got[c.ID] = c
	}
	if c := got["Sample#test_pass"]; c.State != tr.Passed {
		t.Fatalf("passing case: %+v", c)
	}
	c := got["Sample#test_raise"]
	if c.State != tr.Failed || c.FailureKind != tr.Unknown || c.Message != "UnexpectedError: ArgumentError" {
		t.Fatalf("raising case: %+v", c)
	}
}

// TRE-V0-115 (V1-2156): a unittest or test-unit test that raises is FAILED
// with cause UNKNOWN naming only the exception class; the passing case keeps
// its outcome and the run stays complete.
func TestUnittestAndTestUnitErrorFailsOnlyThatCaseLive(t *testing.T) {
	cases := []struct{ runner, executable, file, selector, source, raiseID, message string }{
		{"unittest", "python3", "test_sample.py", "test_sample", "import unittest\nclass Sample(unittest.TestCase):\n def test_pass(self): self.assertTrue(True)\n def test_raise(self): raise ValueError('secret detail')\n", "test_sample.Sample.test_raise", "Error: ValueError"},
		{"test-unit", "ruby", "sample.rb", "sample.rb", "require 'test/unit'\nclass Sample < Test::Unit::TestCase\n def test_pass; assert true; end\n def test_raise; raise ArgumentError, 'secret detail'; end\nend\n", "test_raise(Sample)", "Error: ArgumentError"},
	}
	for _, c := range cases {
		t.Run(c.runner, func(t *testing.T) {
			exe, e := exec.LookPath(c.executable)
			if e != nil {
				t.Skipf("runtime unavailable: %v", e)
			}
			if c.executable == "ruby" {
				if e := exec.Command(exe, "-e", "require 'test/unit'").Run(); e != nil {
					t.Skipf("Ruby profile dependency unavailable: %v", e)
				}
			}
			dir := t.TempDir()
			art := filepath.Join(dir, "reports")
			if e = os.Mkdir(art, 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, c.file), []byte(c.source), 0600); e != nil {
				t.Fatal(e)
			}
			inv, e := Build(tr.Request{Runner: c.runner, Root: dir, Executable: exe, ReportDir: art, Selectors: []string{c.selector}})
			if e != nil {
				t.Fatal(e)
			}
			for name, body := range inv.Files {
				if e = os.WriteFile(filepath.Join(art, name), body, 0600); e != nil {
					t.Fatal(e)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, inv.Argv...)
			cmd.Dir = dir
			out, runErr := cmd.CombinedOutput()
			ee, ok := runErr.(*exec.ExitError)
			if ctx.Err() != nil || !ok || ee.ExitCode() != 1 {
				t.Fatalf("want native exit 1, got %v: %s", runErr, out)
			}
			in := tr.Input{Runner: c.runner, ExitCode: 1, Reports: map[string][]byte{}}
			for _, name := range inv.ReportPaths {
				b, e := os.ReadFile(filepath.Join(art, name))
				if e != nil {
					t.Fatalf("%v output=%s", e, out)
				}
				in.Reports[name] = b
			}
			obs, e := Parse(in)
			if e != nil || !obs.Complete || len(obs.Tests) != 2 || len(obs.Problems) != 0 {
				t.Fatalf("%+v %v output=%s", obs, e, out)
			}
			for _, got := range obs.Tests {
				if got.ID == c.raiseID {
					if got.State != tr.Failed || got.FailureKind != tr.Unknown || got.Message != c.message {
						t.Fatalf("raising case: %+v", got)
					}
				} else if got.State != tr.Passed {
					t.Fatalf("passing case: %+v", got)
				}
			}
		})
	}
}
