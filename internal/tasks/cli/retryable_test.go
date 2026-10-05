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
// already ran (store.Report.Executed, set by GateRun) reports retryable false
// even for LOCK_TIMEOUT; the same error before any program ran stays retryable.
func TestCALV0078_ExecutedGateRunIsNotRetryable(t *testing.T) {
	contended := wire.Errorf(wire.CodeLockTimeout, "lock", "lock acquisition exceeded 30s")
	for _, c := range []struct {
		report *store.Report
		want   string
	}{
		{&store.Report{Executed: true}, `"retryable":false`},
		{&store.Report{}, `"retryable":true`},
		{nil, `"retryable":true`},
	} {
		raw, err := executedResult([]string{"gate", "run"}, c.report, contended).Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"codes":["LOCK_TIMEOUT"]`) || !strings.Contains(got, c.want) {
			t.Fatalf("executed=%v: %s", c.report != nil && c.report.Executed, got)
		}
	}
}
