//go:build unix

package store

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// CAL-V0-033.
func TestPoolProcessDescendants(t *testing.T) {
	for _, scenario := range []string{"success", "timeout", "interrupt"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			pids := filepath.Join(root, "pids")
			script := `trap 'kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :' INT TERM
/bin/sh -c 'sleep 60 & child=$!; echo $$ >> "$1"; echo $child >> "$1"; trap '\''kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :'\'' EXIT INT TERM; wait "$child"' sh "$1" &
child=$!
echo $$ >> "$1"
while [ "$(/usr/bin/wc -l < "$1")" -lt 3 ]; do sleep 0.01; done
if [ "$2" = success ]; then exit 0; fi
wait "$child"
`
			file := filepath.Join(root, "probe.sh")
			if e := os.WriteFile(file, []byte(script), 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := wire.Count("1")
			if scenario != "timeout" {
				timeout = "5"
			}
			def := &intent.PoolCommand{Argv: []string{"/bin/sh", file, pids, scenario}, TimeoutSeconds: timeout}
			done := make(chan struct{})
			var class string
			var clean bool
			go func() { class, clean, _ = executePool(ctx, def, root, nil); close(done) }()
			owned := map[int]string{}
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(8 * time.Second):
					t.Error("runner did not stop")
				}
				for pid, identity := range owned {
					current, e := poolRunnerIdentity(pid)
					if e == nil && current != "" && current == identity {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			})
			{
				deadline := time.Now().Add(3 * time.Second)
				for {
					raw, _ := os.ReadFile(pids)
					if len(strings.Fields(string(raw))) >= 3 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("descendants did not start")
					}
					time.Sleep(10 * time.Millisecond)
				}
				raw, _ := os.ReadFile(pids)
				for _, p := range strings.Fields(string(raw)) {
					pid, _ := strconv.Atoi(p)
					identity, _ := poolRunnerIdentity(pid)
					if identity != "" {
						owned[pid] = identity
					}
				}
				if scenario == "interrupt" {
					cancel()
				}
			}
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				t.Fatal("runner hung")
			}
			want := map[string]string{"success": "EXIT_ZERO", "timeout": "TIMEOUT", "interrupt": "INTERRUPTED"}[scenario]
			if class != want || !clean {
				t.Fatalf("%s clean=%v", class, clean)
			}
			raw, e := os.ReadFile(pids)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range strings.Fields(string(raw)) {
				pid, _ := strconv.Atoi(p)
				state, e := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "stat=").Output()
				if e == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
					t.Errorf("surviving descendant %d: %s", pid, state)
				}
			}
		})
	}
}

func TestPSRCapturedDescendants(t *testing.T) {
	for _, scenario := range []string{"success", "timeout", "interrupt"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			pids := filepath.Join(root, "pids")
			script := `trap 'kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :' INT TERM
/bin/sh -c 'sleep 60 & child=$!; echo $$ >> "$1"; echo $child >> "$1"; trap '\''kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :'\'' EXIT INT TERM; wait "$child"' sh "$1" &
child=$!
echo $$ >> "$1"
while [ "$(/usr/bin/wc -l < "$1")" -lt 3 ]; do sleep 0.01; done
if [ "$2" = success ]; then exit 0; fi
wait "$child"
`
			file := filepath.Join(root, "probe.sh")
			if e := os.WriteFile(file, []byte(script), 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := wire.Count("1")
			if scenario != "timeout" {
				timeout = "5"
			}
			def := &intent.PoolCommand{Argv: []string{"/bin/sh", file, pids, scenario}, TimeoutSeconds: timeout}
			done := make(chan struct{})
			var class string
			var clean bool
			go func() {
				result := executePoolCaptured(ctx, def, root, nil)
				class, clean = result.Class, result.Clean
				close(done)
			}()
			owned := map[int]string{}
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(8 * time.Second):
					t.Error("runner did not stop")
				}
				for pid, identity := range owned {
					current, e := poolRunnerIdentity(pid)
					if e == nil && current != "" && current == identity {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			})
			{
				deadline := time.Now().Add(3 * time.Second)
				for {
					raw, _ := os.ReadFile(pids)
					if len(strings.Fields(string(raw))) >= 3 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("descendants did not start")
					}
					time.Sleep(10 * time.Millisecond)
				}
				raw, _ := os.ReadFile(pids)
				for _, p := range strings.Fields(string(raw)) {
					pid, _ := strconv.Atoi(p)
					identity, _ := poolRunnerIdentity(pid)
					if identity != "" {
						owned[pid] = identity
					}
				}
				if scenario == "interrupt" {
					cancel()
				}
			}
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				t.Fatal("runner hung")
			}
			want := map[string]string{"success": "UNKNOWN", "timeout": "TIMEOUT", "interrupt": "INTERRUPTED"}[scenario]
			if class != want || (scenario != "success" && !clean) {
				t.Fatalf("%s clean=%v", class, clean)
			}
			raw, e := os.ReadFile(pids)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range strings.Fields(string(raw)) {
				pid, _ := strconv.Atoi(p)
				state, e := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "stat=").Output()
				if e == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
					t.Errorf("surviving descendant %d: %s", pid, state)
				}
			}
		})
	}
}

// PSR-V0-010: reach each identity/group probe with a deadline; never infer quiescence from failure.
func TestPSRBoundedProbes(t *testing.T) {
	original := poolProbe
	defer func() { poolProbe = original }()
	calls := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		calls++
		if len(argv) == 0 || argv[0] != "/bin/ps" {
			t.Fatal(argv)
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	for _, group := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		start := time.Now()
		if group {
			live, e := poolGroupLiveContext(ctx, 1)
			if !live || e == nil {
				t.Fatal("false group quiescence")
			}
		} else {
			identity, e := poolRunnerIdentityContext(ctx, 1)
			if identity != "" || e == nil {
				t.Fatal("false identity")
			}
		}
		cancel()
		if time.Since(start) > time.Second {
			t.Fatal("probe unbounded")
		}
	}
	if calls != 2 {
		t.Fatal("injected path not reached", calls)
	}
	t.Run("actual-blocked-probe-process", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "probe")
		reached := 0
		poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
			if len(argv) == 0 || argv[0] != "/bin/ps" {
				t.Fatal(argv)
			}
			reached++
			return original(ctx, "/bin/sh", "-c", "echo $$ > '"+marker+"'; exec /bin/sleep 60")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		live, e := poolGroupLiveContext(ctx, 1)
		if e == nil || !live || reached != 1 || time.Since(start) > 2*time.Second {
			t.Fatal("actual probe deadline/quiescence", live, e, reached)
		}
		raw, e := os.ReadFile(marker)
		if e != nil {
			t.Fatal("probe process not reached", e)
		}
		pid, e := strconv.Atoi(strings.TrimSpace(string(raw)))
		if e != nil {
			t.Fatal(e)
		}
		if raw, e := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=").CombinedOutput(); e == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("probe survived joined cancellation: %s", raw)
		}
	})
}
func TestPSRCapturedOutputLimit(t *testing.T) {
	result := executePoolCaptured(context.Background(), &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", "while :; do printf 1234567890123456789012345678901234567890; done"}, TimeoutSeconds: "3"}, t.TempDir(), nil)
	if result.Class != "OUTPUT_LIMIT" || len(result.Stdout)+len(result.Stderr) > 65536 {
		t.Fatalf("%s %d", result.Class, len(result.Stdout)+len(result.Stderr))
	}
	if !result.Clean {
		t.Fatal("output-limited group unproved")
	}
}

// Test-only exports let the external native-store fixtures inject response loss
// after the real commit; no product export or fake receipt bypass is installed.
func PSRTestResponseFailure(ctx context.Context, hook func(LeaseChoice, *Report, error) error) context.Context {
	return context.WithValue(ctx, sweepResponseKey{}, hook)
}

// PSRTestRequestFailure fails a sweep write before its writer runs, as
// contention that commits nothing would.
func PSRTestRequestFailure(ctx context.Context, hook func(LeaseChoice) error) context.Context {
	return context.WithValue(ctx, sweepRequestKey{}, hook)
}
func PSRTestFirstCleanupProbe(t *testing.T, marker string) *int {
	t.Helper()
	original := poolProbe
	calls := new(int)
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		if len(argv) > 2 && argv[1] == "-axo" && *calls == 0 {
			if _, e := os.Stat(marker); e == nil {
				*calls++
				return nil, fmt.Errorf("injected first cleanup probe failure")
			}
		}
		return original(ctx, argv...)
	}
	t.Cleanup(func() { poolProbe = original })
	return calls
}
func TestPSRSignalTiming(t *testing.T) {
	r := executePoolCaptured(context.Background(), &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", "printf out; printf err >&2; kill -TERM $$"}, TimeoutSeconds: "3"}, t.TempDir(), nil)
	if r.Class != "SIGNAL" || r.Signal == nil || r.Signal.Int() != int64(syscall.SIGTERM) || r.Exit != nil || !r.Clean {
		t.Fatalf("signal identity: %+v", r)
	}
	if r.Timing.StartedAt.Uint64() == 0 || r.Timing.Deadline.Uint64() <= r.Timing.StartedAt.Uint64() || r.Timing.WaitReturnedAt.Uint64() < r.Timing.StartedAt.Uint64() || r.Timing.CleanupEndedAt.Uint64() < r.Timing.WaitReturnedAt.Uint64() || r.Timing.CleanupAllowanceMillis != poolCleanupAllowanceMillis || r.Timing.CleanupMillis.Uint64() > uint64(poolCleanupAllowance.Milliseconds()) {
		t.Fatalf("timing: %+v", r.Timing)
	}
	if string(r.Stdout) != "out" || string(r.Stderr) != "err" {
		t.Fatal("streams")
	}
}

// This seam changes only a process-identity read, after the native fixture has
// proved a live sweep owner. It never fabricates a gone process.
func PSRTestUnknownRunner() (func(), *int) {
	original := poolProbe
	calls := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		if len(argv) > 1 && argv[1] == "-p" {
			calls++
			return nil, fmt.Errorf("injected runner identity unavailable")
		}
		return original(ctx, argv...)
	}
	return func() { poolProbe = original }, &calls
}

// PSR-V0-007/010: the recorded cleanup allowance is the enforced bound and the
// value the observation codec accepts; probe diagnostics stay out of parsed output.
func TestPSRCleanupAllowanceAndProbeStreams(t *testing.T) {
	if poolCleanupAllowanceMillis != "5000" {
		t.Fatal("recorded allowance differs from observation codec", poolCleanupAllowanceMillis)
	}
	out, e := poolProbe(context.Background(), "/bin/sh", "-c", "printf listing; printf diagnostic >&2")
	if e != nil || string(out) != "listing" {
		t.Fatalf("probe streams %q %v", out, e)
	}
}

// PSR-V0-004: a Git source observation that hits its bound records TIMEOUT or
// INTERRUPTED, not SOURCE_CHANGED; any other failure still records SOURCE_CHANGED.
func TestPSRSweepSourceBoundClass(t *testing.T) {
	root := t.TempDir()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tc := range []struct {
		ctx  context.Context
		want string
	}{{cancelled, "INTERRUPTED"}, {expired, "TIMEOUT"}, {context.Background(), "SOURCE_CHANGED"}} {
		_, _, e := sweepSource(tc.ctx, root)
		if e == nil || sweepSourceClass(e) != tc.want {
			t.Fatalf("want %s got %s (%v)", tc.want, sweepSourceClass(e), e)
		}
	}
}
