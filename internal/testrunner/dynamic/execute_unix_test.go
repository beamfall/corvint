//go:build darwin || linux

package dynamic

import (
	"context"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNodeBuildExecuteParse(t *testing.T) {
	exe, e := exec.LookPath("node")
	if e != nil {
		t.Skip("Node unavailable")
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	source := []byte(`import test from 'node:test';test('pass',()=>{});test('fail',()=>{throw Error('deliberate')});test.skip('skip',()=>{});`)
	if e = os.WriteFile(filepath.Join(root, "a.test.mjs"), source, 0600); e != nil {
		t.Fatal(e)
	}
	req := tr.Request{Runner: "node-test", Root: root, Executable: exe, ExecutableSha256: tr.Digest(tool), InputFiles: map[string]string{"a.test.mjs": tr.Digest(source)}, Selectors: []string{"a.test.mjs"}, ReportDir: filepath.Join(root, "reports"), TimeoutSeconds: 20}
	inv, e := Build(req)
	if e != nil {
		t.Fatal(e)
	}
	result, e := tr.Execute(context.Background(), req, inv)
	if e != nil {
		t.Fatal(e)
	}
	o, e := Parse(result.Input)
	o = tr.Normalize(result.Input, o)
	if e != nil || !o.Complete || len(o.Tests) != 3 || result.Input.ExitCode != 1 {
		t.Fatalf("%+v %v stderr=%s", o, e, result.Input.Stderr)
	}
}

// Opt-in dependency admission lets qualification retain a real pinned local
// Mocha without requiring package installation as part of the repository test.
func TestMochaBuildExecuteParse(t *testing.T) {
	exe := os.Getenv("CORVINT_TEST_MOCHA")
	if exe == "" {
		t.Skip("explicit installed Mocha not admitted")
	}
	exe, e := filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal(e)
	}
	node, e = filepath.EvalSymlinks(node)
	if e != nil {
		t.Fatal(e)
	}
	nodeBytes, e := os.ReadFile(node)
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	source := []byte(`const assert=require('node:assert/strict');describe('fixture',function(){it('pass',()=>{});it('fail',()=>assert.equal(1,2));it.skip('skip',()=>{});it('retry',function(){this.retries(1);assert.equal(this.test.currentRetry(),1)});});`)
	if e = os.WriteFile(filepath.Join(root, "sample.cjs"), source, 0600); e != nil {
		t.Fatal(e)
	}
	req := tr.Request{Runner: "mocha", Root: root, Executable: exe, ExecutableSha256: tr.Digest(tool), Tools: map[string]tr.Tool{"node": {Executable: node, Sha256: tr.Digest(nodeBytes)}}, InputFiles: map[string]string{"sample.cjs": tr.Digest(source)}, Selectors: []string{"sample.cjs"}, ReportDir: filepath.Join(root, "reports"), TimeoutSeconds: 30}
	inv, e := Build(req)
	if e != nil {
		t.Fatal(e)
	}
	result, e := tr.Execute(context.Background(), req, inv)
	if e != nil {
		t.Fatal(e)
	}
	o, e := Parse(result.Input)
	o = tr.Normalize(result.Input, o)
	if e != nil || !o.Complete || len(o.Tests) != 4 || result.Input.ExitCode != 1 || o.RetryInformation != tr.Retained {
		t.Fatalf("%+v %v stderr=%s", o, e, result.Input.Stderr)
	}
	counts := map[string]int{}
	for _, x := range o.Tests {
		counts[x.State]++
	}
	if counts[tr.Passed] != 1 || counts[tr.Failed] != 1 || counts[tr.Skipped] != 1 || counts[tr.Flaky] != 1 {
		t.Fatal(counts)
	}
}

// TestJasmineBuildExecuteParse runs an explicitly admitted pinned Jasmine
// (CORVINT_TEST_JASMINE names its bin/jasmine.js) through the common executor:
// TRE-V0-031, TRE-V0-032 and TRE-V0-033.
func TestJasmineBuildExecuteParse(t *testing.T) {
	exe := os.Getenv("CORVINT_TEST_JASMINE")
	if exe == "" {
		t.Skip("explicit installed Jasmine not admitted")
	}
	exe, e := filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	tool, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal(e)
	}
	node, e = filepath.EvalSymlinks(node)
	if e != nil {
		t.Fatal(e)
	}
	nodeBytes, e := os.ReadFile(node)
	if e != nil {
		t.Fatal(e)
	}
	run := func(t *testing.T, sources map[string]string, selectors []string) (tr.Observation, int, error) {
		root, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		inputs := map[string]string{}
		for name, body := range sources {
			if e = os.WriteFile(filepath.Join(root, name), []byte(body), 0600); e != nil {
				t.Fatal(e)
			}
			inputs[name] = tr.Digest([]byte(body))
		}
		req := tr.Request{Runner: "jasmine", Root: root, Executable: exe, ExecutableSha256: tr.Digest(tool), Tools: map[string]tr.Tool{"node": {Executable: node, Sha256: tr.Digest(nodeBytes)}}, InputFiles: inputs, Selectors: selectors, ReportDir: filepath.Join(root, "reports"), TimeoutSeconds: 60}
		inv, e := Build(req)
		if e != nil {
			t.Fatal(e)
		}
		result, e := tr.Execute(context.Background(), req, inv)
		if e != nil {
			// A declared report the runner never wrote refuses the whole run.
			return tr.Observation{}, result.Input.ExitCode, e
		}
		o, e := Parse(result.Input)
		if e != nil {
			t.Fatalf("%v stderr=%s", e, result.Input.Stderr)
		}
		return tr.Normalize(result.Input, o), result.Input.ExitCode, nil
	}
	t.Run("mixed", func(t *testing.T) {
		source := `describe('fixture', function () {
  it('passes', function () { expect(1 + 1).toBe(2); });
  it('fails deliberately', function () { expect(1).toBe(2); });
  xit('is skipped with xit', function () {});
  it('is pending', function () { pending('fixture'); });
  describe('nested', function () { it('passes too', function () { expect(true).toBeTrue(); }); });
});
`
		o, code, e := run(t, map[string]string{"fixture.spec.js": source}, []string{"fixture.spec.js"})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		ids := map[string]string{}
		for _, x := range o.Tests {
			counts[x.State]++
			ids[x.ID] = x.State
		}
		if code != 3 || !o.Complete || len(o.Tests) != 5 || counts[tr.Passed] != 2 || counts[tr.Failed] != 1 || counts[tr.Skipped] != 2 || o.RetryInformation != tr.NotApplicable {
			t.Fatalf("exit=%d counts=%v %+v", code, counts, o)
		}
		if ids["fixture.spec.js::fixture fails deliberately"] != tr.Failed || ids["fixture.spec.js::fixture nested passes too"] != tr.Passed || ids["fixture.spec.js::fixture is skipped with xit"] != tr.Skipped {
			t.Fatal(ids)
		}
	})
	t.Run("missing selector", func(t *testing.T) {
		o, code, e := run(t, map[string]string{"ok.spec.js": `describe('ok', function () { it('passes', function () { expect(1).toBe(1); }); });`}, []string{"ok.spec.js", "missing.spec.js"})
		if e != nil || code != 0 || o.Complete || !problemCodes(o)["jasmine-selector-without-specs"] {
			t.Fatalf("exit=%d %+v", code, o)
		}
	})
	t.Run("load error", func(t *testing.T) {
		o, code, e := run(t, map[string]string{"bad.spec.js": `describe('bad', function () { it('x', function () {}); }); throw new Error('load');`}, []string{"bad.spec.js"})
		if e == nil || code != 1 || o.Complete || len(o.Tests) != 0 {
			t.Fatalf("exit=%d err=%v %+v", code, e, o)
		}
	})
}
