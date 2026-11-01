package jstestprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// CCE-V0-140 (V1-2644): the receipt binds the version of the node executable that runs the runner
// when argv[0] is an absolute node path, not the node found on PATH.
func TestNodeVersionBindsRunnerExecutable(t *testing.T) {
	dir := t.TempDir()
	fake := func(sub, version string) string {
		bin := filepath.Join(dir, sub)
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(bin, "node")
		if err := execfile.WriteFile(path, []byte("#!/bin/sh\necho "+version+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	onPath, runner := fake("path", "v22.23.3"), fake("runner", "v24.11.1")
	t.Setenv("PATH", filepath.Dir(onPath))
	if got := nodeVersion(); got != "v22.23.3" {
		t.Fatalf("PATH node = %q", got)
	}
	t.Setenv("npm_node_execpath", "")
	if got := nodeVersionFor(context.Background(), []string{runner, "cli.js"}); got != "v24.11.1" {
		t.Fatalf("runner node = %q, want the runner's v24.11.1", got)
	}
	// V1-2822: an npx launch never records the PATH node. Without npm_node_execpath it is unknown.
	for _, argv := range [][]string{nil, {"npx", "playwright"}, {"node", "cli.js"}, {filepath.Join(filepath.Dir(runner), "npx")}} {
		if got := nodeVersionFor(context.Background(), argv); got != "" {
			t.Fatalf("%v: %q, want unknown (not the PATH node)", argv, got)
		}
	}
	// With npm_node_execpath (set by npm for npx) the node that executes is bound.
	t.Setenv("npm_node_execpath", runner)
	if got := nodeVersionFor(context.Background(), []string{"npx", "--no-install", "playwright", "test"}); got != "v24.11.1" {
		t.Fatalf("npx with npm_node_execpath = %q, want v24.11.1", got)
	}
	t.Setenv("npm_node_execpath", "relative/node")
	if got := nodeVersionFor(context.Background(), []string{"npx"}); got != "" {
		t.Fatalf("relative npm_node_execpath = %q, want unknown", got)
	}
	t.Setenv("npm_node_execpath", "")
	config := filepath.Join(dir, "config.cjs")
	if err := os.WriteFile(config, []byte("module.exports={}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity, err := Config{ConfigFile: config}.identity(context.Background(), []string{runner, "cli.js"})
	if err != nil || identity.NodeVersion != "v24.11.1" {
		t.Fatalf("identity: %+v %v", identity, err)
	}
}

// V1-2822: the runner-reported process.version fills an unobserved npx identity only when every
// test reports the same one.
func TestRecordedNodeVersion(t *testing.T) {
	outcome := func(v string) TestOutcome {
		o := TestOutcome{Project: &ProjectIdentity{}}
		o.Project.Use = []byte(`{"corvintBrowser":{"nodeVersion":"` + v + `"}}`)
		return o
	}
	if got := recordedNodeVersion([]TestOutcome{outcome("v22.23.2"), outcome("v22.23.2")}); got != "v22.23.2" {
		t.Fatalf("agreeing = %q", got)
	}
	for name, tests := range map[string][]TestOutcome{
		"disagree": {outcome("v22.23.2"), outcome("v24.11.1")},
		"missing":  {outcome("v22.23.2"), {}},
		"none":     nil,
	} {
		if got := recordedNodeVersion(tests); got != "" {
			t.Fatalf("%s = %q, want unknown", name, got)
		}
	}
}
