//go:build darwin || linux

package store_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0078_SupervisedGateRecordFailureIsNotRetryable: a supervised
// reviewer run whose required gate ran but could not be recorded returns the
// LOCK_TIMEOUT marked not retryable, because a repeated `run --role reviewer`
// skips the CHECKING attempt instead of finishing its gates (CAL-V0-078).
func TestCALV0078_SupervisedGateRecordFailureIsNotRetryable(t *testing.T) {
	f := buildProgramFixture(t, true, false, nil)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	ran := 0
	restore := store.SetGateRecordFaultForTest(func() error {
		ran++
		return wire.Errorf(wire.CodeLockTimeout, "lock", "injected contention after the gate ran")
	})
	a, err := w.RunRole(ctx, "reviewer", "")
	restore()
	if ran != 1 {
		t.Fatalf("gate ran %d times before recording", ran)
	}
	if wire.CodeOf(err) != wire.CodeLockTimeout || !wire.RetryForbidden(err) {
		t.Fatalf("unrecorded gate reported retryable: %v (forbidden=%v)", err, wire.RetryForbidden(err))
	}
	if a == nil || a.Phase != "CHECKING" || len(a.GateResults) != 0 {
		t.Fatalf("attempt after the unrecorded gate: %+v", a)
	}
}

// TestCALV0078_SupervisedCheckingFailureIsNotRetryable: once review leaves the
// attempt CHECKING, a repeated `run --role reviewer` (which selects only BUILT
// attempts) skips it, so every later error is not retryable: a READY step that
// fails after a gate passed, a later gate refused after an earlier one ran,
// and a first gate refused before any ran (CAL-V0-078).
func TestCALV0078_SupervisedCheckingFailureIsNotRetryable(t *testing.T) {
	for _, c := range []struct {
		name   string
		gates  []string
		fail   string
		seen   []string
		passed []string
	}{
		{"ready-after-gate-passed", []string{"verify"}, "ready", []string{"gate:verify", "ready"}, []string{"verify"}},
		{"later-gate-after-earlier-ran", []string{"check", "verify"}, "gate:verify", []string{"gate:check", "gate:verify"}, []string{"check"}},
		{"first-gate-before-any-ran", []string{"check", "verify"}, "gate:check", []string{"gate:check"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := buildProgramFixture(t, true, false, c.gates)
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
				t.Fatalf("implement: %+v %v", a, err)
			}
			seen := []string{}
			restore := store.SetRunFaultForTest(func(point string) error {
				if strings.HasPrefix(point, "gate:") || point == "ready" {
					seen = append(seen, point)
				}
				if point == c.fail {
					return wire.Errorf(wire.CodeLockTimeout, "lock", "injected contention at %s", point)
				}
				return nil
			})
			a, err := w.RunRole(ctx, "reviewer", "")
			restore()
			if strings.Join(seen, ",") != strings.Join(c.seen, ",") {
				t.Fatalf("points reached %v, want %v", seen, c.seen)
			}
			if wire.CodeOf(err) != wire.CodeLockTimeout || !wire.RetryForbidden(err) {
				t.Fatalf("CHECKING failure reported retryable: %v (forbidden=%v)", err, wire.RetryForbidden(err))
			}
			if a == nil || a.Phase != "CHECKING" {
				t.Fatalf("attempt after the failure: %+v", a)
			}
			results := f.s.results(t, a.AttemptID)
			if len(results) != len(c.passed) {
				t.Fatalf("recorded gates %v, want passed %v", results, c.passed)
			}
			for _, id := range c.passed {
				if g := results[id]; g == nil || g.State != "PASSED" {
					t.Fatalf("gate %s did not pass before the failure: %+v", id, g)
				}
			}
		})
	}
}

// TestCALV0078_SupervisedRunAfterCommitIsNotRetryable: once a supervisor
// transition of a reviewer run commits, a repeated `run --role reviewer` no
// longer selects the attempt, so a failure after that commit is not retryable:
// the refresh after DISPATCH or STOPPED, and either FINISHED program record
// after review left the attempt CHECKING. A failure before DISPATCH commits
// leaves the attempt BUILT and stays retryable (CAL-V0-078).
func TestCALV0078_SupervisedRunAfterCommitIsNotRetryable(t *testing.T) {
	for _, c := range []struct {
		fail      string
		forbidden bool
		phase     string
	}{
		{"transition:DISPATCH", false, "BUILT"},
		{"refresh:DISPATCH", true, ""},
		{"refresh:STOPPED", true, "CHECKING"},
		{"stage-finished", true, "CHECKING"},
		{"role-finished", true, "CHECKING"},
	} {
		t.Run(strings.ReplaceAll(c.fail, ":", "-"), func(t *testing.T) {
			f := buildProgramFixture(t, true, false, nil)
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			a, err := w.RunRole(ctx, "implementer", "")
			if err != nil || a.Phase != "BUILT" {
				t.Fatalf("implement: %+v %v", a, err)
			}
			id := a.AttemptID
			hit := 0
			restore := store.SetRunFaultForTest(func(point string) error {
				if point == c.fail {
					hit++
					return wire.Errorf(wire.CodeLockTimeout, "lock", "injected contention at %s", point)
				}
				return nil
			})
			_, err = w.RunRole(ctx, "reviewer", "")
			restore()
			if hit != 1 {
				t.Fatalf("fault point %s reached %d times", c.fail, hit)
			}
			if wire.CodeOf(err) != wire.CodeLockTimeout || wire.RetryForbidden(err) != c.forbidden {
				t.Fatalf("failure at %s: %v (forbidden=%v, want %v)", c.fail, err, wire.RetryForbidden(err), c.forbidden)
			}
			phase := f.s.attempt(t, id).Phase
			if c.phase != "" && phase != c.phase || c.phase == "" && phase == "BUILT" {
				t.Fatalf("stored phase after the failure at %s: %s", c.fail, phase)
			}
		})
	}
}
