package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// developmentModeWarnings returns the warnings of res that explain the
// DEVELOPMENT_MODE selection disposition.
func developmentModeWarnings(res *wire.Result) []string {
	var found []string
	for _, w := range res.Warnings {
		if strings.HasPrefix(w, "SELECTED DEVELOPMENT_MODE is the selection's disposition") {
			found = append(found, w)
		}
	}
	return found
}

// TestNTPV0670_PlanPreviewNamesTheDevelopmentModeRule: a preview that selects
// an entry with DEVELOPMENT_MODE carries one warning naming the rule (not a
// build check; the execution cutover gates claims) and how to proceed
// (claim --next), in both the full and the --selected-only projection; a
// preview that selects nothing carries none (V1-3535).
func TestNTPV0670_PlanPreviewNamesTheDevelopmentModeRule(t *testing.T) {
	t.Run("NTP-V0-670", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
		policy := fixture.PolicyValue()
		budgets, _ := policy.Obj.Get("budgets")
		budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
		fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
		if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("init: %+v", x.res)
		}
		planTicket(t, r.Root, "only", "P1", `["src/"]`)
		for _, args := range [][]string{{"plan", "preview"}, {"plan", "preview", "--selected-only"}} {
			x := atm(t, r.Root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %+v", args, x.res)
			}
			got := developmentModeWarnings(x.res)
			if len(got) != 1 {
				t.Fatalf("%v warnings = %q, want one DEVELOPMENT_MODE warning", args, x.res.Warnings)
			}
			for _, want := range []string{"not a build or qualification check", "To proceed, run claim --next", "cutover --execution", "BLOCKED CUTOVER_MISSING"} {
				if !strings.Contains(got[0], want) {
					t.Errorf("%v warning %q lacks %q", args, got[0], want)
				}
			}
		}
		if p := atm(t, r.Root, nil, "pause", "--request-id", "pause"); p.res.Outcome != wire.OutcomeOK {
			t.Fatalf("pause: %+v", p.res)
		}
		if paused := atm(t, r.Root, nil, "plan", "preview"); len(developmentModeWarnings(paused.res)) != 0 {
			t.Fatalf("paused preview selected nothing but warned: %q", paused.res.Warnings)
		}
	})
}
