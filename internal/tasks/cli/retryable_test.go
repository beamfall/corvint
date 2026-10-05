package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0078_RedoPendingReadIsRetryable: a read that meets a writer between
// receipt link-in and head rename reports NOT_RUN/REDO_PENDING with an
// explicit retryable true, so callers need not match the code.
func TestCALV0078_RedoPendingReadIsRetryable(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	fixture.PlantReceipt(t, r, 2)
	var out bytes.Buffer
	Run(Env{Cwd: r.Root, Args: []string{"queue", "status"}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &out})
	got := out.String()
	if !strings.Contains(got, `"codes":["REDO_PENDING"]`) || !strings.Contains(got, `"outcome":"NOT_RUN"`) || !strings.Contains(got, `"retryable":true`) {
		t.Fatalf("pending-redo read envelope: %s", got)
	}
}

// TestCALV0078_ExecutedGateRunIsNotRetryable: a lease command whose program
// already ran (store.Report.Unretryable, set by GateRun) reports retryable false
// even for LOCK_TIMEOUT; the same error before any program ran stays retryable.
func TestCALV0078_ExecutedGateRunIsNotRetryable(t *testing.T) {
	contended := wire.Errorf(wire.CodeLockTimeout, "lock", "lock acquisition exceeded 30s")
	for _, c := range []struct {
		report *store.Report
		want   string
	}{
		{&store.Report{Unretryable: true}, `"retryable":false`},
		{&store.Report{}, `"retryable":true`},
		{nil, `"retryable":true`},
	} {
		raw, err := unretryableResult([]string{"gate", "run"}, c.report, contended).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"codes":["LOCK_TIMEOUT"]`) || !strings.Contains(got, c.want) {
			t.Fatalf("executed=%v: %s", c.report != nil && c.report.Unretryable, got)
		}
	}
}

// TestCALV0078_CommittedSweepIsNotRetryable: a pool sweep whose fresh owner
// committed before the failure (store.PoolSweepReport.Unretryable) reports
// retryable false; the same error before that commit stays retryable.
func TestCALV0078_CommittedSweepIsNotRetryable(t *testing.T) {
	contended := wire.Errorf(wire.CodeLockTimeout, "lock", "lock acquisition exceeded 30s")
	for _, c := range []struct {
		report *store.PoolSweepReport
		want   string
	}{
		{&store.PoolSweepReport{Unretryable: true}, `"retryable":false`},
		{&store.PoolSweepReport{}, `"retryable":true`},
		{nil, `"retryable":true`},
	} {
		raw, err := sweepErrorResult([]string{"pool", "sweep"}, c.report, contended).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"codes":["LOCK_TIMEOUT"]`) || !strings.Contains(got, c.want) {
			t.Fatalf("committed=%v: %s", c.report != nil && c.report.Unretryable, got)
		}
	}
}

// TestCALV0078_MarkedErrorIsNotRetryable: an error carrying the store's
// non-retryable mark (a supervised reviewer's gate that ran but was not
// recorded) reports retryable false through errorResult, which `program run`
// uses; the same code unmarked stays retryable.
func TestCALV0078_MarkedErrorIsNotRetryable(t *testing.T) {
	contended := wire.Errorf(wire.CodeLockTimeout, "lock", "lock acquisition exceeded 30s")
	for _, c := range []struct {
		err  error
		want string
	}{
		{wire.WithoutRetry(contended), `"retryable":false`},
		{contended, `"retryable":true`},
	} {
		raw, err := errorResult([]string{"run"}, c.err).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"codes":["LOCK_TIMEOUT"]`) || !strings.Contains(got, c.want) {
			t.Fatalf("marked=%v: %s", wire.RetryForbidden(c.err), got)
		}
	}
}

// TestCALV0078_AnyMarkedLaneMakesBatchNotRetryable: `program run --count N`
// reports its first failed lane, but a repeat reruns every lane, so a later
// lane's non-retryable failure makes the command not retryable even when the
// reported one is retryable; idle and successful lanes do not.
func TestCALV0078_AnyMarkedLaneMakesBatchNotRetryable(t *testing.T) {
	contended := wire.Errorf(wire.CodeLockTimeout, "lock", "lock acquisition exceeded 30s")
	marked := wire.WithoutRetry(wire.Errorf(wire.CodeLockTimeout, "lock", "gate ran but was not recorded"))
	for _, c := range []struct {
		name  string
		lanes []error
		want  string
	}{
		{"retryable-then-marked", []error{contended, marked}, `"retryable":false`},
		{"marked-then-retryable", []error{marked, contended}, `"retryable":false`},
		{"retryable-then-idle", []error{contended, store.ErrProgramIdle}, `"retryable":true`},
		{"retryable-then-success", []error{contended, nil}, `"retryable":true`},
	} {
		raw, err := laneFailureResult([]string{"run"}, c.lanes[0], c.lanes).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"codes":["LOCK_TIMEOUT"]`) || !strings.Contains(got, c.want) {
			t.Fatalf("%s: %s", c.name, got)
		}
	}
}
