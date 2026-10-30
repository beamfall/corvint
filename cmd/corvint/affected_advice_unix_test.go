//go:build unix && !solaris

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-032: a declaration path that exists but is not a readable regular
// file is unknown, never absent, and reading it never blocks.
func TestAffectedAdviceUnreadableDeclarationSuppressesNoGate(t *testing.T) {
	t.Parallel()
	t.Run("AFP-V0-032 directory declaration", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, adviceMakefileName), 0o755); err != nil {
			t.Fatal(err)
		}
		assertUnreadableDeclaration(t, root, adviceMakefileName)
	})
	t.Run("AFP-V0-032 symlink to a FIFO never blocks", func(t *testing.T) {
		root := t.TempDir()
		fifo := filepath.Join(t.TempDir(), "gate.fifo")
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		if err := os.Symlink(fifo, filepath.Join(root, adviceAgentsName)); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			assertUnreadableDeclaration(t, root, adviceAgentsName)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			// Release every blocked read (each assertion reads twice) so the
			// test binary can exit and report the failure.
			for released := false; !released; {
				if writer, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
					_ = writer.Close()
				}
				select {
				case <-done:
					released = true
				case <-time.After(100 * time.Millisecond):
				}
			}
			t.Fatal("declaration read blocked on a FIFO")
		}
	})
	t.Run("AFP-V0-032 dangling symlink is not absence", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, adviceMakefileName)); err != nil {
			t.Fatal(err)
		}
		assertUnreadableDeclaration(t, root, adviceMakefileName)
	})
}

func assertUnreadableDeclaration(t *testing.T, root, name string) {
	t.Helper()
	checks, unknown, truncated := mandatoryAffectedChecks(root)
	if len(checks) != 0 || !truncated {
		t.Errorf("unreadable %s: checks=%v truncated=%v", name, checks, truncated)
	}
	advice := compileAffectedAdvice(root, nil, affected.Plan{}, affectedGoProvider{State: providerStateEmpty, Packages: []string{}})
	want := []string{"MANDATORY_DECLARATION_UNREADABLE: " + name + " exists but is not a readable regular file"}
	if !slices.Contains(unknown, want[0]) || !slices.Contains(advice.Unknown, want[0]) {
		t.Errorf("unreadable %s not named: %v / %v", name, unknown, advice.Unknown)
	}
	if slices.Contains(advice.Unknown, adviceNoGateUnknown) {
		t.Errorf("unreadable %s produced a gate-absence claim: %v", name, advice.Unknown)
	}
}

func TestAffectedAdviceFocusedNextStep_AFPV0496(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, adviceMakefileName), []byte("gate:\n\ttrue\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Dirty: []string{"a.go"}}
	base := compileAffectedAdvice(root, nil, plan, affectedGoProvider{State: providerStateRunnable, Packages: []string{"./a"}})
	advice := withFocusedNextStep(base, "abc123", plan)
	first := -1
	for index, check := range advice.Checks {
		if check.Kind == adviceKindAdvisory {
			first = index
			break
		}
	}
	if first < 1 || advice.Checks[first].Command != "corvint test --base 'abc123' --dry-run" {
		t.Fatalf("focused command is not the first advisory check: %+v", advice.Checks)
	}
	if !strings.HasPrefix(advice.Checks[first+1].Command, adviceAdvisoryGoTest) {
		t.Errorf("per-unit command does not follow: %+v", advice.Checks)
	}
	gate := advice.Checks[0]
	if gate.Command != "make gate" || !strings.Contains(gate.Reason, "terminal boundary, not as the default next step") {
		t.Errorf("gate reason: %+v", gate)
	}
	if got := withFocusedNextStep(base, "", plan); len(got.Checks) != len(base.Checks) {
		t.Errorf("no base added a check")
	}
	if got := withFocusedNextStep(base, "abc123", affected.Plan{}); len(got.Checks) != len(base.Checks) {
		t.Errorf("no dirty path added a check")
	}
}
