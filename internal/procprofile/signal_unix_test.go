//go:build unix

package procprofile

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

const signalHelperEnv = "CORVINT_PROCPROFILE_SIGNAL_HELPER"

// TestPRBV0004SignalHelperProcess is the child of TestPRBV0004SignalWritesProfiles; it is a
// no-op in an ordinary run.
func TestPRBV0004SignalHelperProcess(t *testing.T) {
	if os.Getenv(signalHelperEnv) != "1" {
		t.Skip("helper process only")
	}
	// The helper waits for its parent's signal; it exits if the parent dies first (V1-1498).
	testsupport.ExitWhenParentGone()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(Run(ctx, os.LookupEnv, Reporter(os.Stderr, "helper"), func() int {
		_, _ = os.Stdout.WriteString("ready\n")
		<-ctx.Done()
		return 0
	}))
}

// PRB-V0-004: a process that receives SIGINT or SIGTERM still writes both profiles.
func TestPRBV0004SignalWritesProfiles(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
			cmd := exec.Command(os.Args[0], "-test.run=^TestPRBV0004SignalHelperProcess$", "-test.count=1")
			cmd.Env = append(os.Environ(), signalHelperEnv+"=1", "CORVINT_CPUPROFILE="+cpu, "CORVINT_MEMPROFILE="+mem, testsupport.ParentPIDEntry())
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
				_ = cmd.Process.Kill()
				t.Fatalf("helper did not start: %q %v", line, err)
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Fatalf("helper exit: %v", err)
			}
			requireProfile(t, cpu)
			requireProfile(t, mem)
		})
	}
}
