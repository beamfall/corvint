package cli_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// recordingDeriver derives "derived/<title>/" for every ticket it is asked
// about and records the titles in the order asked, so a test sees which
// ticket the CLI hands the deriver.
type recordingDeriver struct{ asked []string }

func (d *recordingDeriver) derive(_ context.Context, _, _, title, _ string) ([]string, string, bool) {
	d.asked = append(d.asked, title)
	sum := sha256.Sum256([]byte("derive:" + title))
	return []string{"derived/" + title + "/"}, hex.EncodeToString(sum[:]), true
}

// selectedEntry is the ticket a plan preview marks SELECTED.
func selectedEntry(t *testing.T, plan run) string {
	t.Helper()
	var chosen []string
	for _, e := range field(plan.res.Items[0], "entries").Arr {
		if field(e, "state").Str == "SELECTED" {
			chosen = append(chosen, field(e, "ticketId").Str)
		}
	}
	if len(chosen) != 1 {
		t.Fatalf("preview selects %v, want exactly one: %s", chosen, plan.stdout)
	}
	return chosen[0]
}

// PSR-V0-132, CAL-V0-350 (V1-1715): `claim --next` runs NextClaimTicket's
// candidate through the CLI's scope deriver, so with a waiter present the
// ticket the deriver is asked about, the ticket the claim takes and the
// ticket `plan preview --pool` selects are one. A CLI that gave the claim's
// candidate selection other inputs than the preview (the clock that honours
// the waitlist, the barrier) would derive scope for the first ticket and
// claim the waiter's.
func TestCALV0350_ClaimNextDerivesScopeForThePreviewedWaiterCLI(t *testing.T) {
	r := exclusionCLIRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a"})).Set("maxWaitSeconds", wire.String("600")))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init %s", x.stdout)
	}
	one := planTicket(t, r.Root, "one", "P2", `[]`)
	two := planTicket(t, r.Root, "two", "P2", `[]`)
	if x := atm(t, r.Root, nil, "pool", "wait", "--ticket", two, "--holder", "builder", "--pool", "db", "--ttl-seconds", "300", "--request-id", "w2"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket wait %s", x.stdout)
	}
	preview := atm(t, r.Root, nil, "plan", "preview", "--pool", "db", "--stage", "implement")
	if preview.res.Outcome != wire.OutcomeOK {
		t.Fatalf("preview %s", preview.stdout)
	}
	if got := selectedEntry(t, preview); got != two {
		t.Fatalf("preview selects %s, want the waiter %s (not %s): %s", got, two, one, preview.stdout)
	}

	d := &recordingDeriver{}
	x := atmWith(t, r.Root, nil, d.derive, "claim", "--next", "--holder", "builder", "--pool", "db", "--stage", "implement", "--request-id", "next")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "ticketId").Str != two {
		t.Fatalf("claim --next %s", x.stdout)
	}
	if !reflect.DeepEqual(d.asked, []string{"two"}) {
		t.Fatalf("deriver was asked about %v, want only the previewed waiter's ticket", d.asked)
	}
	sc := field(x.res.Items[0], "scope")
	res := field(sc, "resources")
	if field(sc, "source").Str != "DERIVED" || len(res.Arr) != 1 || field(res.Arr[0], "key").Str != "derived/two/" {
		t.Fatalf("claimed scope is not the one derived for the previewed ticket: %s", x.stdout)
	}
}
