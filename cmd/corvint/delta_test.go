package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeltaInternalCLIExplicitImmutableNoOp(t *testing.T) {
	t.Run("DLT-V0-010 read-only CLI no-op", testDeltaInternalCLIExplicitImmutableNoOp)
}
func testDeltaInternalCLIExplicitImmutableNoOp(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		raw, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	git("init", "-q")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("commit", "--allow-empty", "-qm", "base")
	head := git("rev-parse", "HEAD")
	var publicReference []byte
	for _, args := range [][]string{
		{"--root", root, "delta", "--base", head, "--head", head},
		{"--root=" + root, "delta", "--base=" + head, "--head=" + head},
	} {
		var output, stderr bytes.Buffer
		if code := runContext(context.Background(), args, strings.NewReader(""), &output, &stderr); code != 0 || stderr.Len() != 0 || !bytes.Contains(output.Bytes(), []byte(`"decision":"no-op"`)) {
			t.Fatalf("public argv %v code %d: %s %s", args, code, output.String(), stderr.String())
		}
		if publicReference != nil && !bytes.Equal(publicReference, output.Bytes()) {
			t.Fatal("public split and inline arguments differ")
		}
		publicReference = append([]byte(nil), output.Bytes()...)
	}
	for _, tail := range [][]string{
		{}, {"--base", "HEAD", "--head", head}, {"--base", strings.ToUpper(head), "--head", head},
		{"--unknown"}, {"--base", head, "--head", head, "extra"},
		{"--base", "--help"}, {"--provider", "-x"}, {"--checkout", "--help"},
	} {
		var output, stderr bytes.Buffer
		args := append([]string{"--root", root, "delta"}, tail...)
		if code := runContext(context.Background(), args, strings.NewReader(""), &output, &stderr); code != 2 || output.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("public refusal %v code %d: %s %s", args, code, output.String(), stderr.String())
		}
	}
	var out, diagnostic bytes.Buffer
	if code := runDelta(context.Background(), root, []string{"--base", head, "--head", head}, &out, &diagnostic); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"decision":"no-op"`)) {
		t.Fatalf("code %d: %s %s", code, out.String(), diagnostic.String())
	}
	out.Reset()
	if code := runDelta(context.Background(), root, []string{"--base", "HEAD", "--head", head}, &out, &diagnostic); code != 2 || out.Len() != 0 {
		t.Fatal("invalid revision admitted")
	}
}

func TestDeltaPublicHelp(t *testing.T) {
	for _, args := range [][]string{{"delta", "--help"}, {"help", "delta"}, {"--root", "/absent-delta-root", "delta", "-h"}, {"--help"}} {
		var out, stderr bytes.Buffer
		if code := runContext(context.Background(), args, strings.NewReader(""), &out, &stderr); code != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "delta") {
			t.Fatalf("help %v code %d: %s %s", args, code, out.String(), stderr.String())
		}
	}
}

func TestDeltaCLIPreservesClosedRefusalCodes(t *testing.T) {
	t.Run("DLT-V0-010 compiler refusal codes reach the CLI unchanged", testDeltaCLIPreservesClosedRefusalCodes)
}
func testDeltaCLIPreservesClosedRefusalCodes(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bare, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		raw, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	git("init", "-q")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("commit", "--allow-empty", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "tab\tfile"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-qm", "control")
	control := git("rev-parse", "HEAD")
	absent := strings.Repeat("0", len(base))
	for _, c := range []struct {
		root string
		args []string
		code string
	}{
		{root, []string{"--base", base, "--head", base, "--work-key-pattern", "("}, "delta-invalid-arguments"},
		{bare, []string{"--base", base, "--head", base}, "delta-repository-unavailable"},
		{root, []string{"--base", base, "--head", absent}, "delta-head-unavailable"},
		{root, []string{"--base", absent, "--head", base}, "delta-change-set-unavailable"},
		{root, []string{"--base", base, "--head", control}, "delta-unrepresentable-path"},
	} {
		var out, diagnostic bytes.Buffer
		if code := runDelta(context.Background(), c.root, c.args, &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.String() != c.code+"\n" {
			t.Errorf("%v: code %d stdout %q diagnostic %q, want %s", c.args, code, out.String(), diagnostic.String(), c.code)
		}
	}
}
