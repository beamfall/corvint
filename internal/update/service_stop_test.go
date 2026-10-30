package update

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func touch(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// V1-3011: an operation cancelled after the service stopped and before the
// switch still starts the old build, under its own bounded context, and a
// restart failure is kept in the outcome.
func TestUPDV0015CancelledRunRestartsOldService(t *testing.T) {
	for _, startFails := range []bool{false, true} {
		e, dir, dest := serviceEngine(t)
		touch(t, dir, map[string]string{"running": ""})
		ctx, cancel := context.WithCancel(context.Background())
		e.beforeActivate = func() {
			if startFails {
				touch(t, dir, map[string]string{"start-fails": ""})
			}
			cancel()
		}
		r, err := e.execute(ctx, "apply")
		cancel()
		if err == nil || r.Switched || r.Service == nil {
			t.Fatalf("cancelled apply: %+v %v", r, err)
		}
		if d, _ := digest(dest); d != hash(scriptBinary(162, 0)) {
			t.Fatal("destination changed")
		}
		if startFails {
			if r.Service.State != "FAILED" || !strings.Contains(r.Service.Reason, "old build restart: service start failed") || !strings.Contains(r.Service.Reason, "destination unchanged") {
				t.Fatalf("restart failure dropped: %+v", r.Service)
			}
			continue
		}
		if r.Service.State != "RESUMED_OLD" || !exists(filepath.Join(dir, "running")) || !strings.Contains(readLog(t, dir), "start Corvint 1.0.0-rc.1 (build 162)") {
			t.Fatalf("old service not restarted: %+v\n%s", r.Service, readLog(t, dir))
		}
	}
}

// V1-3011: a drain interrupted by cancellation restarts the old build under the
// same cleanup context.
func TestUPDV0015CancelledDrainRestartsOldService(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	e.config.Component = "tasks"
	e.config.DrainTimeout = 10 * time.Second
	touch(t, dir, map[string]string{"running": "", "live": "3\n"})
	s := service{program: e.config.ServiceProgram, component: "tasks", binary: dest, poll: e.servicePoll}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		tick := time.NewTicker(2 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				log := readLog(t, dir)
				if i := strings.Index(log, "\nstop "); i >= 0 && strings.Contains(log[i:], "\nlive-attempts ") {
					cancel()
					return
				}
			}
		}
	}()
	out, err := e.quiesce(ctx, s, "DRAIN")
	if err == nil || out.State != "RESUMED_OLD" || !exists(filepath.Join(dir, "running")) {
		t.Fatalf("cancelled drain: %+v %v\n%s", out, err, readLog(t, dir))
	}
}

// V1-3012: a zero exit from stop is not a stopped service; status is polled
// within the bound before the switch.
func TestUPDV0015StopObservedBeforeSwitch(t *testing.T) {
	// Asynchronous shutdown that completes after three status reads: the
	// switch waits for it.
	e, dir, dest := serviceEngine(t)
	touch(t, dir, map[string]string{"running": "", "slow-stop": "3"})
	r, err := e.execute(context.Background(), "apply")
	if err != nil || r.Service.State != "RESUMED" || !r.Switched {
		t.Fatalf("delayed stop: %+v %+v %v", r, r.Service, err)
	}
	log := readLog(t, dir)
	stop, start := strings.Index(log, "stop Corvint 1.0.0-rc.1 (build 162)"), strings.Index(log, "start Corvint 1.0.0-rc.1 (build 163)")
	if stop < 0 || start < stop || strings.Count(log[stop:start], "status Corvint 1.0.0-rc.1 (build 162)") < 4 {
		t.Fatalf("switch before stop was observed:\n%s", log)
	}
	// Still running at the bound, or unknown after stop: refused, destination
	// unchanged and the old build resumed.
	for name, files := range map[string]map[string]string{
		"running at the bound": {"running": "", "slow-stop": "100000"},
		"unknown after stop":   {"running": "", "broken-after-stop": ""},
	} {
		e, dir, dest = serviceEngine(t)
		touch(t, dir, files)
		r, err = e.execute(context.Background(), "apply")
		if err == nil || r.Switched || r.Service.State != "RESUMED_OLD" || !strings.Contains(r.Service.Reason, "destination unchanged") || !exists(filepath.Join(dir, "running")) {
			t.Fatalf("%s: %+v %+v %v\n%s", name, r, r.Service, err, readLog(t, dir))
		}
		if d, _ := digest(dest); d != hash(scriptBinary(162, 0)) {
			t.Fatalf("%s: switched under a service not observed stopped", name)
		}
	}
}

// V1-3012: automatic rollback also needs an observed stop of the new build.
// V1-3033: a run cancelled during the automatic rollback still restores the
// previous build and restarts the service on it, keeping the start failure.
func TestUPDV0013CancelledRollbackStillRestores(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	touch(t, dir, map[string]string{"running": "", "fail-new": ""})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.beforeRollback = cancel
	r, err := e.execute(ctx, "apply")
	if err == nil || r.Switched || r.Action != "rolled-back" || r.Service.State != "ROLLED_BACK" || !strings.Contains(r.Service.Reason, "new build:") {
		t.Fatalf("cancelled rollback: %+v %+v %v\n%s", r, r.Service, err, readLog(t, dir))
	}
	if d, _ := digest(dest); d != hash(scriptBinary(162, 0)) {
		t.Fatal("previous build not restored after a cancelled rollback")
	}
	if !exists(filepath.Join(dir, "running")) {
		t.Fatal("service not running on the previous build")
	}
}

func TestUPDV0015StopObservedBeforeRollback(t *testing.T) {
	e, dir, dest := serviceEngine(t)
	touch(t, dir, map[string]string{"running": "", "fail-new": "", "slow-stop-new": "100000"})
	r, err := e.execute(context.Background(), "apply")
	if err == nil || !strings.Contains(err.Error(), "could not be stopped; the new build stays installed") || !r.Switched || r.Service.State != "FAILED" || !strings.Contains(r.Service.Reason, "still reports running after stop") {
		t.Fatalf("rollback under a running service: %+v %+v %v", r, r.Service, err)
	}
	if d, _ := digest(dest); d != hash(scriptBinary(163, 0)) {
		t.Fatal("previous build restored under a service not observed stopped")
	}
	// A delayed stop of the new build is waited for, then rolled back.
	e, dir, dest = serviceEngine(t)
	touch(t, dir, map[string]string{"running": "", "fail-new": "", "slow-stop-new": "3"})
	r, err = e.execute(context.Background(), "apply")
	if err == nil || r.Switched || r.Service.State != "ROLLED_BACK" || !exists(filepath.Join(dir, "running")) {
		t.Fatalf("delayed rollback stop: %+v %+v %v\n%s", r, r.Service, err, readLog(t, dir))
	}
	if d, _ := digest(dest); d != hash(scriptBinary(162, 0)) {
		t.Fatal("previous build not restored")
	}
}
