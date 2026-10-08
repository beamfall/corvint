//go:build darwin && (arm64 || amd64)

package testrunner

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
)

func detachedPIDs(t *testing.T, marker string, stop <-chan struct{}) []int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(marker); err == nil {
			var pids []int
			for _, f := range strings.Fields(string(b)) {
				n, err := strconv.Atoi(f)
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, n)
			}
			return pids
		}
		select {
		case <-stop:
			return nil
		case <-time.After(5 * time.Millisecond):
		}
	}
	return nil
}

// processStart is the process's start time and command, or "" when it is
// gone or a zombie. Test cleanup signals only an unchanged identity.
func processStart(pid int) string {
	out, err := exec.Command("/bin/ps", "-o", "stat=,lstart=,command=", "-p", strconv.Itoa(pid)).Output()
	line := strings.TrimSpace(string(out))
	if err != nil || line == "" || strings.HasPrefix(line, "Z") {
		return ""
	}
	return line[strings.IndexByte(line, ' ')+1:]
}

// TestExecuteRetiresDetachedDescendants is TRE-V0-030's executor witness:
// a readiness-marked Setpgid child and setsid grandchild escape the group
// kill on timeout and interruption. With retirement requested the plan's
// Retirer records and retires them; without it TRE-V0-034's structural
// containment still retires them, and the receipt keeps its pre-retirement
// shape.
func TestExecuteRetiresDetachedDescendants(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		retire bool
	}{{"timeout", false}, {"timeout", true}, {"interrupt", true}, {"exit", true}} {
		t.Run(tc.mode+"-retire="+strconv.FormatBool(tc.retire), func(t *testing.T) {
			r, inv := executorRequest(t)
			r.TimeoutSeconds = 3
			marker := filepath.Join(r.Root, "ready")
			inv.Environment["CORVINT_EXEC_MODE"] = "detach"
			inv.Environment["CORVINT_EXEC_TARGET"] = marker
			if tc.mode == "exit" {
				inv.Environment["CORVINT_EXEC_EXIT"] = "1"
			}
			inv.RetireDetachedDescendants = tc.retire
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stop := make(chan struct{})
			pidsc := make(chan []int, 1)
			go func() {
				pids := detachedPIDs(t, marker, stop)
				starts := map[int]string{}
				for _, pid := range pids {
					starts[pid] = processStart(pid)
				}
				// Clean up only identities this test started, if they survive.
				t.Cleanup(func() {
					for pid, s := range starts {
						if s != "" && processStart(pid) == s {
							_ = syscall.Kill(pid, syscall.SIGKILL)
						}
					}
				})
				if tc.mode == "interrupt" {
					cancel()
				}
				pidsc <- pids
			}()
			out, err := Execute(ctx, r, inv)
			close(stop)
			pids := <-pidsc
			if len(pids) != 2 {
				t.Fatalf("readiness marker missing: %v %v", pids, err)
			}
			if err != nil {
				t.Fatalf("%v: %+v", err, out.Retirement)
			}
			switch tc.mode {
			case "timeout":
				if !out.Input.TimedOut {
					t.Fatal("timeout not retained")
				}
			case "interrupt":
				if !out.Input.Interrupted {
					t.Fatal("interruption not retained")
				}
			case "exit":
				if out.Input.ExitCode != 0 || out.Input.TimedOut || out.Input.Interrupted {
					t.Fatalf("%+v", out.Input)
				}
			}
			time.Sleep(50 * time.Millisecond)
			for _, pid := range pids {
				if processStart(pid) != "" {
					t.Fatalf("retire=%v: detached pid %d survived", tc.retire, pid)
				}
			}
			raw, _ := json.Marshal(out)
			if !tc.retire {
				if out.Retirement != nil || strings.Contains(string(raw), "retirement") {
					t.Fatal("receipt without retirement changed shape")
				}
				return
			}
			if out.Retirement == nil || !out.Retirement.Clean() || len(out.Retirement.Retired) != 2 {
				t.Fatalf("%+v", out.Retirement)
			}
			if tc.mode != "exit" {
				// A killed phase retains exitCode -1, which the closed
				// document decoder refuses independently of retirement.
				return
			}
			var back Execution
			if err := DecodeDocument(raw, &back); err != nil {
				t.Fatalf("retirement receipt does not decode: %v", err)
			}
			if again, _ := json.Marshal(back); string(again) != string(raw) {
				t.Fatal("retirement receipt bytes changed on decode")
			}
		})
	}
}

// TestRetirementSurvivesLaterExecutionFailure keeps the retirement record when
// a post-run binding check fails: the leader mutates a pinned input after its
// detached descendants are ready, and both are still retired and retained.
func TestRetirementSurvivesLaterExecutionFailure(t *testing.T) {
	r, inv := executorRequest(t)
	marker := filepath.Join(r.Root, "ready")
	inv.Environment["CORVINT_EXEC_MODE"] = "detach"
	inv.Environment["CORVINT_EXEC_TARGET"] = marker
	inv.Environment["CORVINT_EXEC_EXIT"] = "1"
	inv.Environment["CORVINT_EXEC_MUTATE"] = filepath.Join(r.Root, "source")
	inv.RetireDetachedDescendants = true
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		pids := detachedPIDs(t, marker, stop)
		for _, pid := range pids {
			if s := processStart(pid); s != "" {
				t.Cleanup(func() {
					if processStart(pid) == s {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				})
			}
		}
	}()
	out, err := Execute(context.Background(), r, inv)
	if err == nil {
		t.Fatal("mutated pinned input admitted")
	}
	if out.Retirement == nil || len(out.Retirement.Retired) != 2 || !out.Retirement.Clean() {
		t.Fatalf("retirement record lost on later failure: %v %+v", err, out.Retirement)
	}
}

// TestExecuteRetirementAdmission refuses ambiguous lifecycles before launch.
func TestExecuteRetirementAdmission(t *testing.T) {
	r, inv := executorRequest(t)
	inv.RetireDetachedDescendants = true
	inv.GracefulInterrupt = true
	if _, err := Execute(context.Background(), r, inv); err == nil {
		t.Fatal("graceful interrupt combined with detached retirement")
	}
	if _, err := os.Stat(r.ReportDir); err == nil {
		t.Fatal("refusal created the report directory")
	}
	r, inv = executorRequest(t)
	inv.RetireDetachedDescendants = true
	inv.Environment["CORVINT_TEST_RUNNER_OWNER"] = "forged"
	if _, err := Execute(context.Background(), r, inv); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("plan-supplied owner token admitted: %v", err)
	}
}

// TestRetirementFailureHidesPassingObservation binds the failure path: the
// executor's retirement error becomes an execution-boundary problem, and no
// complete or resolved observation survives it.
func TestRetirementFailureHidesPassingObservation(t *testing.T) {
	in := Input{Runner: "probe", ExitCode: 0, ExecutionProblems: []Problem{{"execution-boundary", "detached descendant retirement incomplete: 1 unretired, 0 problems"}}}
	o := Normalize(in, Observation{Complete: true, RetryInformation: NotApplicable, Tests: []Test{{ID: "a", State: Passed}}})
	if o.Complete || o.Tests[0].State != Unknown {
		t.Fatalf("cleanup failure hidden: %+v", o)
	}
}
