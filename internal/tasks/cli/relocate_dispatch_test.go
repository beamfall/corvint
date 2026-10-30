//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TRL-V0-003: with --dispatch-config, relocate takes every program's
// single-dispatcher lock without waiting; a lock another holder keeps refuses
// QUIESCENCE_UNPROVED and leaves the store unchanged, and once it is free the
// relocation commits with the dispatcher check observed.
func TestTRLV0003_RelocateRefusesHeldDispatcher(t *testing.T) {
	r := receiptFixture(t)
	old := r.Root
	moved := moveRepo(t, old, "moved")
	state := filepath.Join(moved, ".git", "taskman")
	dispatchState, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": dispatchState, "workRoot": moved,
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts":   map[string]any{"sh": map[string]any{"argv": []string{"/bin/sh", "-c", "exit 0"}}},
		"roles":   []any{map[string]any{"name": "impl", "host": "sh", "cap": 1, "match": map[string]any{"labels": []string{"never-launch"}}, "prompt": "work", "idleSeconds": 30, "wallSeconds": 60}},
		"backoff": map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":    map[string]any{"handoff": true, "reap": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(filepath.Dir(moved), "dispatch.json")
	fixture.Write(t, cfg, raw)
	if err := os.MkdirAll(filepath.Join(dispatchState, "prog"), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(dispatchState, "prog", "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	before := fixture.TreeSnapshot(t, state)
	x := atm(t, moved, nil, "relocate", "--from", old, "--request-id", "r", "--dispatch-config", cfg)
	if !hasCode(x.res, wire.CodeQuiescenceUnproved) {
		t.Fatalf("held dispatcher: %s", x.stdout)
	}
	if after := fixture.TreeSnapshot(t, state); len(after) != len(before) || headPrimary(t, state) != old {
		t.Fatal("refused relocation changed the store")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	x = atm(t, moved, nil, "relocate", "--from", old, "--request-id", "r", "--dispatch-config", cfg)
	if x.res.Outcome != wire.OutcomeOK || strings.Contains(strings.Join(x.res.Warnings, " "), "NOT_OBSERVED") {
		t.Fatalf("idle dispatcher: %s", x.stdout)
	}
	if got := headPrimary(t, state); got != moved {
		t.Fatalf("head primary %q", got)
	}
}
