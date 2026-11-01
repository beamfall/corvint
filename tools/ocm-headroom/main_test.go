package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, dir, name string, ids int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("# Spec\n\n## Requirements\n\n")
	for i := 1; i <= ids; i++ {
		fmt.Fprintf(&b, "- `HDR-V0-%03d`: requirement %d.\n", i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ASD-V0-572: a spec at or above the threshold is named with its count; one below it is not; the
// check never fails on a count, and a bad threshold is a usage error.
func TestASDV0572WarnsNearTheOCMObligationBound(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "near.md", 5)
	writeSpec(t, dir, "far.md", 2)
	var stdout, stderr bytes.Buffer
	if rc := run([]string{"--dir", dir, "--warn-at", "5"}, "", &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	want := "ocm-headroom: warning: " + filepath.ToSlash(filepath.Join(dir, "near.md")) + " holds 5 of 256 OCM obligations (warn at 5)"
	if got := stdout.String(); !strings.Contains(got, want) || strings.Contains(got, "far.md") || strings.Count(got, "\n") != 1 {
		t.Fatalf("warn-at 5 output %q, want only %q", got, want)
	}
	stdout.Reset()
	if rc := run([]string{"--dir", dir}, "6", &stdout, &stderr); rc != 0 || stdout.Len() != 0 {
		t.Fatalf("env warn-at 6: rc=%d output %q, want no warning", rc, stdout.String())
	}
	if rc := run([]string{"--dir", dir}, "x", &stdout, &stderr); rc != 2 {
		t.Fatalf("bad env threshold rc=%d, want 2", rc)
	}
	if rc := run([]string{"--dir", dir, "--warn-at", "0"}, "", &stdout, &stderr); rc != 2 {
		t.Fatalf("zero threshold rc=%d, want 2", rc)
	}
}
