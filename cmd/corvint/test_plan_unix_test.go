//go:build unix

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTestPlanRefusesFIFO: a FIFO with no writer named as --input or --plan is refused as
// invalid input at once instead of blocking the open (TCN-V0-001, TCN-V0-011).
func TestTestPlanRefusesFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "input.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	input := testPlanInput(t, testPlanVariation("V1", nil))
	root, _ := testPlanRepository(t)
	for name, arguments := range map[string][]string{
		"input": {"consolidate", "--input", fifo},
		"plan":  {"check", "--input", input, "--plan", fifo},
	} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			code, stdout, stderr := runTestPlanCLI(t, append([]string{"--root", root, "test-plan"}, arguments...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "test-plan-invalid-input"`) {
				t.Errorf("%s: exit %d stdout %q stderr %s", name, code, stdout, stderr)
			}
		}()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatalf("%s: a FIFO blocked test-plan", name)
		}
	}
}
