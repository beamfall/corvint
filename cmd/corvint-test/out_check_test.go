package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/changee2e"
)

// CCE-V0-137: prove refuses an absent, non-directory or unwritable --out before any run
// starts, and a node-bin directory is refused with a message that names the documented form.
func TestCCEV0137ProveOutCheckedBeforeRun(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	t.Setenv("CORVINT_E2E_NODE_BIN", "")
	parent := t.TempDir()
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ro := filepath.Join(parent, "ro")
	if err := os.Mkdir(ro, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []string{filepath.Join(parent, "absent"), file}
	if os.Geteuid() != 0 {
		if err := os.Chmod(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(ro, 0o755) })
		cases = append(cases, ro)
	}
	for _, out := range cases {
		for _, extra := range [][]string{{"--hosted-test", "e2e/specs/teesheet-click.spec.ts"}, {}} {
			args := append([]string{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", out}, extra...)
			var r *changee2e.Refusal
			// The toolchain is unset, so a run that started would refuse e2e-toolchain-unavailable instead.
			if err := run(ctx, args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-out-unsafe" {
				t.Fatalf("%s %v: %v, want e2e-out-unsafe before any run", out, extra, err)
			}
		}
	}
	if entries, _ := os.ReadDir(ro); len(entries) != 0 {
		t.Fatalf("the probe left files behind: %v", entries)
	}
	// A writable existing directory passes the check and reaches the toolchain refusal.
	dir := t.TempDir()
	var r *changee2e.Refusal
	args := []string{"e2e", "prove", "--root", root, "--base", base, "--head", head, "--out", dir, "--hosted-test", "e2e/specs/teesheet-click.spec.ts"}
	if err := run(ctx, args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-toolchain-unavailable" {
		t.Fatalf("writable out: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("the probe left files behind: %v", entries)
	}
	// V1-2639: --node-bin is the node file; its directory is refused naming that form.
	args = append(args, "--node-bin", t.TempDir())
	if err := run(ctx, args, &bytes.Buffer{}); !errors.As(err, &r) || r.Code != "e2e-toolchain-unavailable" || !strings.Contains(r.Detail, "not its directory") {
		t.Fatalf("node-bin directory: %v", err)
	}
}
