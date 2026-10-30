//go:build darwin

package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestProductionCapabilitySandboxSpy runs the production stdin/stdout path in
// a kernel-enforced capability sandbox. The child has no access to the caller
// repository or ambient HOME, and cannot start a process or open a socket.
func TestProductionCapabilitySandboxSpy(t *testing.T) {
	root := t.TempDir()
	worktree, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	decoy := filepath.Join(root, "repository-decoy")
	if err := os.WriteFile(decoy, []byte("caller-owned bytes only"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	raw := []byte(compiledCLIFrozenSuccessRequest)
	want := []byte(compiledCLIFrozenSuccessResponse)
	binary := buildCompiledCLI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	profile := `(version 1)
(allow default)
(deny network-inbound)
(deny network-outbound)
(deny process-fork)
(deny file-read-data (subpath "` + root + `"))
(deny file-read-data (subpath "` + realRoot + `"))
(deny file-read-data (subpath "` + worktree + `"))
(deny file-read-data (subpath "` + home + `"))`
	cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, binary)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Env = []string{
		"CORVINT_ANALYZER_PRODUCTION_SPY=1",
		"PATH=" + root,
		"HOME=" + root,
		"GIT_DIR=" + filepath.Join(root, ".git"),
		"ALL_PROXY=http://127.0.0.1:1",
		"HTTPS_PROXY=http://127.0.0.1:1",
	}
	got, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("production capability spy timed out: %v", ctx.Err())
	}
	if err != nil || string(got) != string(want) {
		t.Fatalf("production capability spy err=%v want=%q got=%q", err, want, got)
	}
	for _, probe := range []struct {
		name  string
		value string
	}{
		{"process", ""},
		{"network", listener.Addr().String()},
		{"repository", decoy},
		{"worktree", filepath.Join(worktree, "main.go")},
	} {
		probeCtx, probeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		probeCmd := exec.CommandContext(probeCtx, "/usr/bin/sandbox-exec", "-p", profile, os.Args[0], "-test.run=^TestProductionCapabilityPolicyHelper$")
		probeCmd.Dir = root
		probeCmd.Env = []string{
			"CORVINT_ANALYZER_CAPABILITY_PROBE=" + probe.name,
			"CORVINT_ANALYZER_CAPABILITY_VALUE=" + probe.value,
			"PATH=" + root,
			"HOME=" + root,
		}
		got, err = probeCmd.CombinedOutput()
		probeErr := probeCtx.Err()
		probeCancel()
		if probeErr != nil {
			t.Fatalf("%s capability probe timed out: %v", probe.name, probeErr)
		}
		if err != nil || !bytes.HasSuffix(got, []byte("PASS\n")) {
			t.Fatalf("%s capability probe escaped sandbox: err=%v output=%q", probe.name, err, got)
		}
	}
}

func TestProductionCapabilityPolicyHelper(t *testing.T) {
	switch os.Getenv("CORVINT_ANALYZER_CAPABILITY_PROBE") {
	case "":
		return
	case "process":
		if err := exec.Command("/usr/bin/true").Run(); err == nil {
			t.Fatal("process launch was allowed")
		}
	case "network":
		conn, err := net.DialTimeout("tcp", os.Getenv("CORVINT_ANALYZER_CAPABILITY_VALUE"), time.Second)
		if err == nil {
			conn.Close()
			t.Fatal("network connection was allowed")
		}
	case "repository":
		if _, err := os.ReadFile(os.Getenv("CORVINT_ANALYZER_CAPABILITY_VALUE")); err == nil {
			t.Fatal("ambient repository read was allowed")
		}
	case "worktree":
		if _, err := os.ReadFile(os.Getenv("CORVINT_ANALYZER_CAPABILITY_VALUE")); err == nil {
			t.Fatal("caller worktree read was allowed")
		}
	default:
		t.Fatal("unknown capability probe")
	}
}
