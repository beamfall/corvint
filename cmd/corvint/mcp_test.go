package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// AHI-373: `corvint mcp` hands every later argument to the server before any other parser reads
// it, and serves under the process's one CORVINT_CPUPROFILE bracket instead of opening a second.
func TestAHI373MCPVerbOwnsItsArgumentsAndProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"mcp", "--corpus=data.json"}, strings.NewReader(""), &stdout, &stderr); code != 2 ||
		stderr.String() != "corvint-mcp: invalid arguments\n" {
		t.Fatalf("mcp --corpus=data.json: exit %d stderr %q; the server, not the corpus parser, must refuse it", code, &stderr)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// A started server pins Git for its whole process (V1-0191), which would
	// hide every later countingGit shim in this test binary, so the profiled
	// server runs in a child of this test binary.
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "cpu.out")
	child := exec.Command(binary, "-test.run=^TestAHI373MCPProfiledHelper$", "--", "--ahi373-mcp-helper", root, profile)
	child.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}` + "\n")
	stdout.Reset()
	stderr.Reset()
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil || !strings.Contains(stdout.String(), `"protocolVersion":"2025-11-25"`) {
		t.Fatalf("profiled mcp: %v stdout %q stderr %q", err, &stdout, &stderr)
	}
}

// TestAHI373MCPProfiledHelper is the child of TestAHI373MCPVerbOwnsItsArgumentsAndProfile, selected
// by argv: it serves `corvint mcp --root ROOT --protocol-version 2025-11-25` from stdin under a
// CORVINT_CPUPROFILE lookup naming PROFILE and exits with the command's code.
func TestAHI373MCPProfiledHelper(t *testing.T) {
	index := slices.Index(os.Args, "--ahi373-mcp-helper")
	if index < 0 || len(os.Args) != index+3 {
		return
	}
	root, profile := os.Args[index+1], os.Args[index+2]
	lookup := func(name string) (string, bool) {
		if name == "CORVINT_CPUPROFILE" {
			return profile, true
		}
		return "", false
	}
	ctx := context.Background()
	os.Exit(runProfiledContext(ctx, ctx, []string{"mcp", "--root", root, "--protocol-version", "2025-11-25"}, os.Stdin, os.Stdout, os.Stderr, lookup))
}
