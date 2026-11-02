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
	node := nodeForTest(t)
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
	// AHI-022: the real binary pins the refusals the permissive fixture cannot, such as a non-repository root.
	realBinary := filepath.Join(t.TempDir(), "corvint")
	build = procgroup.Run(ctx, procgroup.Spec{Argv: []string{goTool, "build", "-o", realBinary, "./cmd/corvint"}, Dir: root, Env: testEnvironment("GOTOOLCHAIN=local"), Timeout: 30 * time.Minute, OutputLimit: 1 << 20}) // hang detector, not a budget (decision 0082)
	if build.Err != nil || build.ExitStatus != 0 {
		t.Fatalf("real corvint binary: %v exit=%d\n%s\n%s", build.Err, build.ExitStatus, build.Stdout, build.Stderr)
	}
	args := []string{node, "--test"}
	if os.Getenv("CORVINT_TEST_HOST_INTERRUPT_WITNESS") != "" {
		args = append(args, "--test-name-pattern=opencode interruption")
	}
	args = append(args, path)
	result := procgroup.Run(ctx, procgroup.Spec{Argv: args, Dir: filepath.Dir(path), Env: testEnvironment("GOTOOLCHAIN=local", "CORVINT_TEST_NATIVE_FIXTURE="+fixture, "CORVINT_TEST_REAL_BINARY="+realBinary), Timeout: 30 * time.Minute, OutputLimit: 1 << 20}) // hang detector, not a budget (decision 0082)
	if result.Err != nil || result.ExitStatus != 0 {
		t.Fatalf("host adapters: %v exit=%d\n%s\n%s", result.Err, result.ExitStatus, result.Stdout, result.Stderr)
	}
}

// requireNode resolves node for the node-backed host adapter tests (ASD-V0-644, V1-3740). A
// missing node is a skip naming NOT_RUN, so a lane host without node is not a red package, unless
// CORVINT_REQUIRE_NODE is set (make host-adapter-test sets it), when it is a failure.
func requireNode(lookPath func(string) (string, error), getenv func(string) string) (path, reason string, fatal bool) {
	path, err := lookPath("node")
	if err == nil {
		return path, "", false
	}
	// CI provisions node, so a missing node fails there too (ASD-V0-973).
	return "", "NOT_RUN: node is not on PATH; set CORVINT_REQUIRE_NODE=1 to fail instead", getenv("CORVINT_REQUIRE_NODE") != "" || getenv("CI") != ""
}

func nodeForTest(t *testing.T) string {
	t.Helper()
	path, reason, fatal := requireNode(exec.LookPath, os.Getenv)
	if fatal {
		t.Fatal(reason)
	}
	if reason != "" {
		t.Skip(reason)
	}
	return path
}

func TestASDV0644RequireNode(t *testing.T) {
	t.Parallel()
	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	env := func(value string) func(string) string {
		return func(key string) string {
			if key == "CORVINT_REQUIRE_NODE" {
				return value
			}
			return ""
		}
	}
	const reason = "NOT_RUN: node is not on PATH; set CORVINT_REQUIRE_NODE=1 to fail instead"
	if path, got, fatal := requireNode(missing, env("")); path != "" || got != reason || fatal {
		t.Fatalf("missing node, unset: %q %q %v", path, got, fatal)
	}
	if path, got, fatal := requireNode(missing, env("1")); path != "" || got != reason || !fatal {
		t.Fatalf("missing node, CORVINT_REQUIRE_NODE=1: %q %q %v", path, got, fatal)
	}
	ci := func(key string) string { return map[string]string{"CI": "true"}[key] }
	if path, got, fatal := requireNode(missing, ci); path != "" || got != reason || !fatal {
		t.Fatalf("missing node, CI set: %q %q %v", path, got, fatal) // ASD-V0-973
	}
	found := func(string) (string, error) { return "/opt/node/bin/node", nil }
	if path, got, fatal := requireNode(found, env("1")); path != "/opt/node/bin/node" || got != "" || fatal {
		t.Fatalf("found node: %q %q %v", path, got, fatal)
	}
}

func TestHostAdapterJavaScriptHosts(t *testing.T) {
	t.Parallel()
	t.Run("GOC-V0-008 native core and JavaScript host safety", testHostAdapterJavaScriptHosts)
}

func TestHostAdapterJavaScriptHarnessInterruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host adapters explicitly refuse Windows process-tree cleanup")
	}
	// The child run skips without node, so the witness would never appear (ASD-V0-644).
	nodeForTest(t)
	witness := filepath.Join(t.TempDir(), "descendant.pid")
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// The harness builds two Go binaries before the witness exists, so this bound is a hang
	// detector like theirs, not a budget (decision 0082): 20 s expired under host load.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	t.Cleanup(cancel)
	done := make(chan procgroup.Observation, 1)
	go func() {
		done <- procgroup.Run(ctx, procgroup.Spec{Argv: []string{os.Args[0], "-test.run=^TestHostAdapterJavaScriptHosts$"}, Dir: directory, Env: testEnvironment("CORVINT_TEST_HOST_INTERRUPT_WITNESS=" + witness), Timeout: 30 * time.Minute, OutputLimit: 1 << 20})
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
			t.Fatalf("harness ended before descendant witness: %v exit=%d\n%s\n%s", result.Err, result.ExitStatus, result.Stdout, result.Stderr)
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
	// (integrations/host-adapters.test.mjs:65), so it is orphaned into its own session:
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
