//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

func psrWaitFile(t *testing.T, p string) {
	t.Helper()
	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		if _, e := os.Stat(p); e == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("command fixture not reached: %s", p)
}
func psrNativeConfig(root, state, script string) *dispatch.Config {
	return &dispatch.Config{Profile: dispatch.ConfigProfile, StateDir: state, WorkRoot: root, TickSeconds: 1, GlobalCap: 2, KillGraceSeconds: 1,
		PoolSweep: &dispatch.PoolSweepConfig{TimeoutSeconds: 15, IntervalSeconds: 1},
		Hosts:     map[string]dispatch.Host{"sh": {Argv: []string{"/bin/sh", "-c", script}}},
		Roles:     []dispatch.Role{{Name: "impl", Host: "sh", Cap: 1, Match: &dispatch.Match{IDGlob: "never"}, Prompt: "work", IdleSeconds: 30, WallSeconds: 60}},
		Backoff:   dispatch.Backoff{CooldownSeconds: 3600, ParkAfter: 2}}
}
func TestPSRDispatchNativeAdapter(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	t.Run("bounded-native-writer-join", func(t *testing.T) {
		scratch := t.TempDir()
		marker := filepath.Join(scratch, "started")
		repo, root := psrCLIReady(t, "echo reached > '"+marker+"'; sleep 60")
		c := psrNativeConfig(root, filepath.Join(scratch, "dispatch"), "true")
		d, e := dispatch.Open("bound", c, dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}, io.Discard)
		if e != nil {
			t.Fatal(e)
		}
		defer d.Close()
		if e = d.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		psrWaitFile(t, marker)
		lock, e := authority.AcquireLock(context.Background(), repo, authority.LockOptions{Wait: time.Second})
		if e != nil {
			t.Fatal(e)
		}
		defer lock.Close()
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- d.Close() }()
		select {
		case <-done:
		case <-time.After(38 * time.Second):
			// Release the fixture obstruction, then join the actual operation before failing.
			_ = lock.Close()
			<-done
			t.Fatal("native cancellation exceeded bound while writer lock held")
		}
		if elapsed := time.Since(start); elapsed > 38*time.Second {
			t.Fatal(elapsed)
		}
		raw, e := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
		if e != nil {
			t.Fatal(e)
		}
		pools, e := snapshot.DecodePools(raw)
		if e != nil || len(pools.Entries) != 1 || pools.Entries[0].State == "FREE" {
			t.Fatal("blocked publication freed allocation", pools, e)
		}
		_ = lock.Close()
		// The returned operation released the dispatcher lock; persisted state remains readable.
		again, e := dispatch.Open("bound", c, dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}, io.Discard)
		if e != nil {
			t.Fatal(e)
		}
		if e = again.Tick(context.Background()); e != nil {
			t.Fatal(e)
		}
		if e = again.Close(); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("compiled-worker-overlap", func(t *testing.T) {
		_ = psrBinary(t)
		scratch := t.TempDir()
		worker := filepath.Join(scratch, "worker")
		overlap := filepath.Join(scratch, "overlap")
		repo, root := psrCLIReady(t, "i=0; while [ ! -f '"+worker+"' ] && [ $i -lt 100 ]; do sleep .02; i=$((i+1)); done; kill -0 $(cat '"+worker+"') || exit 7; echo overlap > '"+overlap+"'; printf reset")
		c := psrNativeConfig(root, filepath.Join(scratch, "dispatch"), "echo $$ > '"+worker+"'; sleep 2")
		c.Roles[0].Match = &dispatch.Match{}
		raw, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		config := filepath.Join(scratch, "config.json")
		if e = os.WriteFile(config, raw, 0600); e != nil {
			t.Fatal(e)
		}
		out := psrBinaryCall(t, root, "dispatch", "--program", "native", "--config", config, "--ticks", "5")
		if string(out.Outcome) != "OK" {
			t.Fatal(out)
		}
		psrWaitFile(t, overlap)
		raw, e = os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
		if e != nil {
			t.Fatal(e)
		}
		pools, e := snapshot.DecodePools(raw)
		if e != nil || len(pools.Entries) != 0 {
			t.Fatal(pools, e)
		}
		ledger, e := dispatch.LoadLedger(dispatch.ProgramDir(c, "native"), "native")
		if e != nil {
			t.Fatal(e)
		}
		if len(ledger.Workers) != 0 || len(ledger.PoolSweeps) != 1 {
			t.Fatalf("workers or sweep missing: %+v", ledger)
		}
		for _, r := range ledger.PoolSweeps {
			if r.Phase != "TERMINAL" || r.Result.Receipt == "" {
				t.Fatalf("unpublished result: %+v", r)
			}
		}
		status := psrBinaryCall(t, root, "dispatch", "status", "--program", "native", "--config", config)
		if string(status.Outcome) != "OK" {
			t.Fatal(status)
		}
	})
}
