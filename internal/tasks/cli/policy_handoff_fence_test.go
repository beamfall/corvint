package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-122 and CAL-V0-124 replay issue 643: attempts claimed before an
// OWNER policy update that only adds a pool member keep their clean HANDOFF
// and REVIEW_RETURNED release and retry exemption, and the update reports no
// fence. A later budget change lists every live attempt it fences, and that
// handoff still refuses STALE_POLICY.
func TestCALV0122_Issue643AdditiveMemberKeepsHandoff(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	runOK := func(args ...string) run {
		t.Helper()
		r := handoffCLI(t, root, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	loaded, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	policyDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policyFile := filepath.Join(policyDir, "policy.json")
	update := func(version, request string) run {
		t.Helper()
		v.Obj.Set("policyVersion", wire.String(version))
		if err := os.WriteFile(policyFile, wire.EncodeFile(v), 0600); err != nil {
			t.Fatal(err)
		}
		prior := map[string]string{"2": "1", "3": "2", "4": "3"}[version]
		return runOK("policy", "update", "--request-id", request, "--expected-policy-version", prior, "--file", policyFile)
	}
	fences := func(r run) []wire.Value {
		t.Helper()
		f := field(r.res.Items[0], "handoffFences")
		if f.Kind != wire.KindArray {
			t.Fatalf("handoffFences: %s", r.stdout)
		}
		return f.Arr
	}
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("demo-lanes")).Set("members", wire.Strings([]string{"lane-1", "lane-2"})))))
	update("2", "pool-policy")

	claim := func(title, stage, holder, pool string) (string, string) {
		t.Helper()
		id := planTicket(t, root, title, "P1", `["src/`+title+`/"]`)
		args := []string{"claim", id, "--holder", holder, "--stage", stage, "--request-id", "claim-" + title}
		if pool != "" {
			args = append(args, "--pool", pool)
		}
		c := runOK(args...)
		return field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	}
	impl, implGen := claim("implement", "implement", "implementer", "demo-lanes")
	review, reviewGen := claim("review", "review", "verifier", "demo-lanes")
	earlier, _ := claim("earlier", "implement", "earlier", "")
	before := map[string]run{impl: runOK("attempt", "show", impl), review: runOK("attempt", "show", review)}

	// Issue 643: the OWNER update only adds lane-3 to demo-lanes.
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("demo-lanes")).Set("members", wire.Strings([]string{"lane-1", "lane-2", "lane-3"})))))
	if got := fences(update("3", "op-policy-v9-lane-3")); len(got) != 0 {
		t.Fatalf("additive update reported fences: %s", wire.Encode(wire.Array(got...)))
	}
	runOK("release", "--attempt", impl, "--generation", implGen, "--request-id", "handoff", "--reason", wire.CodeHandoff, "--evidence", "implementer-handoff")
	runOK("release", "--attempt", review, "--generation", reviewGen, "--request-id", "return", "--reason", wire.CodeReviewReturned, "--evidence", "g2-verdict")
	for a, reason := range map[string]string{impl: wire.CodeHandoff, review: wire.CodeReviewReturned} {
		after := runOK("attempt", "show", a)
		item := after.res.Items[0]
		accounting := field(item, "retryAccounting")
		if field(item, "phase").Str != "CANCELLED" || field(accounting, "disposition").Str != reason || field(accounting, "failedOrUnknown").Bool {
			t.Fatalf("%s: %s", reason, after.stdout)
		}
		for _, key := range []string{"policySha256", "configSha256", "retryCount"} {
			if !bytes.Equal(wire.Encode(field(before[a].res.Items[0], key)), wire.Encode(field(item, key))) {
				t.Fatalf("%s rewrote %s", reason, key)
			}
		}
	}

	// Counterexample: a budget change fences the attempt claimed under the
	// current policy and cannot be judged for one whose interval began earlier.
	current, currentGen := claim("current", "implement", "current", "")
	x, _ := v.Obj.Get("budgets")
	x.Obj.Set("ticketMultiplier", wire.String("5"))
	r := update("4", "budget")
	got := map[string]string{}
	for _, f := range fences(r) {
		got[field(f, "attemptId").Str] = field(f, "handoff").Str
	}
	if len(got) != 2 || got[current] != "FENCED" || got[earlier] != "NOT_OBSERVED" || len(r.res.Warnings) == 0 {
		t.Fatalf("budget fences: %s", r.stdout)
	}
	refused := handoffCLI(t, root, "release", "--attempt", current, "--generation", currentGen, "--request-id", "fenced", "--reason", wire.CodeHandoff, "--evidence", "fenced-handoff")
	if len(refused.res.Codes) != 1 || refused.res.Codes[0] != wire.CodeStalePolicy {
		t.Fatalf("budget change did not fence: %s", refused.stdout)
	}
	runOK("receipt", "audit")
}
