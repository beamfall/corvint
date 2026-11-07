// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCL-V0-859: a ticket show racing a bounded series of real commits returns
// its ticket, or an explicitly retryable refusal; OK with no items is never
// an admissible observation (V1-3462, reproduce-or-refute).
func TestTCLV0859_ShowConcurrentWritesNeverEmpty(t *testing.T) {
	r, id, _ := papercutRepo(t)
	start := make(chan struct{})
	done := make(chan string, 1)
	go func() {
		<-start
		for i := 0; i < 30; i++ {
			x := atm(t, r.Root, nil, "ticket", "refine", "--target", id, "--expected-revision", fmt.Sprint(i+1), "--request-id", fmt.Sprintf("concurrent-%d", i), "--payload", fmt.Sprintf(`{"title":"Concurrent %d"}`, i))
			if x.code != 0 {
				done <- string(x.stdout)
				return
			}
		}
		done <- ""
	}()
	close(start)
	var bad string
	for i := 0; i < 60; i++ {
		x := atm(t, r.Root, nil, "ticket", "show", id)
		if x.res.Outcome == wire.OutcomeOK {
			if len(x.res.Items) != 1 || field(x.res.Items[0], "ticketId").Str == "" {
				bad = string(x.stdout)
				break
			}
		} else {
			v, err := wire.Parse(x.stdout)
			if err != nil || !field(v, "retryable").Bool {
				bad = string(x.stdout)
				break
			}
		}
	}
	writerError := <-done
	if writerError != "" {
		t.Fatalf("concurrent write: %s", writerError)
	}
	if bad != "" {
		t.Fatalf("ticket show lost an existing ticket: %s", bad)
	}
}
