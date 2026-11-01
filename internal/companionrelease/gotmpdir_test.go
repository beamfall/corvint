package companionrelease

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPUBV0965ClosedGoEnvPinsItsOwnGOTMPDIR (V1-3799): the closed build
// environment names GOTMPDIR under the build's own home and nothing of the
// caller's TMPDIR or GOTMPDIR, the runner creates that directory, and the go
// command it starts reports it as its GOTMPDIR, so a full host /tmp cannot
// fail the double build's $WORK.
func TestPUBV0965ClosedGoEnvPinsItsOwnGOTMPDIR(t *testing.T) {
	caller := filepath.Join(t.TempDir(), "caller-tmp")
	t.Setenv("TMPDIR", caller)
	t.Setenv("GOTMPDIR", caller)
	home := filepath.Join(t.TempDir(), "home")
	env := closedGoEnv(home, "")
	var gotmp []string
	for _, entry := range env {
		if strings.Contains(entry, caller) || strings.HasPrefix(entry, "TMPDIR=") {
			t.Errorf("caller temp directory leaked into the closed env: %q", entry)
		}
		if dir, ok := strings.CutPrefix(entry, "GOTMPDIR="); ok {
			gotmp = append(gotmp, dir)
		}
	}
	want := filepath.Join(home, "gotmp")
	if len(gotmp) != 1 || gotmp[0] != home+"/gotmp" {
		t.Fatalf("closed env GOTMPDIR = %q, want exactly %q", gotmp, home+"/gotmp")
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool unavailable")
	}
	if err = os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runCaptured(context.Background(), home, env, subprocessTimeout, goPath, "env", "GOTMPDIR")
	if err != nil {
		t.Fatalf("go env GOTMPDIR: %v (%s)", err, stderr)
	}
	if got := filepath.Clean(strings.TrimSpace(string(out))); got != want {
		t.Fatalf("go env GOTMPDIR = %q, want %q", got, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("runner did not create GOTMPDIR %s: %v", want, err)
	}
	if _, err := os.Stat(caller); err == nil {
		t.Fatalf("the caller's temp directory %s was created", caller)
	}
}
