package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// kernelRepository is a committed repository carrying a governing instruction
// file, which cliRepository deliberately does not.
func kernelRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md": "# Agent contract\n\nEvidence is pinned to immutable Git content.\n",
		"README.md": "# test\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commands := [][]string{
		{"init", "-q"},
		{"config", "user.email", "corvint@example.test"},
		{"config", "user.name", "Corvint Test"},
		{"add", "."},
		{"commit", "-qm", "initial"},
	}
	for _, arguments := range commands {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func runKernelCapture(t *testing.T, root string, arguments []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := runKernel(context.Background(), root, arguments, strings.NewReader(stdin), &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

// CKN-V0-008: `kernel` prints the envelope and `kernel verify` exits 0 only on
// an intact verdict.
func TestKernelVerbRendersAndVerifies(t *testing.T) {
	t.Parallel()
	root := kernelRepository(t)
	status, block, stderr := runKernelCapture(t, root, nil, "")
	if status != 0 {
		t.Fatalf("kernel status = %d, stderr %s", status, stderr)
	}
	if !strings.Contains(block, "BEGIN CORVINT KERNEL") || !strings.Contains(block, "END CORVINT KERNEL") {
		t.Fatalf("kernel block = %q", block)
	}
	if !strings.Contains(block, `"path":"AGENTS.md"`) {
		t.Fatalf("kernel omitted the governing authority: %q", block)
	}

	status, verdict, stderr := runKernelCapture(t, root, []string{"verify"}, "post-compaction summary\n"+block)
	if status != 0 {
		t.Fatalf("verify status = %d, stderr %s, verdict %s", status, stderr, verdict)
	}
	if !strings.Contains(verdict, `"state":"intact"`) {
		t.Fatalf("verdict = %s", verdict)
	}

	status, verdict, _ = runKernelCapture(t, root, []string{"verify"}, "the summary kept nothing pinned\n")
	if status == 0 || !strings.Contains(verdict, `"state":"missing"`) {
		t.Fatalf("missing verify status = %d, verdict = %s", status, verdict)
	}
}

// CKN-V0-008: `kernel verify` refuses stdin beyond the shared kernel input
// bound before attempting recovery.
func TestKernelVerifyRefusesOversizedInput(t *testing.T) {
	t.Parallel()
	root := kernelRepository(t)
	status, stdout, stderr := runKernelCapture(t, root, []string{"verify"}, strings.Repeat("x", gokernel.MaxInputBytes+1))
	if status != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-kernel-input"`) {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, stdout, stderr)
	}
}

// CKN-V0-008: the verb's argument surface is closed.
func TestKernelInvocationParsing(t *testing.T) {
	t.Parallel()
	repository := kernelRepository(t)
	root, rest, isKernel, err := parseKernelInvocation([]string{"--root=" + repository, "kernel", "--requirements=CKN-V0-001"})
	if !isKernel || err != nil || root == "" || len(rest) != 1 {
		t.Fatalf("parse = %q %v %v %v", root, rest, isKernel, err)
	}
	if _, _, isKernel, _ := parseKernelInvocation([]string{"observations"}); isKernel {
		t.Fatal("kernel parser claimed another verb")
	}
	options, err := parseKernelOptions([]string{"--requirements", "CKN-V0-001,CKN-V0-002"})
	if err != nil || len(options.requirements) != 2 || options.verify {
		t.Fatalf("options = %+v, err %v", options, err)
	}
	if _, err := parseKernelOptions([]string{"verify", "--requirements=CKN-V0-001"}); err == nil {
		t.Fatal("expected verify to refuse --requirements")
	}
	if _, err := parseKernelOptions([]string{"--limit", "3"}); err == nil {
		t.Fatal("expected an unrecognized flag to be refused")
	}
}
