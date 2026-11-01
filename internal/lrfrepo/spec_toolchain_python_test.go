//go:build !windows

package lrfrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSTAV0965_ConformancePicksAPython311 runs the conformance script's
// offline --print-python probe with stub interpreters as the whole PATH: an
// old python3 is skipped for python3.12, SPEC_TOOLCHAIN_PYTHON wins, an old
// override and a host without a qualifying interpreter are refused naming
// the override variable (STA-V0-965, V1-3657).
func TestSTAV0965_ConformancePicksAPython311(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "script", "spec-toolchain-conformance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	dirname, err := exec.LookPath("dirname")
	if err != nil {
		t.Skip("no dirname")
	}
	bin := t.TempDir()
	if err := os.Symlink(dirname, filepath.Join(bin, "dirname")); err != nil {
		t.Fatal(err)
	}
	stub := func(name string, ok bool) string {
		code := "1"
		if ok {
			code = "0"
		}
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit "+code+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	run := func(env ...string) (string, string, int) {
		cmd := exec.Command(bash, script, "--print-python")
		cmd.Env = append([]string{"PATH=" + bin}, env...)
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), code
	}
	old := stub("python3", false)
	if _, stderr, code := run(); code != 1 || !strings.Contains(stderr, "python3 python3.14 python3.13 python3.12 python3.11") || !strings.Contains(stderr, "SPEC_TOOLCHAIN_PYTHON=") {
		t.Fatalf("no qualifying python: exit %d stderr %q", code, stderr)
	}
	stub("python3.12", true)
	if stdout, stderr, code := run(); code != 0 || stdout != "python3.12\n" {
		t.Fatalf("selection: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	chosen := stub("custom-python", true)
	if stdout, _, code := run("SPEC_TOOLCHAIN_PYTHON=" + chosen); code != 0 || stdout != chosen+"\n" {
		t.Fatalf("override: exit %d stdout %q", code, stdout)
	}
	if _, stderr, code := run("SPEC_TOOLCHAIN_PYTHON=" + old); code != 1 || !strings.Contains(stderr, old) || !strings.Contains(stderr, "SPEC_TOOLCHAIN_PYTHON=") {
		t.Fatalf("old override: exit %d stderr %q", code, stderr)
	}
}
