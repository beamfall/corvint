package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0191_StaleProjectionReadWarns pins V1-0324: a read over an intent
// projection that differs from the journal head's afterimage still succeeds
// without blocking, and it carries a PROJECTION_STALE warning naming the
// projected tree, the head seq and the stale path. A current projection
// carries no such warning, and an audited read keeps INTENT_DIVERGED.
func TestPSRV0191_StaleProjectionReadWarns(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	warned := func(x run) string {
		for _, w := range x.res.Warnings {
			if strings.HasPrefix(w, "PROJECTION_STALE") {
				return w
			}
		}
		return ""
	}
	if x := atm(t, r.Root, nil, "ticket", "show", id); x.res.Outcome != wire.OutcomeOK || warned(x) != "" {
		t.Fatalf("current projection: %+v", x.res)
	}

	files, err := filepath.Glob(filepath.Join(r.IntentDir, "tickets", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("intent tickets: %v %v", files, err)
	}
	path := files[0]
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var title string
	if x := atm(t, r.Root, nil, "ticket", "show", id); len(x.res.Items) == 1 {
		title = field(x.res.Items[0], "title").Str
	}
	if title == "" || !strings.Contains(string(b), title) {
		t.Fatalf("ticket %s has no title to edit", id)
	}
	fixture.Write(t, path, []byte(strings.Replace(string(b), title, title+" edited outside the journal", 1)))

	for _, c := range [][]string{{"queue", "status"}, {"ticket", "list"}, {"ticket", "search", "--status", "OPEN"}} {
		x := atm(t, r.Root, nil, c...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: a stale-projection read blocked: %+v", c, x.res)
		}
		w := warned(x)
		if w == "" {
			t.Fatalf("%v: stale projection read without PROJECTION_STALE: %v", c, x.res.Warnings)
		}
		for _, want := range []string{"intent/tickets/" + filepath.Base(path), "journal head seq 2", "INTENT_DIVERGED"} {
			if !strings.Contains(w, want) {
				t.Errorf("%v: warning does not name %q: %s", c, want, w)
			}
		}
	}
	// A read that audits the journal for private state keeps the audit's
	// INTENT_DIVERGED refusal: only the unaudited reads are non-blocking.
	if x := atm(t, r.Root, nil, "ticket", "show", id); x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeIntentDiverged) {
		t.Errorf("audited read over a stale projection: %+v", x.res)
	}
}
