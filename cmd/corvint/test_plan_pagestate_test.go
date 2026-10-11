package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ANL-V0-014: page-state is read-only; valid input prints the report and invalid use exits 2.
func TestANLV0014PageStateCLI(t *testing.T) {
	dir := t.TempDir()
	row := func(id string) string {
		return `{"variation_id":"` + id + `","spec":"a.spec.ts","app":"admin","setup":"s.ts","screen":"screen:x","user":"u","org":"o","action":["element:a"],"assertion":["element:` + id + `"],"requires":[],"changes":[],"destructive":false}`
	}
	file := filepath.Join(dir, "in.json")
	if err := os.WriteFile(file, []byte(`{"schema":"test-consolidation-input/0","variations":[`+row("v1")+","+row("v2")+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		var so, se bytes.Buffer
		c := runTestPlan(context.Background(), dir, append([]string{"page-state"}, args...), &so, &se)
		return c, so.String()
	}
	c, out := run("--input", file)
	if c != 0 || !strings.Contains(out, `"visits_saved":1`) || !strings.Contains(out, "test-page-state-groups/0") {
		t.Fatalf("%d %s", c, out)
	}
	if c, _ = run(); c != 2 {
		t.Fatalf("missing input exit %d", c)
	}
	if c, _ = run("--input", filepath.Join(dir, "none.json")); c != 2 {
		t.Fatalf("unreadable exit %d", c)
	}
}
