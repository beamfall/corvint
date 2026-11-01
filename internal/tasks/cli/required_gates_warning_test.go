package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// requiredGatesEmpty is the TCD-V0-807 warnings of one result.
func requiredGatesEmpty(x run) []string {
	var out []string
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, "REQUIRED_GATES_EMPTY: ") {
			out = append(out, w)
		}
	}
	return out
}

// requiredGatesStore is a store whose policy defines the given gates: the
// fixture's required `verify` gate, plus an optional `docs` gate when asked.
func requiredGatesStore(t *testing.T, gates bool) string {
	t.Helper()
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	if gates {
		verify, _ := policy.Obj.Get("gates")
		other, _ := fixture.PolicyValue().Obj.Get("gates")
		docs := other.Arr[0]
		docs.Obj.Set("gateId", wire.String("docs")).Set("required", wire.Bool(false))
		policy.Obj.Set("gates", wire.Array(docs, verify.Arr[0]))
	} else {
		policy.Obj.Set("gates", wire.Array())
	}
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	return r.Root
}

// TestTCDV0807_CreateWithoutGatesNamesThePolicyGates: a create naming no
// requiredGates under a policy with a required and an optional gate commits
// unchanged and warns naming both; a create naming the optional gate, a
// replay and a policy without gates warn nothing (V1-3541).
func TestTCDV0807_CreateWithoutGatesNamesThePolicyGates(t *testing.T) {
	root := requiredGatesStore(t, true)
	create := func(req, gates string) run {
		t.Helper()
		payload := strings.Replace(createPayloadJSON, `"requiredGates":[]`, `"requiredGates":`+gates, 1)
		x := atm(t, root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("create %s: %s", req, x.stdout)
		}
		return x
	}
	x := create("empty", `[]`)
	id := field(x.res.Items[0], "ticketId").Str
	w := requiredGatesEmpty(x)
	if len(w) != 1 {
		t.Fatalf("warnings = %q", x.res.Warnings)
	}
	for _, want := range []string{"ticket " + id + " names no requiredGates", "complete still demands the policy-required gates verify", "effectiveRequiredGates", "it will not demand docs unless the ticket names them with ticket set-gates", "(TCD-V0-807)"} {
		if !strings.Contains(w[0], want) {
			t.Fatalf("warning lacks %q: %q", want, w[0])
		}
	}
	if replay := create("empty", `[]`); !field(replay.res.Items[0], "replayed").Bool || len(requiredGatesEmpty(replay)) != 0 {
		t.Fatalf("replay: %s", replay.stdout)
	}
	if named := create("named", `["docs"]`); len(requiredGatesEmpty(named)) != 0 {
		t.Fatalf("named gates warned: %s", named.stdout)
	}
	bare := requiredGatesStore(t, false)
	if x := atm(t, bare, nil, "ticket", "create", "--request-id", "bare", "--issued-at", "2026-10-08T12:00:00Z", "--payload", createPayloadJSON); x.res.Outcome != wire.OutcomeOK || len(requiredGatesEmpty(x)) != 0 {
		t.Fatalf("policy without gates: %s", x.stdout)
	}
}
