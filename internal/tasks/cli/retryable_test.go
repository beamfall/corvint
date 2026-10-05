package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
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
