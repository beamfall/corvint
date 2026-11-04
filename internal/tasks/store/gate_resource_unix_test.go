//go:build darwin || linux

package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sharedGate is commandGate with a sharedResource of class SHARED_GATE.
func sharedGate(id, script, key string) wire.Value {
	g := commandGate(id, script, "30", false)
	g.Obj.Set("sharedResource", obj("class", str("SHARED_GATE"), "key", str(key)))
	return g
}

// TestLGLV0885_GateRunSerializesASharedResource (LGL-V0-885, V1-3950): a gate
// that declares a sharedResource runs instead of refusing UNSUPPORTED; while
// one run holds the resource a second gate on the same resource refuses
// LOCK_TIMEOUT, retryable, before its program starts and without writing,
// a gate on another resource still runs, and once the holder ends the
// refused gate runs. Preflight still refuses the gate UNSUPPORTED.
func TestLGLV0885_GateRunSerializesASharedResource(t *testing.T) {
	t.Parallel()
	// CSD-V0-7513: force the 68-byte scratch prefix from the original Mac failure.
	// Concatenating it twice was 199 bytes; even a cd-based script was 134 (>128).
	parent, err := os.MkdirTemp("", "g")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parent) })
	sync := filepath.Join(parent, strings.Repeat("p", max(1, 68-len(parent)-1)))
	if err := os.Mkdir(sync, 0o700); err != nil {
		t.Fatal(err)
	}
	started, release, ran := filepath.Join(sync, "started"), filepath.Join(sync, "release"), filepath.Join(sync, "ran")
	holdScript := `: > "$1"; while [ ! -e "$2" ]; do /bin/sleep 0.05; done`
	hold := sharedGate("hold", holdScript, "heavy")
	// Each path is its own bounded argv entry, rather than part of the shell program.
	hold.Obj.Set("argv", wire.Strings([]string{"sh", "-c", holdScript, "hold", started, release}))
	s := newLeaseStore(t,
		hold,
		sharedGate("probe", ": > "+ran, "heavy"),
		sharedGate("other", "printf ok", "light"))
	claim, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)

	done := make(chan error, 1)
	go func() {
		r, err := s.gate(t, "gate-hold", gateOf(claim, "hold"), 2)
		if err == nil && r.Outcome.Outcome != mutation.OutcomeCompleted {
			err = wire.Errorf(wire.CodeMalformed, "gate", "hold: %+v", r)
		}
		done <- err
	}()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the holding gate never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	before := storeDigest(t, s.repo)
	_, err = s.gate(t, "gate-probe", gateOf(claim, "probe"), 2)
	if wire.CodeOf(err) != wire.CodeLockTimeout || !wire.RetryOf(wire.CodeOf(err)).Retryable {
		t.Fatalf("probe while held: %v", err)
	}
	if _, statErr := os.Stat(ran); statErr == nil || storeDigest(t, s.repo) != before {
		t.Fatal("the refused gate ran or wrote")
	}
	s.passes(t, "gate-other", gateOf(claim, "other"), 2)
	fixtureTouch(t, release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	s.passes(t, "gate-probe-2", gateOf(claim, "probe"), 4)
	if _, err := os.Stat(ran); err != nil {
		t.Fatalf("probe after release did not run: %v", err)
	}
	for _, id := range []string{"hold", "probe", "other"} {
		if g := s.results(t, claim.AttemptID)[id]; g == nil || g.State != "PASSED" {
			t.Fatalf("%s result: %+v", id, g)
		}
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, intent.Dir, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PreflightGateDefinitions(policy, []string{"probe"}); wire.CodeOf(err) != wire.CodeUnsupported {
		t.Fatalf("preflight: %v", err)
	}
}

func fixtureTouch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}
