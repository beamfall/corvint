package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0868_CoverageIsAdvisoryUnderEveryPolicy (V1-3794): INCOMPLETE
// effects never make ticket show or roadmap read BLOCKED, because no queue
// policy makes coverage a claim prerequisite. Under serialFallback BLOCK the
// view lists COVERAGE_UNKNOWN as an advisory beside PASSED intent checks and
// next action set-effects, and a claim without --scope is admitted; under
// WHOLE_REPOSITORY the view carries no coverage entry and the next action is
// admit.
func TestTCLV0868_CoverageIsAdvisoryUnderEveryPolicy(t *testing.T) {
	for _, tc := range []struct {
		fallback, next string
		advisory       bool
	}{
		{"BLOCK", "set-effects", true},
		{"WHOLE_REPOSITORY", "admit", false},
	} {
		t.Run(tc.fallback, func(t *testing.T) {
			r := fixture.TempRepo(t)
			policy := fixture.PolicyValue()
			policy.Obj.Set("serialFallback", wire.String(tc.fallback))
			budgets, _ := policy.Obj.Get("budgets")
			budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
			if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("init: %s", x.stdout)
			}
			git(t, r.Root, "init", "-q", "-b", "main")
			fixture.Write(t, filepath.Join(r.Root, "src", "a.go"), []byte("a"))
			git(t, r.Root, "add", "src")
			git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
			id, _ := effectsTicket(t, r.Root, "incomplete", "P2", `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":[]}`)

			show := atm(t, r.Root, nil, "ticket", "show", id)
			if show.res.Outcome != wire.OutcomeOK || len(show.res.Items) != 1 {
				t.Fatalf("ticket show: %s", show.stdout)
			}
			item := show.res.Items[0]
			if got := field(item, "eligibility").Str; got == "BLOCKED" {
				t.Fatalf("eligibility BLOCKED for INCOMPLETE coverage: %s", show.stdout)
			}
			if field(item, "intentChecks").Str != "PASSED" || len(field(item, "blockers").Arr) != 0 || len(field(item, "intentChecksFailed").Arr) != 0 {
				t.Fatalf("coverage judged as an intent check: %s", show.stdout)
			}
			if got := field(item, "nextAction").Str; got != tc.next {
				t.Fatalf("nextAction %q, want %q: %s", got, tc.next, show.stdout)
			}
			adv, ok := item.Obj.Get("advisories")
			if tc.advisory != ok {
				t.Fatalf("advisories present=%v, want %v: %s", ok, tc.advisory, show.stdout)
			}
			if ok && (len(adv.Arr) != 1 || field(adv.Arr[0], "code").Str != wire.CodeCoverageUnknown) {
				t.Fatalf("advisories %s", wire.Encode(adv))
			}

			rm := atm(t, r.Root, nil, "roadmap")
			if rm.res.Outcome != wire.OutcomeOK || len(rm.res.Items) != 1 {
				t.Fatalf("roadmap: %s", rm.stdout)
			}
			if row := rm.res.Items[0]; field(row, "eligibility").Str == "BLOCKED" || field(row, "nextAction").Str != tc.next {
				t.Fatalf("roadmap row: %s", wire.Encode(row))
			}

			// The advisory is honest: the claim it does not block is admitted.
			c := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-"+tc.fallback)
			if c.res.Outcome != wire.OutcomeOK || field(c.res.Items[0], "outcome").Str != "COMPLETED" {
				t.Fatalf("claim: %s", c.stdout)
			}
		})
	}
}
