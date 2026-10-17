//go:build darwin || linux

package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0270_DispatchStatusShowsFailingWorkStateAfterOneTick is the issue
// 984 reproduction through the CLI: one `dispatch --once` tick with a failing
// work-state command makes `dispatch status` report the reader FAILING with
// its error, start time and failed tick count.
func TestCALV0270_DispatchStatusShowsFailingWorkStateAfterOneTick(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "workstate", "P1", `["src/"]`)
	config, _ := readerCLIConfig(t, root, `echo interpreter missing >&2; exit 127`)
	base := []string{"--program", "prog", "--config", config}
	status := func() wire.Value {
		t.Helper()
		r := atm(t, root, nil, append([]string{"dispatch", "status"}, base...)...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("status: %s %s", r.stdout, r.stderr)
		}
		return field(r.res.Items[0], "workState")
	}
	if ws := status(); field(ws, "health").Str != "NOT_OBSERVED" || field(ws, "escalateAfterTicks").Str != "3" || field(ws, "holdLaunchesAfterTicks").Str != "NONE" {
		t.Fatalf("before any tick: %+v", ws)
	}
	if r := atm(t, root, nil, append([]string{"dispatch", "--once"}, base...)...); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("tick: %s %s", r.stdout, r.stderr)
	}
	ws := status()
	if field(ws, "health").Str != "FAILING" || field(ws, "ok").Bool || field(ws, "failedTicks").Str != "1" || field(ws, "escalated").Bool || field(ws, "launchesHeld").Bool {
		t.Fatalf("after one failed tick: %+v", ws)
	}
	if !strings.Contains(field(ws, "lastError").Str, "work state command") {
		t.Fatalf("lastError %q", field(ws, "lastError").Str)
	}
	if _, err := time.Parse(time.RFC3339, field(ws, "since").Str); err != nil {
		t.Fatalf("since %q: %v", field(ws, "since").Str, err)
	}
}
