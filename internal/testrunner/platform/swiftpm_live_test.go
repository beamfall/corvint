//go:build darwin

package platform

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// swiftPMLiveFixture returns the operator-built SwiftPM package and pinned swift
// executable. Nothing is downloaded; absent inputs skip the actual witness.
func swiftPMLiveFixture(t *testing.T) (root, exe, sha, out string, inputs map[string]string) {
	t.Helper()
	root, exe = os.Getenv("CORVINT_SWIFTPM_LIVE_ROOT"), os.Getenv("CORVINT_SWIFTPM_LIVE_EXE")
	if root == "" || exe == "" {
		t.Skip("operator-built SwiftPM XCTest fixture and swift executable not supplied")
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	sha = tr.Digest(b)
	inputs = map[string]string{}
	for _, n := range []string{"Package.swift", "Sources/Proof/Proof.swift", "Tests/ProofTests/Proof.swift", "Tests/ProofTests/Hang.swift"} {
		b, e := os.ReadFile(filepath.Join(root, n))
		if e != nil {
			t.Fatal(e)
		}
		inputs[n] = tr.Digest(b)
	}
	out = os.Getenv("CORVINT_SWIFTPM_LIVE_OUT")
	if out == "" {
		if out, e = os.MkdirTemp("", "swiftpm-live-"); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("retained live evidence: %s", out)
	return root, exe, sha, out, inputs
}

func retainSwiftPMLive(t *testing.T, out, name string, r tr.Request, v tr.Invocation, x tr.Execution, o tr.Observation, execErr, parseErr error) {
	t.Helper()
	errText := func(e error) string {
		if e == nil {
			return ""
		}
		return e.Error()
	}
	raw, e := json.MarshalIndent(struct {
		Request      tr.Request
		Invocation   tr.Invocation
		Execution    tr.Execution
		Stdout       string
		Observation  tr.Observation
		ExecuteError string
		ParseError   string
	}{r, v, x, string(x.Input.Stdout), o, errText(execErr), errText(parseErr)}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(out, name+".json"), append(raw, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}

// TestSwiftPMXCTestLiveThreeOutcomes is the actual TRE-V0-029 witness: the
// serial native transport keeps pass, assertion failure and XCTSkip distinct
// through the shared executor, where SwiftPM's xUnit writer reports the skip
// as an ordinary pass.
func TestSwiftPMXCTestLiveThreeOutcomes(t *testing.T) {
	root, exe, sha, out, inputs := swiftPMLiveFixture(t)
	r := tr.Request{Runner: "swift-xctest", Root: root, Executable: exe, ExecutableSha256: sha, Config: filepath.Join(root, "Package.swift"), ConfigSha256: inputs["Package.swift"], InputFiles: inputs, ReportDir: filepath.Join(out, "three"), TimeoutSeconds: 600, Selectors: []string{"ProofTests.Proof/testPass", "ProofTests.Proof/testFail", "ProofTests.Proof/testSkip"}}
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	x, execErr := tr.Execute(context.Background(), r, v)
	o, parseErr := Parse(x.Input)
	o = tr.Normalize(x.Input, o)
	retainSwiftPMLive(t, out, "three", r, v, x, o, execErr, parseErr)
	if execErr != nil || parseErr != nil || x.Input.ExitCode != 1 || !o.Complete || len(o.Tests) != 3 {
		t.Fatalf("execute=%v parse=%v exit=%d observation=%+v", execErr, parseErr, x.Input.ExitCode, o)
	}
	want := map[string]string{"ProofTests.Proof/testPass": tr.Passed, "ProofTests.Proof/testFail": tr.Failed, "ProofTests.Proof/testSkip": tr.Skipped}
	for _, row := range o.Tests {
		if want[row.ID] != row.State {
			t.Fatalf("%s: got %s want %s", row.ID, row.State, want[row.ID])
		}
		if row.State == tr.Failed && row.FailureKind != tr.Assertion {
			t.Fatalf("assertion failure kind lost: %+v", row)
		}
	}
}

// liveStart is a process's state-free start time and command, or "" when it
// is gone or a zombie. Cleanup signals only an unchanged identity.
func liveStart(pid int) string {
	out, err := exec.Command("/bin/ps", "-o", "stat=,lstart=,command=", "-p", strconv.Itoa(pid)).Output()
	line := strings.TrimSpace(string(out))
	if err != nil || line == "" || strings.HasPrefix(line, "Z") {
		return ""
	}
	return line[strings.IndexByte(line, ' ')+1:]
}

// TestSwiftPMXCTestLiveDetachedTeardown is the actual TRE-V0-030 witness.
// testHang starts a Foundation Process helper, writes "xctestpid helperpid"
// as its readiness marker and hangs. SwiftPM 6.4 runs xctest and the helper
// outside swift-test's process group, so the group kill alone would leave
// both; without the profile's flag ("contained-only") TRE-V0-034's structural
// containment still retires them with no retirement report, and the
// profile's retirement records and removes them on timeout and on
// interruption.
func TestSwiftPMXCTestLiveDetachedTeardown(t *testing.T) {
	root, exe, sha, out, inputs := swiftPMLiveFixture(t)
	for _, mode := range []string{"contained-only", "timeout", "interrupt"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(out, mode+".ready")
			_ = os.Remove(marker)
			r := tr.Request{Runner: "swift-xctest", Root: root, Executable: exe, ExecutableSha256: sha, Config: filepath.Join(root, "Package.swift"), ConfigSha256: inputs["Package.swift"], InputFiles: inputs, ReportDir: filepath.Join(out, mode), TimeoutSeconds: 120, Selectors: []string{"ProofTests.Hang/testHang"}}
			if mode != "interrupt" {
				// The package is prebuilt by the three-outcome witness, so a
				// short bound times out while testHang is ready.
				r.TimeoutSeconds = 30
			}
			v, e := Build(r)
			if e != nil || !v.RetireDetachedDescendants {
				t.Fatalf("%+v %v", v, e)
			}
			v.Environment = map[string]string{"CORVINT_SWIFT_READY": marker}
			if mode == "contained-only" {
				v.RetireDetachedDescendants = false
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan []int, 1)
			done := make(chan struct{})
			go func() {
				defer close(ready)
				for {
					if b, err := os.ReadFile(marker); err == nil && strings.HasSuffix(string(b), "\n") {
						var pids []int
						for _, f := range strings.Fields(string(b)) {
							n, _ := strconv.Atoi(f)
							pids = append(pids, n)
						}
						starts := map[int]string{}
						for _, pid := range pids {
							starts[pid] = liveStart(pid)
						}
						t.Cleanup(func() {
							for pid, s := range starts {
								if s != "" && liveStart(pid) == s {
									_ = syscall.Kill(pid, syscall.SIGKILL)
								}
							}
						})
						if mode == "interrupt" {
							cancel()
						}
						ready <- pids
						return
					}
					select {
					case <-done:
						return
					case <-time.After(20 * time.Millisecond):
					}
				}
			}()
			x, execErr := tr.Execute(ctx, r, v)
			close(done)
			o, parseErr := Parse(x.Input)
			o = tr.Normalize(x.Input, o)
			retainSwiftPMLive(t, out, "teardown-"+mode, r, v, x, o, execErr, parseErr)
			pids := <-ready
			if len(pids) != 2 {
				t.Fatalf("readiness marker missing: execute=%v", execErr)
			}
			time.Sleep(100 * time.Millisecond)
			survivors := []int{}
			for _, pid := range pids {
				if liveStart(pid) != "" {
					survivors = append(survivors, pid)
				}
			}
			t.Logf("xctest=%d helper=%d survivors=%v retirement=%+v", pids[0], pids[1], survivors, x.Retirement)
			if o.Complete {
				t.Fatal("hung run produced a complete observation")
			}
			if mode == "contained-only" {
				if len(survivors) != 0 || x.Retirement != nil {
					t.Fatalf("structural containment left survivors=%v retirement=%+v", survivors, x.Retirement)
				}
				return
			}
			if execErr != nil || len(survivors) != 0 || x.Retirement == nil || !x.Retirement.Clean() {
				t.Fatalf("execute=%v survivors=%v retirement=%+v", execErr, survivors, x.Retirement)
			}
			retired := map[int]bool{}
			for _, p := range x.Retirement.Retired {
				retired[p.PID] = true
			}
			if !retired[pids[0]] || !retired[pids[1]] {
				t.Fatalf("xctest/helper identities not in the retirement report: %+v", x.Retirement)
			}
			if (mode == "timeout") != x.Input.TimedOut || (mode == "interrupt") != x.Input.Interrupted {
				t.Fatalf("lifecycle not retained: %+v", x.Input)
			}
		})
	}
}
