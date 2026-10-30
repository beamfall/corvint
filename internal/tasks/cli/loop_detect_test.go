package cli_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// loopCLIHandoffs claims id at stage implement n times through the CLI and
// releases each generation as a clean no-tree hand-off.
func loopCLIHandoffs(t *testing.T, root, id string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		c := handoffCLI(t, root, "claim", id, "--holder", fmt.Sprintf("w%d", i), "--stage", "implement", "--request-id", fmt.Sprintf("claim-%d", i))
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, c.stdout)
		}
		a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		if r := handoffCLI(t, root, "release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", "no-change", "--request-id", fmt.Sprintf("release-%d", i)); r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release %d: %s", i, r.stdout)
		}
	}
}

// loopCLIPolicy adds loopDetection to the store's policy through policy update.
func loopCLIPolicy(t *testing.T, root string) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, e := intent.Load(repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	next := fmt.Sprint(loaded.Policy.PolicyVersion.Uint64() + 1)
	v.Obj.Set("policyVersion", wire.String(next))
	v.Obj.Set("loopDetection", wire.ObjectValue(wire.NewObject().Set("maxAlternatingReturns", wire.String("2")).Set("maxNoProgressGenerations", wire.String("2"))))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := handoffCLI(t, root, "policy", "update", "--request-id", "loop-policy", "--expected-policy-version", string(loaded.Policy.PolicyVersion), "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

// CAL-V0-102/103 through the CLI: under an opted-in policy the third
// no-progress hand-off holds the ticket; ticket show, ticket blockers, plan
// preview, claim and the dispatcher observation all report LOOP_DETECTED
// with the counted generations and reopen as the next action; the owner
// reopen clears it. Without the policy the same history shows no hold (D8).
func TestCALV0102_CLILoopHoldSurfaces(t *testing.T) {
	t.Run("CAL-V0-102 CAL-V0-103 CLILoopHoldSurfaces", func(t *testing.T) {
		for _, opted := range []bool{true, false} {
			root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
			if opted {
				loopCLIPolicy(t, root)
			}
			id := planTicket(t, root, "looping", "P1", `["src/"]`)
			loopCLIHandoffs(t, root, id, 3)
			show := handoffCLI(t, root, "ticket", "show", id)
			blockers := handoffCLI(t, root, "ticket", "blockers", id)
			plan := handoffCLI(t, root, "plan", "preview")
			claim := handoffCLI(t, root, "claim", id, "--holder", "late", "--stage", "implement", "--request-id", "claim-late")
			obs, err := cli.ObserveDispatch(root)
			if err != nil || len(obs.Tickets) != 1 {
				t.Fatalf("dispatch observation: %v", err)
			}
			if !opted {
				for name, out := range map[string][]byte{"show": show.stdout, "blockers": blockers.stdout, "plan": plan.stdout} {
					if strings.Contains(string(out), "LOOP_DETECTED") || strings.Contains(string(out), "no-progress loop") || strings.Contains(string(out), `"loop"`) {
						t.Fatalf("policy-absent %s mentions a loop: %s", name, out)
					}
				}
				if claim.res.Outcome != wire.OutcomeOK || obs.Tickets[0].Loop != nil {
					t.Fatalf("policy-absent claim %s loop %+v", claim.stdout, obs.Tickets[0].Loop)
				}
				continue
			}
			for name, out := range map[string][]byte{"show": show.stdout, "blockers": blockers.stdout} {
				if !strings.Contains(string(out), `"LOOP_DETECTED"`) || !strings.Contains(string(out), "exceed the policy bound 2") || !strings.Contains(string(out), `"nextAction":"reopen"`) {
					t.Fatalf("%s: %s", name, out)
				}
			}
			entries := field(plan.res.Items[0], "entries").Arr
			if len(entries) != 1 || field(entries[0], "reason").Str != wire.CodeLoopDetected {
				t.Fatalf("plan: %s", plan.stdout)
			}
			// The plain plan entry carries the hold's evidence, not only its code.
			loop := field(entries[0], "loop")
			if loop.Kind != wire.KindObject || field(loop, "signal").Str != "NO_PROGRESS" || len(field(loop, "generations").Arr) != 3 || field(loop, "limit").Str != "2" || field(loop, "acceptanceRevision").Str == "" {
				t.Fatalf("plan entry loop evidence: %s", plan.stdout)
			}
			gens := make([]string, 0, 3)
			for _, g := range field(loop, "generations").Arr {
				gens = append(gens, g.Str)
			}
			evidence := "no-progress loop NO_PROGRESS at acceptanceRevision " + field(loop, "acceptanceRevision").Str + ": generations " + strings.Join(gens, ",") + " exceed the policy bound 2"
			next := handoffCLI(t, root, "claim", "--next", "--holder", "late-next", "--stage", "implement", "--request-id", "claim-next-late")
			if !hasCode(next.res, wire.CodeLoopDetected) || !strings.Contains(string(next.stdout), evidence) {
				t.Fatalf("claim --next lacks %q: %s", evidence, next.stdout)
			}
			if !hasCode(claim.res, wire.CodeLoopDetected) || !strings.Contains(string(claim.stdout), evidence) {
				t.Fatalf("claim: %s", claim.stdout)
			}
			if h := obs.Tickets[0].Loop; h == nil || h.Signal != "NO_PROGRESS" || len(h.Generations) != 3 {
				t.Fatalf("dispatcher hold %+v", h)
			}
			rev := field(show.res.Items[0], "revision").Str
			if r := handoffCLI(t, root, "ticket", "reopen", "--target", id, "--expected-revision", rev, "--payload", `{"reason":"owner acknowledges the loop"}`, "--issued-at", "2026-09-29T00:00:00Z", "--request-id", "ack", "--role", "OWNER"); r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("reopen: %s %s", r.stdout, r.stderr)
			}
			if again := handoffCLI(t, root, "ticket", "show", id); strings.Contains(string(again.stdout), "LOOP_DETECTED") {
				t.Fatalf("reopen did not clear the hold: %s", again.stdout)
			}
			if r := handoffCLI(t, root, "claim", id, "--holder", "fresh", "--stage", "implement", "--request-id", "claim-fresh"); r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("fresh claim: %s", r.stdout)
			}
			if r := handoffCLI(t, root, "receipt", "audit"); r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("audit: %s", r.stdout)
			}
		}
	})
}
