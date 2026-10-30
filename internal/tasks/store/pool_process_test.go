//go:build unix

package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	// The listing is the fallback after a failed kernel read (PSR-V0-201).
	originalNative := poolNativeIdentity
	defer func() { poolNativeIdentity = originalNative }()
	poolNativeIdentity = func(int) (string, error) { return "", errors.New("kernel read failed") }
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

// CAL-V0-188 (V1-0989): the full process-table listing overruns its bound on
// a loaded host, as `ps -axo` measured 0.7-2.4s at load 45-60. That must not
// fail the cleanup proof of a group the kernel already reports empty, for the
// health path and the captured sweep path alike; a group that still has a
// member stays unproved and its listing failure is still uncertainty.
func TestCALV0188_GroupListingOverrun(t *testing.T) {
	original := poolProbe
	t.Cleanup(func() { poolProbe = original })
	listings := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		if len(argv) > 1 && argv[1] == "-axo" {
			listings++
			return nil, context.DeadlineExceeded
		}
		return original(ctx, argv...)
	}
	// V1-1294: the post-reap signal 0 is answered for the state each phase
	// models, because the real answer races the kernel. Darwin briefly answers
	// EPERM for a group whose only process was just reaped (13 of 200 reaps
	// at load 95), and the group SIGKILL before the leader's reap (V1-1036)
	// kills the survivor phase's member, so once launchd or init collects it
	// the answer is ESRCH. Signal 0 sends nothing; other signals are real.
	originalSignal := poolGroupSignal
	t.Cleanup(func() { poolGroupSignal = originalSignal })
	probeAnswer := syscall.ESRCH // the group the kernel already reports empty
	poolGroupSignal = func(pid int, signal syscall.Signal) error {
		if signal == 0 {
			return probeAnswer
		}
		return originalSignal(pid, signal)
	}
	done := &intent.PoolCommand{Argv: []string{"/usr/bin/true"}, TimeoutSeconds: "3"}
	if class, clean, _ := executePool(context.Background(), done, t.TempDir(), nil); class != "EXIT_ZERO" || !clean || listings != 0 {
		t.Fatalf("retired group unproved: %s clean=%v listings=%d", class, clean, listings)
	}
	if r := executePoolCaptured(context.Background(), done, t.TempDir(), nil); r.Class != "EXIT_ZERO" || !r.Clean || listings != 0 {
		t.Fatalf("retired captured group unproved: %s clean=%v listings=%d", r.Class, r.Clean, listings)
	}
	probeAnswer = syscall.EPERM // a group that still has a member, as Darwin answers
	marker := filepath.Join(t.TempDir(), "child")
	survivor := &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", "/bin/sleep 60 </dev/null >/dev/null 2>&1 & echo $! > " + marker}, TimeoutSeconds: "3"}
	_, clean, _ := executePool(context.Background(), survivor, t.TempDir(), nil)
	raw, e := os.ReadFile(marker)
	if e != nil {
		t.Fatal("survivor not started", e)
	}
	if pid, e := strconv.Atoi(strings.TrimSpace(string(raw))); e == nil {
		t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	}
	if clean || listings == 0 {
		t.Fatalf("surviving member admitted: clean=%v listings=%d", clean, listings)
	}
}

// PSR-V0-022: a Git source probe whose own group cleanup is unproved (here a
// pipe holder outlives the leader past the wait delay) records UNKNOWN, never
// SOURCE_CHANGED; the post-exit proof lets it replace only a passing class.
func TestPSRV0022_UnprovedProbeCleanupRecordsUnknown(t *testing.T) {
	bin := t.TempDir()
	if e := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n/bin/sleep 3 &\nexit 0\n"), 0o755); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin)
	_, _, e := sweepSource(context.Background(), t.TempDir())
	if e == nil || sweepSourceClass(e) != "UNKNOWN" {
		t.Fatalf("unproved probe cleanup: want UNKNOWN got %s (%v)", sweepSourceClass(e), e)
	}
	if got := poolPostClass("EXIT_ZERO", true, e); got != "UNKNOWN" {
		t.Fatalf("post-exit proof on a passing class: want UNKNOWN got %s", got)
	}
	// Unproved cleanup is terminal: a retryable failure class is replaced so
	// the retry planner cannot launch another attempt (PSR-V0-004).
	for _, failed := range []string{"EXIT_NONZERO", "STDOUT_MISMATCH"} {
		if got := poolPostClass(failed, false, e); got != "UNKNOWN" {
			t.Fatalf("post-exit proof on %s: want UNKNOWN got %s", failed, got)
		}
	}
	// A probe that hit its bound and whose cleanup is also unproved keeps the
	// PSR-V0-004 bound class, and is still terminal after exit; a clean bound
	// keeps the command's own failure as before.
	for _, c := range []struct {
		result poolCommandResult
		source string
		post   string
	}{
		{poolCommandResult{Class: "UNKNOWN", Bound: "TIMEOUT"}, "TIMEOUT", "TIMEOUT"},
		{poolCommandResult{Class: "UNKNOWN", Bound: "INTERRUPTED"}, "INTERRUPTED", "INTERRUPTED"},
		{poolCommandResult{Class: "UNKNOWN"}, "UNKNOWN", "UNKNOWN"},
		{poolCommandResult{Class: "TIMEOUT", Bound: "TIMEOUT", Clean: true}, "TIMEOUT", "EXIT_NONZERO"},
		{poolCommandResult{Class: "EXIT_NONZERO", Clean: true}, "SOURCE_CHANGED", "SOURCE_CHANGED"},
	} {
		failure := sweepGitFailure(c.result)
		if got := sweepSourceClass(failure); got != c.source {
			t.Fatalf("%+v: source class want %s got %s", c.result, c.source, got)
		}
		if got := poolPostClass("EXIT_NONZERO", false, failure); got != c.post {
			t.Fatalf("%+v: post class want %s got %s", c.result, c.post, got)
		}
	}
	if timed := executePoolCaptured(context.Background(), &intent.PoolCommand{Argv: []string{"/bin/sleep", "5"}, TimeoutSeconds: "1"}, t.TempDir(), nil); timed.Class != "TIMEOUT" || timed.Bound != "TIMEOUT" {
		t.Fatalf("runner bound: want TIMEOUT/TIMEOUT got %s/%s", timed.Class, timed.Bound)
	}
	// The pinned-cwd proof keeps the probe's own error rather than claiming
	// the path is not a Git worktree.
	pin, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	def := &intent.PoolCommand{Argv: []string{"/usr/bin/true"}, TimeoutSeconds: "3", Pinned: &intent.PinnedCwd{Path: pin, Revision: strings.Repeat("a", 40)}}
	if _, _, e = poolCommandDir(context.Background(), def, t.TempDir()); wire.CodeOf(e) != wire.CodeUnsupported || sweepSourceClass(e) != "UNKNOWN" {
		t.Fatalf("pinned probe: want UNSUPPORTED/UNKNOWN got %s/%s (%v)", wire.CodeOf(e), sweepSourceClass(e), e)
	}
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
	// The group is SIGKILLed before the leader's reap (V1-1036), so once the
	// killed child has also been collected the CAL-V0-188 ESRCH fast path
	// would skip the listing this test fails. Answer the post-reap signal 0
	// as Darwin does for a zombie-only group (EPERM); signal 0 sends nothing.
	originalSignal := poolGroupSignal
	poolGroupSignal = func(pid int, signal syscall.Signal) error {
		if signal == 0 {
			if _, e := os.Stat(marker); e == nil {
				return syscall.EPERM
			}
		}
		return originalSignal(pid, signal)
	}
	t.Cleanup(func() { poolGroupSignal = originalSignal })
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
	original, originalNative := poolProbe, poolNativeIdentity
	calls := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		if len(argv) > 1 && argv[1] == "-p" {
			calls++
			return nil, fmt.Errorf("injected runner identity unavailable")
		}
		return original(ctx, argv...)
	}
	// A kernel-read record is re-read with the kernel read (PSR-V0-201,
	// PSR-V0-204), so the injection covers that reader too.
	poolNativeIdentity = func(int) (string, error) {
		calls++
		return "", fmt.Errorf("injected runner identity unavailable")
	}
	return func() { poolProbe, poolNativeIdentity = original, originalNative }, &calls
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

// PSR-V0-201 (V1-1008): on Darwin the runner identity comes from the kernel,
// so a process listing that overruns its bound on a loaded host cannot fail a
// preparation; recovery re-reads a recorded identity with the reader that
// produced it, so a listed record from an earlier binary is never compared
// with a kernel identity.
func TestPSRV0201_RunnerIdentityNeedsNoListing(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the kernel runner identity is Darwin-only")
	}
	original := poolProbe
	t.Cleanup(func() { poolProbe = original })
	listings := 0
	poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
		listings++
		if len(argv) > 2 && argv[1] == "-p" && strings.HasPrefix(argv[len(argv)-1], "lstart") {
			return []byte("Thu Oct 22 09:00:00 2026\n"), nil
		}
		return nil, context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	identity, e := poolRunnerIdentityContext(ctx, os.Getpid())
	if e != nil || !strings.HasPrefix(identity, "darwin:") || listings != 0 {
		t.Fatalf("identity %q %v after %d listings, want a kernel identity and none", identity, e, listings)
	}
	if again, e := poolRecordedIdentity(os.Getpid(), identity); e != nil || again != identity || listings != 0 {
		t.Fatalf("recorded kernel identity re-read as %q %v after %d listings", again, e, listings)
	}
	if legacy, e := poolRecordedIdentity(os.Getpid(), "Thu Oct 22 09:00:00 2026"); e != nil || legacy != "Thu Oct 22 09:00:00 2026" || listings != 1 {
		t.Fatalf("listed record re-read as %q %v after %d listings, want the listing", legacy, e, listings)
	}
	if gone, e := poolRunnerIdentityContext(ctx, 1<<30); e != nil || gone != "" {
		t.Fatalf("absent process identity %q %v, want none", gone, e)
	}
}

// PSR-V0-204: equal start ticks with btimes within one second name one
// runner; further apart (a realtime clock step) is unknown, never another
// runner; another start tick is another runner.
func TestPSRV0204_LinuxIdentityClockStepIsUnknown(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want supervisor.IdentityMatch
	}{
		{"linux:1000:77", "linux:1000:77", supervisor.IdentitySame},
		{"linux:1000:77", "linux:1001:77", supervisor.IdentitySame},
		{"linux:1001:77", "linux:1000:77", supervisor.IdentitySame},
		{"linux:1000:77", "linux:1002:77", supervisor.IdentityUnknown},
		{"linux:4600:77", "linux:1000:77", supervisor.IdentityUnknown},
		{"linux:1000:77", "linux:1000:78", supervisor.IdentityDifferent},
		{"linux:1000:77", "Thu Oct 22 09:00:00 2026", supervisor.IdentityDifferent},
	} {
		if got := supervisor.CompareIdentity(c.a, c.b); got != c.want {
			t.Fatalf("compare %q %q = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
