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
