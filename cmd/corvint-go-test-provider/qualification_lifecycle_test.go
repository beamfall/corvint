//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/processidentity"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

type observedProcess struct {
	PID   int
	Start string
}
type processRendezvous struct {
	PGID      int
	Processes []observedProcess
}

// A known-live child outside the run is a sensitivity control, never an escaped
// descendant. Cleanup runs on assertion failure AND SIGINT/SIGTERM; t.Cleanup
// alone cannot intercept a process signal.
func qualificationControl(t *testing.T) (context.Context, observedProcess) {
	t.Helper()
	ctx, cancel := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	command := exec.Command("/bin/sleep", "600")
	command.Env = []string{}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	var once sync.Once
	cleanup := func() {
		once.Do(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL); _ = command.Wait(); cancel() })
	}
	t.Cleanup(cleanup)
	go func() { <-ctx.Done(); cleanup() }()
	start, err := processidentity.Start(ctx, command.Process.Pid)
	if err != nil {
		t.Skipf("INCONCLUSIVE: record control identity: %v", err)
	}
	return ctx, observedProcess{PID: command.Process.Pid, Start: start}
}

func probeRecordedProcess(ctx context.Context, process observedProcess) (bool, error) {
	// ESRCH is conclusive for this PID, independently of the group probe. Never
	// interpret an unreadable start time from a live PID as disappearance.
	err := syscall.Kill(process.PID, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	start, err := processidentity.Start(ctx, process.PID)
	if err != nil {
		return false, err
	}
	return start == process.Start, nil
}

func TestExecuteNormalCompletionHasNoRecordedSurvivors(t *testing.T) {
	verifyExecuteRecordedCleanup(t, false)
}

func TestExecuteInterruptedHasNoRecordedSurvivors(t *testing.T) {
	verifyExecuteRecordedCleanup(t, true)
}

func verifyExecuteRecordedCleanup(t *testing.T, interrupted bool) {
	t.Helper()
	bundle, _ := providerE2EBundle(t)
	rendezvous := resolvedTemp(t)
	source := fmt.Sprintf(normalCompletionSource, rendezvous)
	if err := os.WriteFile(filepath.Join(bundle.RepositoryRoot, "cli_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, bundle.GitExecutable, bundle.RepositoryRoot, "add", "cli_test.go")
	runFixtureGit(t, bundle.GitExecutable, bundle.RepositoryRoot, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "process observation fixture")
	controlContext, control := qualificationControl(t)
	ctx, cancelRun := context.WithCancel(controlContext)
	defer cancelRun()
	type handshake struct {
		record processRendezvous
		err    error
	}
	ready := make(chan handshake, 1)
	observerContext, stopObserver := context.WithCancel(ctx)
	defer stopObserver()
	// Execute waits on the helper, which waits on us: start this before Execute.
	go func() {
		deadline := time.NewTimer(4 * time.Minute)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-observerContext.Done():
				ready <- handshake{err: observerContext.Err()}
				return
			case <-deadline.C:
				cancelRun()
				ready <- handshake{err: errors.New("handshake exceeded four minutes")}
				return
			case <-tick.C:
				raw, err := os.ReadFile(filepath.Join(rendezvous, "ready"))
				if os.IsNotExist(err) {
					continue
				}
				var record processRendezvous
				if err == nil {
					err = json.Unmarshal(raw, &record)
				}
				if err == nil {
					if interrupted {
						cancelRun()
					} else {
						err = os.WriteFile(filepath.Join(rendezvous, "unblock"), []byte("observed"), 0600)
					}
				}
				ready <- handshake{record: record, err: err}
				return
			}
		}
	}()
	started := time.Now()
	transcript, err := executeProvider(ctx, bundle)
	wall := time.Since(started)
	stopObserver()
	observation := <-ready
	if observation.err != nil {
		if raw, readErr := os.ReadFile(filepath.Join(rendezvous, "probe-error")); readErr == nil {
			t.Skipf("INCONCLUSIVE: helper PS_LSTART recording: %s", raw)
		}
		t.Fatalf("handshake: %v; Execute: %v", observation.err, err)
	}
	record := observation.record
	// Fallback is containment on a failing test, never evidence for the assertions.
	t.Cleanup(func() {
		for _, process := range record.Processes {
			live, _ := probeRecordedProcess(context.Background(), process)
			if live {
				_ = syscall.Kill(process.PID, syscall.SIGKILL)
			}
		}
	})
	if wall >= 5*time.Minute {
		t.Skipf("INCONCLUSIVE: Execute wall time %s exceeds identity lifetime budget", wall)
	}
	if err != nil || transcript.RunnerError != nil || transcript.CleanupError != nil || !transcript.Runner.ProcessCleanupDone || transcript.Runner.Cancelled != interrupted || transcript.Runner.TimedOut || !transcript.EphemeralDeletionComplete {
		t.Fatalf("normal clean completion precondition: Execute=%v runner=%v cleanup=%v execution=%s containment=%t stdout=%s stderr=%s", err, transcript.RunnerError, transcript.CleanupError, transcript.Execution, transcript.Runner.ProcessCleanupDone, transcript.Runner.Stdout.Data, transcript.Runner.Stderr.Data)
	}
	var terminal map[string]any
	if err := json.Unmarshal(transcript.Receipt.Run, &terminal); err != nil {
		t.Fatal(err)
	}
	if terminal["ephemeralDeletion"] != "COMPLETE" {
		t.Fatalf("wire deletion = %v", terminal["ephemeralDeletion"])
	}
	wantExecution := provider.ExecutionPassed
	if interrupted {
		wantExecution = provider.ExecutionIncomplete
	}
	if transcript.Execution != wantExecution {
		t.Fatalf("execution = %s; want %s", transcript.Execution, wantExecution)
	}
	if record.PGID <= 0 || len(record.Processes) != 2 || record.Processes[0].PID == record.Processes[1].PID {
		t.Fatalf("handshake population = %+v", record)
	}
	t.Logf("GLTP-V0-041 normal-completion=%t identity=%s ExecuteWall=%s population=2 control=1", !interrupted, processidentity.Source, wall)
	probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	live, probeErr := probeRecordedProcess(probeCtx, control)
	if probeErr != nil {
		t.Skipf("INCONCLUSIVE: control identity probe: %v", probeErr)
	}
	if !live {
		t.Fatal("probe failed to see known-live out-of-run control")
	}
	groupErr := syscall.Kill(-record.PGID, 0)
	t.Logf("additional process-group probe: %v", groupErr)
	for _, process := range record.Processes {
		if process.PID <= 0 || process.Start == "" {
			t.Fatalf("invalid recorded identity: %+v", process)
		}
		live, probeErr := probeRecordedProcess(probeCtx, process)
		if errors.Is(probeErr, processidentity.ErrObservation) {
			t.Skipf("INCONCLUSIVE: first PS_LSTART probe: %v", probeErr)
		}
		if probeErr != nil {
			t.Fatalf("unresolved first identity probe for %d: %v", process.PID, probeErr)
		}
		if live {
			t.Fatalf("surviving recorded process %d (%s)", process.PID, process.Start)
		}
	}
	if probeCtx.Err() != nil {
		t.Fatal("probe exceeded five seconds")
	}
	verifyWithIndependentConformance(t, transcript)
}

func TestQualificationControlHelper(t *testing.T) {
	path := os.Getenv("CORVINT_CONTROL_MARKER")
	if path == "" {
		return
	}
	ctx, process := qualificationControl(t)
	raw, err := json.Marshal(process)
	if err != nil {
		t.Fatal(err)
	}
	pending := path + ".pending"
	if err := os.WriteFile(pending, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(pending, path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CORVINT_CONTROL_FAIL") == "1" {
		t.Fatal("deliberate fixture failure")
	}
	<-ctx.Done()
}

func TestQualificationControlCleanupOnFailureAndInterruption(t *testing.T) {
	if _, err := processidentity.Start(t.Context(), os.Getpid()); err != nil {
		t.Skipf("INCONCLUSIVE: process identity probe unavailable: %v", err)
	}
	for _, mode := range []string{"failure", "interrupt"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(resolvedTemp(t), "control")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(executable, "-test.run=^TestQualificationControlHelper$")
			command.Env = []string{"CORVINT_CONTROL_MARKER=" + marker}
			if mode == "failure" {
				command.Env = append(command.Env, "CORVINT_CONTROL_FAIL=1")
			}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			t.Cleanup(func() {
				_ = command.Process.Signal(syscall.SIGTERM)
				select {
				case <-done:
				case <-time.After(time.Second):
					_ = command.Process.Kill()
				}
			})
			deadline := time.Now().Add(5 * time.Second)
			var process observedProcess
			for {
				raw, readErr := os.ReadFile(marker)
				if readErr == nil {
					if err := json.Unmarshal(raw, &process); err != nil {
						t.Fatal(err)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("control marker missing")
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Cleanup(func() {
				live, _ := probeRecordedProcess(context.Background(), process)
				if live {
					_ = syscall.Kill(process.PID, syscall.SIGKILL)
				}
			})
			if mode == "interrupt" {
				if err := command.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if mode == "failure" && err == nil {
					t.Fatal("deliberate failure succeeded")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("control owner did not exit")
			}
			live, err := probeRecordedProcess(context.Background(), process)
			if err != nil || live {
				t.Fatalf("control leaked after %s: live=%t err=%v", mode, live, err)
			}
		})
	}
}

const normalCompletionSource = `package cli
import (
 "context"
 "encoding/json"
 "fmt"
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
func TestCLI(t *testing.T) {
 directory:=%q
 child:=exec.Command("/bin/sleep","600")
 child.Env=[]string{}
 if err:=child.Start();err!=nil {t.Fatal(err)}
 // The child deliberately remains in the inherited group and has no run pipes.
 // On success the runner owns teardown; on fixture failure this defer contains it.
 completed:=false
 defer func(){if !completed {_ = child.Process.Kill(); _ = child.Wait()}}()
 start:=func(pid int)string{
  if runtime.GOOS=="linux" {
   raw,err:=os.ReadFile(fmt.Sprintf("/proc/%%d/stat",pid));if err!=nil {t.Fatal(err)}
   end:=strings.LastIndexByte(string(raw),')');if end<0 {t.Fatal("bad proc stat")}
   fields:=strings.Fields(string(raw[end+1:]));if len(fields)<20 {t.Fatal("short proc stat")}
   return fields[19]
  }
  ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
  command:=exec.CommandContext(ctx,"/bin/ps","-p",strconv.Itoa(pid),"-o","lstart=")
  command.Env=[]string{"LC_ALL=C","TZ=UTC"}
  command.WaitDelay=250*time.Millisecond
  raw,err:=command.Output();if err!=nil {_ = os.WriteFile(filepath.Join(directory,"probe-error"),[]byte(err.Error()),0600);t.Fatal(err)}
  return strings.TrimSpace(string(raw))
 }
 pgid,err:=syscall.Getpgid(0);if err!=nil {t.Fatal(err)}
 childPGID,err:=syscall.Getpgid(child.Process.Pid);if err!=nil || childPGID!=pgid {t.Fatal("child did not inherit run group")}
 type identity struct {PID int;Start string}
 record:=struct{PGID int;Processes []identity}{pgid,[]identity{{os.Getpid(),start(os.Getpid())},{child.Process.Pid,start(child.Process.Pid)}}}
 raw,err:=json.Marshal(record);if err!=nil {t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(directory,"pending"),raw,0600);err!=nil {t.Fatal(err)}
 if err:=os.Rename(filepath.Join(directory,"pending"),filepath.Join(directory,"ready"));err!=nil {t.Fatal(err)}
 deadline:=time.Now().Add(10*time.Minute)
 for {
  if _,err:=os.Stat(filepath.Join(directory,"unblock"));err==nil {completed=true;return}
  if time.Now().After(deadline) {t.Fatal("observer never acknowledged identities")}
  time.Sleep(10*time.Millisecond)
 }
}
`
