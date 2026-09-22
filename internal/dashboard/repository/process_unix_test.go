//go:build darwin || linux

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestNormalLeaderExitWithSurvivingDescendantFailsAfterCleanup(t *testing.T) {
	authority, pidFile := processTestAuthority(t, "normal-residue")
	cwd := stableTestDirectory(t, "normal-cwd-")
	attempt, err := acquireProcessTest(context.Background(), authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
		return current.run(ctx, cwd, nil, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := attempt.stop(); err != nil {
			t.Error(err)
		}
	})
	<-attempt.done
	failure := attempt.failure
	if failure == nil || failure.Code != FailureRepositoryUnavailable {
		t.Fatalf("failure = %#v, want repository unavailable", failure)
	}
	for _, pid := range readHelperPIDs(t, pidFile) {
		assertProcessGone(t, pid)
	}
}

func TestCancellationUsesOneGraceAndKillsHostileTree(t *testing.T) {
	authority, pidFile := processTestAuthority(t, "hostile-tree")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cwd := stableTestDirectory(t, "cancel-cwd-")
	attempt, err := acquireProcessTest(ctx, authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
		return current.run(ctx, cwd, nil, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := attempt.stop(); err != nil {
			t.Error(err)
		}
	})
	t.Run("LOD-V0-005 containment", func(t *testing.T) {
		started := time.Now()
		cancel()
		select {
		case <-attempt.done:
			failure := attempt.failure
			if failure == nil || failure.Code != FailureRepositoryUnavailable {
				t.Fatalf("failure = %#v, want repository unavailable", failure)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("cancelled contained command did not return")
		}
		elapsed := time.Since(started)
		if elapsed < 200*time.Millisecond || elapsed > 1500*time.Millisecond {
			t.Fatalf("cleanup duration = %s, want one 250ms grace plus bounded proof", elapsed)
		}
		for _, pid := range readHelperPIDs(t, pidFile) {
			assertProcessGone(t, pid)
		}
	})
}

func TestOutputLimitUsesContainedCleanupForHostileTree(t *testing.T) {
	authority, pidFile := processTestAuthority(t, "hostile-output")
	cwd := stableTestDirectory(t, "output-cwd-")
	attempt, err := acquireProcessTest(context.Background(), authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
		return current.runWithInput(ctx, cwd, nil, nil, nil, 32, false)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := attempt.stop(); err != nil {
			t.Error(err)
		}
	})
	<-attempt.done
	failure := attempt.failure
	if failure == nil || failure.Code != FailureRepositoryUnavailable {
		t.Fatalf("failure = %#v, want repository unavailable", failure)
	}
	for _, pid := range readHelperPIDs(t, pidFile) {
		assertProcessGone(t, pid)
	}
}

func TestPreCancelledContextStartsNoChild(t *testing.T) {
	authority, pidFile := processTestAuthority(t, "hostile-tree")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, failure := authority.run(ctx, stableTestDirectory(t, "precancel-cwd-"), nil, nil)
	if failure == nil || failure.Code != FailureRepositoryUnavailable {
		t.Fatalf("failure = %#v, want repository unavailable", failure)
	}
	if pids := readHelperPIDsIfPresent(pidFile); len(pids) != 0 {
		t.Fatalf("pre-cancelled operation started descendants: %v", pids)
	}
}

func TestProcessReadinessRetriesOnlyCompletedPreStartRefusal(t *testing.T) {
	t.Run("LOD-V0-005 acquisition", func(t *testing.T) {
		for _, persistent := range []bool{false, true} {
			t.Run(strconv.FormatBool(persistent), func(t *testing.T) {
				authority, pidFile := processTestAuthority(t, "hostile-tree")
				cwd := stableTestDirectory(t, "retry-cwd-")
				var calls, refused, completed, cancelled atomic.Int32
				started := time.Now()
				attempt, err := acquireProcessTest(context.Background(), authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
					ordinal := calls.Add(1)
					if ordinal > 1 && (current == authority || current.budget != authority.budget || current.drifted) {
						t.Error("retry reused drifted authority or reset the budget")
					}
					var result commandResult
					var failure *Failure
					if persistent || ordinal == 1 {
						result, failure = refuseProcessTestExecutable(t, ctx, current, cwd)
					} else {
						result, failure = current.run(ctx, cwd, nil, nil)
					}
					if result.executableRefusedBeforeStart {
						refused.Add(1)
					} else if ctx.Err() != nil {
						cancelled.Add(1)
					} else {
						completed.Add(1)
					}
					return result, failure
				})
				if persistent {
					if err == nil || err.Error() != "helper descendant PID was not published" || calls.Load() < 2 || time.Since(started) < processReadinessTimeout || time.Since(started) > processReadinessTimeout+time.Second {
						t.Fatalf("persistent refusal: calls=%d elapsed=%s error=%v", calls.Load(), time.Since(started), err)
					}
					if len(readHelperPIDsIfPresent(pidFile)) != 0 {
						t.Fatal("qualification refusal started a helper")
					}
					if completed.Load() != 0 || cancelled.Load() > 1 || refused.Load()+cancelled.Load() != calls.Load() {
						t.Fatal("persistent refusal reached a different execution phase")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := attempt.stop(); err != nil {
						t.Error(err)
					}
				})
				if err := attempt.stop(); err != nil {
					t.Fatal(err)
				}
				if calls.Load() < 2 || completed.Load()+cancelled.Load() != 1 || refused.Load() != calls.Load()-1 {
					t.Fatalf("unexpected execution phases: attempts=%d refusals=%d completions=%d", calls.Load(), refused.Load(), completed.Load())
				}
				for _, pid := range readHelperPIDs(t, pidFile) {
					assertProcessGone(t, pid)
				}
			})
		}
	})
}

func refuseProcessTestExecutable(t *testing.T, ctx context.Context, authority *Authority, cwd string) (commandResult, *Failure) {
	t.Helper()
	mode := os.FileMode(authority.executableBefore.identity.mode)
	if err := os.Chmod(authority.executable, mode^0100); err != nil {
		t.Error(err)
		return commandResult{}, unavailable(ReasonUnavailable)
	}
	result, failure := authority.run(ctx, cwd, nil, nil)
	if err := os.Chmod(authority.executable, mode); err != nil {
		t.Error(err)
	}
	if !result.executableRefusedBeforeStart && ctx.Err() == nil {
		t.Errorf("fixture did not reach the actual pre-Start qualification refusal: failure=%#v life=%v children=%d", failure, authority.life.Err(), authority.budget.children)
	}
	return result, failure
}

func TestProcessReadinessDoesNotRetryHelperExitOrPostStartDrift(t *testing.T) {
	t.Run("LOD-V0-005 phase", func(t *testing.T) {
		for _, mode := range []string{"unknown-mode", "changed-executable"} {
			t.Run(mode, func(t *testing.T) {
				authority, pidFile := processTestAuthority(t, mode)
				cwd := stableTestDirectory(t, "early-cwd-")
				var calls, refused, completed atomic.Int32
				var terminalDrift atomic.Bool
				attempt, err := acquireProcessTest(context.Background(), authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
					calls.Add(1)
					result, failure := current.run(ctx, cwd, nil, nil)
					if result.executableRefusedBeforeStart {
						refused.Add(1)
					} else {
						completed.Add(1)
						terminalDrift.Store(current.drifted)
					}
					return result, failure
				})
				if err == nil || attempt == nil || completed.Load() != 1 || refused.Load() != calls.Load()-1 || attempt.result.executableRefusedBeforeStart {
					t.Fatalf("early completion retried: calls=%d attempt=%#v error=%v", calls.Load(), attempt, err)
				}
				if mode == "unknown-mode" && attempt.result.exit != 64 {
					t.Fatalf("helper exit lost: result=%#v failure=%#v drift=%v error=%v", attempt.result, attempt.failure, terminalDrift.Load(), err)
				}
				if mode == "changed-executable" && (!terminalDrift.Load() || attempt.failure == nil || attempt.failure.Reason != ReasonUnavailable) {
					t.Fatal("helper did not reach post-Start executable drift")
				}
			})
		}
	})
}

func TestProcessReadinessFailureCancelsAndJoinsHostileTree(t *testing.T) {
	t.Run("LOD-V0-005 cleanup", func(t *testing.T) {
		authority, pidFile := processTestAuthority(t, "hostile-tree")
		auditFile := pidFile + ".audit"
		authority.environment[1] = "CORVINT_PROCESS_HELPER_PID_FILE=" + auditFile
		t.Cleanup(func() {
			for _, pid := range readHelperPIDsIfPresent(auditFile) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		})
		cwd := stableTestDirectory(t, "unready-cwd-")
		attempt, err := acquireProcessTest(context.Background(), authority, pidFile, func(ctx context.Context, current *Authority) (commandResult, *Failure) {
			return current.run(ctx, cwd, nil, nil)
		})
		if err == nil || attempt == nil {
			t.Fatalf("missing readiness PID accepted: %v", err)
		}
		select {
		case <-attempt.done:
		default:
			t.Fatal("readiness failure left a running command")
		}
		for _, pid := range readHelperPIDs(t, auditFile) {
			assertProcessGone(t, pid)
		}
	})
}

func TestHelperPIDReadinessRequiresCompleteCanonicalLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pids")
	for _, body := range []string{"12", "12\n34", "12 34\n", "12\njunk\n", "012\n", "0\n"} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if pids := readHelperPIDsIfPresent(path); len(pids) != 0 {
			t.Fatalf("incomplete or invalid PID record accepted: %q => %v", body, pids)
		}
	}
	if err := os.WriteFile(path, []byte("12\n34\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if pids := readHelperPIDsIfPresent(path); len(pids) != 2 || pids[0] != 12 || pids[1] != 34 {
		t.Fatalf("complete PID records rejected: %v", pids)
	}
}

func processTestAuthority(t *testing.T, mode string) (*Authority, string) {
	t.Helper()
	helperDirectory := stableTestDirectory(t, "process-helper-")
	helper := filepath.Join(helperDirectory, "process-helper")
	build := exec.Command("go", "build", "-o", helper, "./testdata/process_helper")
	build.Dir = "."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process helper: %v: %s", err, output)
	}
	var executableFile *os.File
	var executableBefore executableEvidence
	var failure *Failure
	qualificationDeadline := time.Now().Add(5 * time.Second)
	for {
		executableFile, executableBefore, failure = openExecutableStableRead(context.Background(), helper)
		if failure == nil || failure.Code != FailureRepositoryChanged || time.Now().After(qualificationDeadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if failure != nil {
		t.Fatalf("qualify process helper: %#v", failure)
	}
	budget := NewBudget(context.Background())
	life, cancel := context.WithCancel(budget.context)
	pidDirectory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(pidDirectory, "pids")
	authority := &Authority{
		life: life, cancel: cancel, budget: budget,
		ownBudget:  true,
		executable: helper, executableFile: executableFile, executableBefore: executableBefore,
		environment: []string{
			"CORVINT_PROCESS_HELPER_MODE=" + mode,
			"CORVINT_PROCESS_HELPER_PID_FILE=" + pidFile,
		},
	}
	t.Cleanup(func() {
		for _, pid := range readHelperPIDsIfPresent(pidFile) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		authority.Close()
	})
	return authority, pidFile
}

const processReadinessTimeout = 5 * time.Second

type processTestAttempt struct {
	cancel       context.CancelFunc
	done         chan struct{}
	result       commandResult
	failure      *Failure
	ownAuthority *Authority
}

func (attempt *processTestAttempt) stop() error {
	attempt.cancel()
	select {
	case <-attempt.done:
		if attempt.ownAuthority != nil {
			attempt.ownAuthority.Close()
		}
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("cancelled contained command did not return")
	}
}

// Acquisition retries only a completed refusal that positively precedes Start.
// Directory metadata can change while binding the helper's shared ancestors;
// the actual executable must still equal its original fully read evidence.
// Production authority execution never retries, and every fixture attempt uses
// the same five-second readiness deadline. Once a PID is ready, no retry occurs.
func acquireProcessTest(ctx context.Context, authority *Authority, pidFile string, run func(context.Context, *Authority) (commandResult, *Failure)) (*processTestAttempt, error) {
	setup, cancelSetup := context.WithTimeout(ctx, processReadinessTimeout)
	defer cancelSetup()
	original := authority
	for setup.Err() == nil {
		attemptContext, cancel := context.WithCancel(ctx)
		attempt := &processTestAttempt{cancel: cancel, done: make(chan struct{})}
		if authority != original {
			attempt.ownAuthority = authority
		}
		go func() {
			attempt.result, attempt.failure = run(attemptContext, authority)
			close(attempt.done)
		}()
		for {
			if setup.Err() != nil {
				if err := attempt.stop(); err != nil {
					return attempt, err
				}
				return attempt, errors.New("helper descendant PID was not published")
			}
			if len(readHelperPIDsIfPresent(pidFile)) != 0 && setup.Err() == nil {
				return attempt, nil
			}
			select {
			case <-attempt.done:
				if setup.Err() == nil && len(readHelperPIDsIfPresent(pidFile)) != 0 && setup.Err() == nil {
					return attempt, nil
				}
				cancel()
				if !attempt.result.executableRefusedBeforeStart || ctx.Err() != nil || authority.life.Err() != nil {
					_ = attempt.stop()
					return attempt, fmt.Errorf("helper descendant PID was not published: early exit=%d failure=%#v", attempt.result.exit, attempt.failure)
				}
				renewed, err := requalifyProcessTestAuthority(setup, original)
				_ = attempt.stop()
				if err != nil {
					return attempt, err
				}
				authority = renewed
				select {
				case <-setup.Done():
					authority.Close()
					return attempt, errors.New("helper descendant PID was not published")
				case <-time.After(10 * time.Millisecond):
				}
				goto retry
			case <-setup.Done():
				if err := attempt.stop(); err != nil {
					return attempt, err
				}
				return attempt, errors.New("helper descendant PID was not published")
			case <-time.After(time.Millisecond):
			}
		}
	retry:
	}
	if authority != original {
		authority.Close()
	}
	return nil, errors.New("helper descendant PID was not published")
}

func requalifyProcessTestAuthority(ctx context.Context, original *Authority) (*Authority, error) {
	for ctx.Err() == nil {
		file, evidence, failure := openExecutableStableRead(ctx, original.executable)
		if failure == nil {
			if evidence != original.executableBefore {
				_ = file.Close()
				return nil, errors.New("process helper executable changed during acquisition")
			}
			life, cancel := context.WithCancel(original.budget.context)
			return &Authority{
				life: life, cancel: cancel, budget: original.budget,
				executable: original.executable, executableFile: file, executableBefore: evidence,
				environment: append([]string(nil), original.environment...),
			}, nil
		}
		if ctx.Err() != nil {
			return nil, errors.New("helper descendant PID was not published")
		}
		if failure.Code != FailureRepositoryChanged {
			return nil, fmt.Errorf("process helper requalification failed: %#v", failure)
		}
		time.Sleep(time.Millisecond)
	}
	return nil, errors.New("helper descendant PID was not published")
}

func readHelperPIDs(t *testing.T, path string) []int {
	t.Helper()
	pids := readHelperPIDsIfPresent(path)
	if len(pids) == 0 {
		t.Fatal("helper published no descendant PID")
	}
	return pids
}

func readHelperPIDsIfPresent(path string) []int {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 || body[len(body)-1] != '\n' {
		return nil
	}
	var result []int
	for _, field := range strings.Split(string(body[:len(body)-1]), "\n") {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 0 || strconv.Itoa(pid) != field {
			return nil
		}
		result = append(result, pid)
	}
	return result
}

func assertProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("process %d survived containment cleanup", pid)
}
