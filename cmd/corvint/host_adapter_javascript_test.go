package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func testHostAdapterJavaScriptHosts(t *testing.T) {
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	t.Cleanup(stop)
	if runtime.GOOS == "windows" {
		t.Skip("host adapters explicitly refuse unsupported Windows process-tree cleanup")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required to verify the optional Gemini/OpenCode host adapters")
	}
	path, err := filepath.Abs("../../integrations/host-adapters.test.mjs")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(t.TempDir(), "adapter-fixture")
	root := filepath.Dir(filepath.Dir(path))
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	build := procgroup.Run(ctx, procgroup.Spec{Argv: []string{goTool, "build", "-o", fixture, "./integrations/testfixture"}, Dir: root, Env: testEnvironment("GOTOOLCHAIN=local"), Timeout: 30 * time.Minute, OutputLimit: 1 << 20}) // hang detector, not a budget (decision 0082)
	if build.Err != nil || build.ExitStatus != 0 {
		t.Fatalf("native adapter fixture: %v exit=%d\n%s\n%s", build.Err, build.ExitStatus, build.Stdout, build.Stderr)
	}
	args := []string{node, "--test"}
	if os.Getenv("CORVINT_TEST_HOST_INTERRUPT_WITNESS") != "" {
		args = append(args, "--test-name-pattern=opencode interruption")
	}
	args = append(args, path)
	result := procgroup.Run(ctx, procgroup.Spec{Argv: args, Dir: filepath.Dir(path), Env: testEnvironment("GOTOOLCHAIN=local", "CORVINT_TEST_NATIVE_FIXTURE="+fixture), Timeout: 30 * time.Minute, OutputLimit: 1 << 20}) // hang detector, not a budget (decision 0082)
	if result.Err != nil || result.ExitStatus != 0 {
		t.Fatalf("host adapters: %v exit=%d\n%s\n%s", result.Err, result.ExitStatus, result.Stdout, result.Stderr)
	}
}

func TestHostAdapterJavaScriptHosts(t *testing.T) {
	t.Run("GOC-V0-008 native core and JavaScript host safety", testHostAdapterJavaScriptHosts)
}

func TestHostAdapterJavaScriptHarnessInterruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host adapters explicitly refuse Windows process-tree cleanup")
	}
	witness := filepath.Join(t.TempDir(), "descendant.pid")
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	t.Cleanup(cancel)
	done := make(chan procgroup.Observation, 1)
	go func() {
		done <- procgroup.Run(ctx, procgroup.Spec{Argv: []string{os.Args[0], "-test.run=^TestHostAdapterJavaScriptHosts$"}, Dir: directory, Env: testEnvironment("CORVINT_TEST_HOST_INTERRUPT_WITNESS=" + witness), Timeout: 20 * time.Second, OutputLimit: 1 << 20})
	}()
	var pid int
	for pid == 0 {
		if raw, err := os.ReadFile(witness); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		}
		if pid != 0 {
			break
		}
		select {
		case result := <-done:
			t.Fatalf("harness ended before descendant witness: %v\n%s\n%s", result.Err, result.Stdout, result.Stderr)
		case <-ctx.Done():
			<-done
			t.Fatal("descendant witness deadline")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	result := <-done
	if !result.Cancelled || !result.OwnedProcessGroupCleanup {
		t.Fatalf("outer harness cleanup incomplete: %+v", result)
	}
	// The witness pid is a node child spawned with {detached: true}
	// (integrations/host-adapters.test.mjs:51,206), so it is orphaned into its own session:
	// OwnedProcessGroupCleanup above only proves the outer wrapper's own process group is
	// quiescent, not that this separately-signalled orphan has finished exiting. That is a
	// second, independent OS-scheduled completion, so this deadline is a hang detector, not a
	// budget (decision 0082): 500 ms starved under host load and produced a one-in-three false
	// failure with no code path change, so it is widened here, not shortened for speed.
	deadline := time.Now().Add(5 * time.Second)
	for {
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		err = process.Signal(syscall.Signal(0))
		_ = process.Release()
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("host fixture descendant survived wrapper interruption")
}
