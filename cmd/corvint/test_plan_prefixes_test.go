package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ANL-V0-011: the lint is read-only; --check passes with nothing flagged and invalid use exits 2.
func TestANLV0011PrefixesCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		var so, se bytes.Buffer
		c := runTestPlan(context.Background(), root, append([]string{"prefixes"}, args...), &so, &se)
		return c, so.String() + se.String()
	}
	code, got := run("--map", mapFile, "--check")
	if code != 0 || !strings.Contains(got, `"schema":"test-plan-prefixes/0"`) || !strings.Contains(got, `"flagged":[]`) {
		t.Fatal(code, got)
	}
	if code, _ = run("--check"); code != 2 {
		t.Fatal("missing --map accepted", code)
	}
	if code, _ = run("--map", mapFile, "--baseline", mapFile); code != 2 {
		t.Fatal("invalid baseline accepted", code)
	}
}
