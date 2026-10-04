//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

func readerDispatcher(t *testing.T) (*Dispatcher, *cancelObservationQueue) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].Match.Labels = []string{"do-not-launch"}
	q := &cancelObservationQueue{fakeQueue: &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}}
	d, err := Open("reader", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	return d, q
}

// Fixture initialization and the real RELEASED path run before any injected
// failure. Ordinary output errors still clear only after Owner collection.
func TestCALV0053_ReaderReleased(t *testing.T) {
	d, _ := readerDispatcher(t)
	for _, tc := range []struct {
		name, script string
		wantError    bool
	}{
		{"json", `printf '{"t":"ready"}'`, false},
		{"nonzero", `exit 7`, true},
		{"malformed", `printf '{'`, true},
		{"overflow", `printf '%1048580s' x`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			starts := 0
			var owner *groupreap.Owner
			d.readerStart = func(cmd *exec.Cmd) (*groupreap.Owner, error) {
				starts++
				if _, err := readReaderMarker(d.dir, d.Program); err != nil {
					t.Errorf("marker absent before Start: %v", err)
				}
				var err error
				owner, err = groupreap.Start(cmd)
				return owner, err
			}
			states, err := d.stateCommand(context.Background(), d.Config.WorkRoot, []string{"/bin/sh", "-c", tc.script})
			if (err != nil) != tc.wantError || starts != 1 || owner == nil || owner.State() != groupreap.Released || d.reader != nil || d.readerErr != nil {
				t.Fatalf("starts=%d owner=%v states=%v err=%v held=%v", starts, owner, states, err, d.readerErr)
			}
			if _, err := os.Lstat(filepath.Join(d.dir, readerMarkerName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("marker not cleared: %v", err)
			}
			if !tc.wantError && states["t"].State != "ready" {
				t.Fatal(states)
			}
		})
	}
	d.readerStart = nil
	if _, err := d.stateCommand(context.Background(), d.Config.WorkRoot, []string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing executable succeeded")
	}
	if _, err := os.Lstat(filepath.Join(d.dir, readerMarkerName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Start refusal retained owned marker", err)
	}
}

func TestCALV0053_ReaderFixedRetirementBound(t *testing.T) {
	now := time.Now()
	for _, deadline := range []time.Time{now.Add(time.Hour), now.Add(-time.Second / 2)} {
		got := readerRetirementDeadline(deadline, now)
		want := now.Add(time.Second)
		if deadline.Add(time.Second).Before(want) {
			want = deadline.Add(time.Second)
		}
		if !got.Equal(want) {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestCALV0053_ReaderExpiredBeforeRetirement(t *testing.T) {
	d, _ := readerDispatcher(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var cmd *exec.Cmd
	var primitives atomic.Int32
	var collected atomic.Bool
	t.Cleanup(func() {
		if cmd != nil && !collected.Load() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	d.readerStart = func(c *exec.Cmd) (*groupreap.Owner, error) {
		cmd = c
		owner, err := groupreap.StartWith(c, groupreap.Primitives{
			KillGroup: func(int) error { primitives.Add(1); return errors.New("must not signal after expiry") },
			ProbeGroup: func(int) (groupreap.Probe, error) {
				primitives.Add(1)
				return groupreap.ProbeLive, errors.New("must not probe after expiry")
			},
			Reap: func(c *exec.Cmd) error { primitives.Add(1); err := c.Wait(); collected.Store(true); return err },
		})
		if err == nil {
			deadline, _ := ctx.Deadline()
			time.Sleep(time.Until(deadline.Add(time.Second + 50*time.Millisecond)))
		}
		return owner, err
	}
	_, err := d.stateCommand(ctx, d.Config.WorkRoot, []string{"/bin/sleep", "5"})
	if !errors.Is(err, ErrReaderQuiescence) || primitives.Load() != 0 || d.reader == nil || d.reader.result.State != groupreap.Hold || !d.reader.bound.Expired() {
		t.Fatalf("expired retirement admitted work: %v calls=%d", err, primitives.Load())
	}
}

// Faults are injected through the actual Owner, never a fabricated result.
// The fixture owns cleanup outside the product: pre-reap faults collect the
// already exited direct child once; pending reap is released and joined once.
func TestCALV0053_ReaderHoldIsSticky(t *testing.T) {
	for _, stage := range []string{"observe", "signal", "probe", "reap", "pending-reap", "post-reap"} {
		t.Run(stage, func(t *testing.T) {
			d, q := readerDispatcher(t)
			before, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			seen := *d.ledger.Seen
			seq := d.LastEvent()
			reads := q.reads
			var output bytes.Buffer
			d.Out = &output
			var cmd *exec.Cmd
			var collected atomic.Bool
			joined := make(chan struct{})
			unblock := make(chan struct{})
			reapEntered := make(chan struct{})
			finished := make(chan struct{})
			var unblockOnce sync.Once
			var started, signals, reaps, probes atomic.Int32
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d.readerStart = func(c *exec.Cmd) (*groupreap.Owner, error) {
				cmd = c
				started.Add(1)
				p := groupreap.Primitives{RetirementMode: groupreap.ReapAfterSuccessfulSignal}
				switch stage {
				case "observe":
					p.WaitExit = func(int) error { return errors.New("injected observe") }
				case "signal":
					p.KillGroup = func(int) error { signals.Add(1); return errors.New("injected signal") }
				case "probe":
					p.ProbeGroup = func(int) (groupreap.Probe, error) {
						probes.Add(1)
						return groupreap.ProbeLive, errors.New("injected probe")
					}
				case "reap":
					p.Reap = func(*exec.Cmd) error { reaps.Add(1); return errors.New("injected reap") }
				case "pending-reap":
					p.Reap = func(c *exec.Cmd) error {
						reaps.Add(1)
						close(reapEntered)
						<-unblock
						err := c.Wait()
						collected.Store(true)
						close(joined)
						return err
					}
				case "post-reap":
					p.ProbeGroup = func(int) (groupreap.Probe, error) { probes.Add(1); return groupreap.ProbeLive, nil }
				}
				if p.Reap == nil {
					p.Reap = func(c *exec.Cmd) error { reaps.Add(1); err := c.Wait(); collected.Store(true); return err }
				}
				return groupreap.StartWith(c, p)
			}
			d.Config.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/sh", "-c", `printf '{"t":"changed"}'`}}
			result := make(chan error, 1)
			t.Cleanup(func() {
				cancel()
				unblockOnce.Do(func() { close(unblock) })
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("fixture result join UNKNOWN")
					return
				}
				if cmd != nil && !collected.Load() {
					if stage == "pending-reap" {
						select {
						case <-joined:
						case <-time.After(5 * time.Second):
							t.Error("fixture reap join UNKNOWN")
						}
					} else {
						_ = cmd.Process.Kill()
						_ = cmd.Wait()
						collected.Store(true)
					}
				}
			})
			go func() { defer close(finished); result <- d.Run(ctx, 1) }()
			if stage == "pending-reap" {
				select {
				case <-reapEntered:
					cancel()
				case <-time.After(5 * time.Second):
					t.Fatal("injected reap not reached")
				}
			}
			select {
			case err := <-result:
				if !errors.Is(err, ErrReaderQuiescence) {
					t.Fatalf("lost HOLD: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("reader did not return bounded HOLD")
			}
			if started.Load() != 1 || d.reader == nil || d.reader.owner == nil || d.reader.result.State != groupreap.Hold {
				t.Fatal("real owner/result not retained")
			}
			// Retire the fixture using the still-owned command handle, never a
			// numeric group signal or a second Owner retirement allowance.
			if stage == "pending-reap" {
				unblockOnce.Do(func() { close(unblock) })
				select {
				case <-joined:
				case <-time.After(5 * time.Second):
					t.Fatal("fixture reap failed to join")
				}
			}
			if !collected.Load() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				collected.Store(true)
			}
			if stage == "signal" && signals.Load() != 1 || stage == "probe" && probes.Load() == 0 || stage == "reap" && reaps.Load() != 1 || stage == "post-reap" && probes.Load() == 0 {
				t.Fatal("injection not reached")
			}
			calls := []int32{started.Load(), signals.Load(), reaps.Load(), probes.Load()}
			cancel()
			if err := d.Run(ctx, 1); !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("cancel normalized HOLD", err)
			}
			if err := d.Tick(context.Background()); !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("later Tick lost HOLD", err)
			}
			if err := d.Close(); !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("Close lost HOLD", err)
			}
			if !reflect.DeepEqual(calls, []int32{started.Load(), signals.Load(), reaps.Load(), probes.Load()}) || d.reader.owner.State() != groupreap.Hold {
				t.Fatal("HOLD restarted lifecycle")
			}
			after, _ := os.ReadFile(filepath.Join(d.dir, "state.json"))
			if !bytes.Equal(before, after) || !reflect.DeepEqual(seen, *d.ledger.Seen) || d.LastEvent() != seq || q.reads != reads+1 || len(q.released)+len(q.reaped) != 0 || d.Running() != 0 {
				t.Fatal("HOLD published observation/accounting/native effects")
			}
			if strings.Contains(output.String(), "left running for the next dispatcher") || !strings.Contains(output.String(), "QUIESCENCE_UNPROVED") {
				t.Fatal(output.String())
			}
			if _, err := Open(d.Program, d.Config, q, io.Discard); !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("restart not refused", err)
			}
		})
	}
}

func TestCALV0053_ReaderMarkerRefusesUnsafeEvidence(t *testing.T) {
	for _, kind := range []string{"valid", "malformed", "duplicate", "symlink", "directory", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			c := testConfig(t, "exit 0")
			dir := ProgramDir(c, "quarantine")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, readerMarkerName)
			switch kind {
			case "valid":
				if _, err := publishReaderMarker(dir, "quarantine"); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				if err := os.WriteFile(path, []byte(`{"profile":"taskman-dispatch-reader-lifecycle/0","program":"quarantine","run":"00000000000000000000000000000000","lifecycle":"UNKNOWN","lifecycle":"UNKNOWN"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("missing", path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			q := &cancelObservationQueue{fakeQueue: &fakeQueue{}}
			if _, err := Open("quarantine", c, q, io.Discard); !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("unsafe marker admitted", err)
			}
			for _, name := range []string{"requests", "state.json", "events.jsonl"} {
				if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("Open created", name, err)
				}
			}
			lock, err := os.ReadFile(filepath.Join(dir, "lock"))
			if err != nil || len(lock) != 0 || q.reads != 0 {
				t.Fatal("Open wrote owner or observed queue")
			}
			state, held, _ := ReaderContainment(dir, "quarantine")
			if state != "UNKNOWN" || !held {
				t.Fatal("status called unsafe evidence clean")
			}
		})
	}
}

func TestCALV0053_ReaderMarkerIdentityAndSetup(t *testing.T) {
	d, _ := readerDispatcher(t)
	mark, err := publishReaderMarker(d.dir, d.Program)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishReaderMarker(d.dir, d.Program); !errors.Is(err, ErrReaderQuiescence) {
		t.Fatal("replaced another run", err)
	}
	if err := os.Rename(mark.path, mark.path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mark.path, mark.raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := clearReaderMarker(mark, d.Program); err == nil {
		t.Fatal("removed different inode")
	}
	if _, err := os.Stat(mark.path); err != nil {
		t.Fatal("replacement lost")
	}
	starts := 0
	d.readerStart = func(*exec.Cmd) (*groupreap.Owner, error) { starts++; return nil, errors.New("must not start") }
	if _, err := d.stateCommand(context.Background(), d.Config.WorkRoot, []string{"/bin/true"}); !errors.Is(err, ErrReaderQuiescence) || starts != 0 {
		t.Fatal("marker setup did not refuse before Start", err, starts)
	}
}

// This is the real sixty-second reader-local deadline, with no outer deadline.
func TestCALV0053_ReaderLocalTimeout(t *testing.T) {
	d, _ := readerDispatcher(t)
	d.Config.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/sleep", "120"}}
	started := time.Now()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < stateTimeout || elapsed > stateTimeout+10*time.Second {
		t.Fatalf("not local timeout: %v", elapsed)
	}
	if !strings.Contains(d.ledger.Seen.Tickets["ticket:a:q:t"], "|UNKNOWN|") || d.readerErr != nil {
		t.Fatal("local timeout did not remain ordinary reader failure")
	}
}

func TestCALV0053_ReaderClearFailureQuarantines(t *testing.T) {
	d, _ := readerDispatcher(t)
	d.readerStart = func(cmd *exec.Cmd) (*groupreap.Owner, error) {
		return groupreap.StartWith(cmd, groupreap.Primitives{Reap: func(c *exec.Cmd) error {
			err := c.Wait()
			// Replace evidence after actual collection, before result handling.
			if e := os.WriteFile(filepath.Join(d.dir, readerMarkerName), []byte("changed"), 0600); e != nil {
				return e
			}
			return err
		}})
	}
	if _, err := d.stateCommand(context.Background(), d.Config.WorkRoot, []string{"/bin/sh", "-c", `printf '{"t":"ready"}'`}); !errors.Is(err, ErrReaderQuiescence) {
		t.Fatal("clear drift became success", err)
	}
	if d.reader == nil || d.reader.result.State != groupreap.Released {
		t.Fatal("fixture did not reach RELEASED")
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, readerMarkerName))
	if err != nil || string(raw) != "changed" {
		t.Fatal("foreign evidence deleted", err)
	}
}

func TestCALV0053_ReaderFilesystemFailures(t *testing.T) {
	for _, stage := range []string{"publish", "remove"} {
		t.Run(stage, func(t *testing.T) {
			d, _ := readerDispatcher(t)
			defer os.Chmod(d.dir, 0700)
			starts := 0
			d.readerStart = func(cmd *exec.Cmd) (*groupreap.Owner, error) {
				starts++
				return groupreap.StartWith(cmd, groupreap.Primitives{Reap: func(c *exec.Cmd) error {
					err := c.Wait()
					if e := os.Chmod(d.dir, 0500); e != nil {
						return e
					}
					return err
				}})
			}
			if stage == "publish" {
				if err := os.Chmod(d.dir, 0500); err != nil {
					t.Fatal(err)
				}
			}
			_, err := d.stateCommand(context.Background(), d.Config.WorkRoot, []string{"/bin/sh", "-c", `printf '{}'`})
			if !errors.Is(err, ErrReaderQuiescence) {
				t.Fatal("filesystem refusal lost; qualification requires unprivileged uid", err)
			}
			if stage == "publish" && starts != 0 {
				t.Fatal("child started after publication failure")
			}
			if stage == "remove" {
				if starts != 1 || d.reader == nil || d.reader.result.State != groupreap.Released {
					t.Fatal("removal failure fixture did not reach released result")
				}
				if _, err := readReaderMarker(d.dir, d.Program); err != nil {
					t.Fatal("failed removal did not preserve marker", err)
				}
			}
		})
	}
}

type failingReaderDiagnostic struct{ writes int }

func (w *failingReaderDiagnostic) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("injected diagnostic persistence failure")
}

func TestCALV0053_ReaderFailedDiagnostic(t *testing.T) {
	d, _ := readerDispatcher(t)
	mark, err := publishReaderMarker(d.dir, d.Program)
	if err != nil {
		t.Fatal(err)
	}
	writer := &failingReaderDiagnostic{}
	d.Out = writer
	seq := d.LastEvent()
	before, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	d.poisonReader(errors.New("fixture terminal containment refusal"))
	if err := d.Close(); !errors.Is(err, ErrReaderQuiescence) || writer.writes != 1 {
		t.Fatal("diagnostic failure replaced HOLD", err, writer.writes)
	}
	after, err := os.ReadFile(mark.path)
	if err != nil || !bytes.Equal(after, mark.raw) {
		t.Fatal("failed diagnostic lost pre-spawn marker", err)
	}
	state, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil || !bytes.Equal(before, state) || d.LastEvent() != seq {
		t.Fatal("failed diagnostic changed ledger")
	}
}

// Every descendant identity is recorded before the leader's release trigger.
// The qualification host must reap orphaned zombies; a container without a
// real init will fail absence qualification rather than count compilation.
func TestCALV0053_ReaderDescendants(t *testing.T) {
	for _, mode := range []string{"cancel", "success", "nonzero"} {
		t.Run(mode, func(t *testing.T) {
			d, _ := readerDispatcher(t)
			dir := d.Config.WorkRoot
			ready := filepath.Join(dir, "identities")
			release := filepath.Join(dir, "release")
			script := fmt.Sprintf(`sleep 120 & child=$!; sh -c 'sleep 120 & echo "$!" > grandchild; wait' & middle=$!; while [ ! -f grandchild ]; do sleep .01; done; echo "$$ $child $middle $(cat grandchild)" > %q; while [ ! -f %q ]; do sleep .01; done; printf '{"t":"ready"}'; %s`, ready, release, map[string]string{"cancel": "wait", "success": "exit 0", "nonzero": "exit 7"}[mode])
			d.Config.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/sh", "-c", script}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			finished := make(chan struct{})
			var pids []int
			var ids []string
			t.Cleanup(func() {
				cancel()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("descendant fixture result join UNKNOWN")
					return
				}
				for i, pid := range pids {
					if i >= len(ids) {
						continue
					}
					if id, _ := identityOf(pid); id != "" && id == ids[i] {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
					if !gone(pid) {
						t.Error("descendant fixture cleanup UNKNOWN", pid)
					}
				}
			})
			go func() { defer close(finished); result <- d.Tick(ctx) }()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				raw, err := os.ReadFile(ready)
				if err == nil {
					var a, b, c, e int
					if n, _ := fmt.Sscanf(string(raw), "%d %d %d %d", &a, &b, &c, &e); n == 4 {
						pids = []int{a, b, c, e}
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if len(pids) != 4 {
				cancel()
				select {
				case <-result:
				case <-time.After(5 * time.Second):
				}
				t.Fatal("descendant fixture not initialized")
			}
			for _, pid := range pids {
				id, err := identityOf(pid)
				if err != nil || id == "" {
					t.Fatal("fixture identity unavailable", pid, err)
				}
				ids = append(ids, id)
			}
			if mode == "cancel" {
				cancel()
			} else if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if mode == "cancel" {
					if !errors.Is(err, context.Canceled) {
						t.Error(err)
					}
				} else if err != nil {
					t.Error(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("descendants outlived reader bound")
			}
			for _, pid := range pids {
				if !gone(pid) {
					t.Error("fixture process remains", pid)
				}
			}
			if d.readerErr != nil {
				t.Fatal("containment not qualified", d.readerErr)
			}
			if mode == "success" && !strings.Contains(d.ledger.Seen.Tickets["ticket:a:q:t"], "|ready|") {
				t.Fatal("valid state not accepted")
			}
		})
	}
}
