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

// ANL-V0-033: --check --baseline exits 1 naming groups that appeared, vanished or changed and
// blocked variations that changed; an unchanged grouping passes, and a tampered or partial use
// exits 2.
func TestANLV0033PageStateDrift(t *testing.T) {
	dir := t.TempDir()
	row := func(id, screen string, destructive bool) string {
		d := "false"
		if destructive {
			d = "true"
		}
		return `{"variation_id":"` + id + `","spec":"a.spec.ts","app":"admin","setup":"s.ts","screen":"` + screen + `","user":"u","org":"o","action":["element:a"],"assertion":["element:` + id + `"],"requires":[],"changes":[],"destructive":` + d + `}`
	}
	write := func(name string, rows ...string) string {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(`{"schema":"test-consolidation-input/0","variations":[`+strings.Join(rows, ",")+`]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	run := func(args ...string) (int, string, string) {
		var so, se bytes.Buffer
		c := runTestPlan(context.Background(), dir, append([]string{"page-state"}, args...), &so, &se)
		return c, so.String(), se.String()
	}
	in := write("in.json", row("v1", "screen:x", false), row("v2", "screen:x", false), row("v3", "screen:y", true))
	c, report, _ := run("--input", in)
	if c != 0 {
		t.Fatal(c)
	}
	base := filepath.Join(dir, "base.json")
	if err := os.WriteFile(base, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, _, se := run("--input", in, "--check", "--baseline", base); c != 0 {
		t.Fatalf("unchanged grouping drifted: %d %s", c, se)
	}
	moved := write("moved.json", row("v1", "screen:x", false), row("v2", "screen:z", false), row("v4", "screen:z", false), row("v3", "screen:y", false))
	c, out, se := run("--input", moved, "--check", "--baseline", base)
	if c != 1 || !strings.Contains(se, "test-plan-mismatch") || !strings.Contains(se, "added 1") || !strings.Contains(se, "removed 1") || !strings.Contains(se, "blocked-changed 1 [v3]") || !strings.Contains(out, "test-page-state-groups/0") {
		t.Fatalf("drift not reported: %d %s", c, se)
	}
	assertion := write("assertion.json", row("v1", "screen:x", false), strings.Replace(row("v2", "screen:x", false), `"element:v2"`, `"element:other"`, 1), row("v3", "screen:y", true))
	if c, _, se = run("--input", assertion, "--check", "--baseline", base); c != 1 || !strings.Contains(se, "changed 1") {
		t.Fatalf("changed group not reported: %d %s", c, se)
	}
	tampered := filepath.Join(dir, "tampered.json")
	if err := os.WriteFile(tampered, []byte(strings.Replace(report, `"visits_saved":1`, `"visits_saved":2`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	trailing := filepath.Join(dir, "trailing.json")
	if err := os.WriteFile(trailing, []byte(strings.TrimRight(report, "\n")+"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--input", in, "--check", "--baseline", tampered}, {"--input", in, "--check", "--baseline", trailing}, {"--input", in, "--check"}, {"--input", in, "--baseline", base}, {"--input", in, "--check", "--baseline", in}} {
		if c, _, _ = run(args...); c != 2 {
			t.Fatalf("%v exit %d", args, c)
		}
	}
}
