//go:build darwin || linux

package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestESCV0008_DispatchObservesTypedRequests drives the real writers and
// the native dispatcher observation (ESC-V0-007, ESC-V0-008): typed escalate
// and answer writes keep the effective work revision, so they are never work
// progress; each current OPEN or ANSWERED request carries its kind, state,
// the holder its audited source recorded and its open time; an ordinary
// refine is visible; and removed material fails the observation closed.
func TestESCV0008_DispatchObservesTypedRequests(t *testing.T) {
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Truncate(time.Second).Add(-11*time.Minute))
	t.Setenv("CORVINT_TASKS_ACTOR", "holder")
	a := claimed[0]
	observe := func() dispatch.Ticket {
		t.Helper()
		tickets, err := cli.ObserveTickets(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range tickets {
			if x.ID == a.Ticket {
				return x
			}
		}
		t.Fatalf("ticket %s not observed", a.Ticket)
		return dispatch.Ticket{}
	}
	must := func(x run) {
		t.Helper()
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %+v", x.res.Command, x.res)
		}
	}
	base := observe()
	if base.AcceptanceRevision == "" || base.Requests != nil || base.EscalationUnknown {
		t.Fatalf("base observation = %+v", base)
	}
	must(atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", a.Receipt,
		"--kind", "infrastructure", "--question", "Runner lost its network?", "--request-id", "q-i"))
	must(atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", a.Receipt,
		"--kind", "decision", "--question", "Which store?", "--request-id", "q-d"))
	must(atm(t, root, nil, "ticket", "answer", "--target", a.Ticket, "--text", "x", "--request", "q-d", "--expected-request-revision", "1", "--request-id", "ans-d"))
	got := observe()
	if got.Revision != base.Revision || got.EscalationUnknown {
		t.Fatalf("typed writes changed the work revision: %s -> %s (unknown %v)", base.Revision, got.Revision, got.EscalationUnknown)
	}
	if len(got.Requests) != 2 {
		t.Fatalf("requests = %+v", got.Requests)
	}
	for _, r := range got.Requests {
		want := map[string][2]string{"q-i": {"infrastructure", "OPEN"}, "q-d": {"decision", "ANSWERED"}}[r.ID]
		if r.Kind != want[0] || r.State != want[1] || r.Holder != "holder" || r.Opened.IsZero() {
			t.Fatalf("request = %+v", r)
		}
	}
	raw, _ := strconv.Atoi(base.Revision)
	must(atm(t, root, nil, "ticket", "refine", "--request-id", "edit-1", "--target", a.Ticket, "--expected-revision", strconv.Itoa(raw+3), "--payload", `{"body":"An ordinary edit"}`))
	if edited := observe(); edited.Revision == base.Revision || edited.EscalationUnknown {
		t.Fatalf("ordinary refine hidden: %+v", edited)
	}
	// Remove one audited event: the observation fails closed.
	stateDir := filepath.Join(root, ".git", "taskman")
	if repo, err := intent.Resolve(root); err == nil {
		stateDir = repo.StateDir
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, e := range entries {
		p := filepath.Join(stateDir, "evidence", e.Name())
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 && strings.Contains(string(b), `"q-i"`) {
			os.Remove(p)
			removed++
		}
	}
	if removed == 0 {
		t.Fatal("no q-i evidence found to remove")
	}
	// The store refuses the whole observation, so nothing launches on it;
	// a record whose material is absent is UNKNOWN (internal test).
	if _, err := cli.ObserveTickets(root); err == nil {
		t.Fatal("observation accepted a store with removed escalation evidence")
	}
}
