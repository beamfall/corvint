package appflows

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func TestAFUV1VerificationRequiresStability(t *testing.T) {
	cases := []struct {
		name, want string
		edit       func(*RunRegistry, *[]TestRunEvidence)
	}{
		{"clean", "verified", func(*RunRegistry, *[]TestRunEvidence) {}},
		{"wildcard-project-missing", GapStabilityMissing, func(_ *RunRegistry, rs *[]TestRunEvidence) {
			other := (*rs)[0]
			other.Project = "firefox"
			*rs = append(*rs, other)
		}},
		{"wildcard-empty-project", "verified", func(r *RunRegistry, rs *[]TestRunEvidence) {
			r.Aggregates[0].Project = ""
			for i := range *rs {
				(*rs)[i].Project = ""
			}
		}},
		{"conflicting-scopes", GapStabilityIncomplete, func(r *RunRegistry, _ *[]TestRunEvidence) {
			other := r.Aggregates[0]
			other.ID = "suite"
			other.Scope = "suite"
			other.Contributions = other.Contributions[:1]
			r.Aggregates = append(r.Aggregates, other)
		}},
		{"missing-aggregate", GapStabilityMissing, func(r *RunRegistry, _ *[]TestRunEvidence) { r.Aggregates[0].TestKey = "other" }},
		{"wrong-project", GapStabilityMissing, func(r *RunRegistry, _ *[]TestRunEvidence) { r.Aggregates[0].Project = "firefox" }},
		{"incomplete", GapStabilityIncomplete, func(r *RunRegistry, _ *[]TestRunEvidence) {
			r.Aggregates[0].Contributions = r.Aggregates[0].Contributions[:1]
		}},
		{"missing-record", GapStabilityIncomplete, func(_ *RunRegistry, rs *[]TestRunEvidence) { *rs = (*rs)[:1] }},
		{"stale", GapStabilityStale, func(_ *RunRegistry, rs *[]TestRunEvidence) {
			for i := range *rs {
				(*rs)[i].Source.Commit = strings.Repeat("a", 40)
			}
		}},
		{"below-threshold", GapStabilityFailed, func(r *RunRegistry, rs *[]TestRunEvidence) {
			(*rs)[1].Attempts = append((*rs)[1].Attempts, (*rs)[1].Attempts[0])
			(*rs)[1].Attempts[1].Ordinal = 2
		}},
		{"flaky-permitted-by-policy", "flaky", func(r *RunRegistry, rs *[]TestRunEvidence) {
			r.Policy.Thresholds[0].MaximumFlaky = 1
			r.Policy.Thresholds[0].MaximumFailed = 1
			r.Policy.Thresholds[0].MaximumRetryConsumed = 1
			r.Policy.Thresholds[0].MinimumPassed = 1
			(*rs)[1].Attempts = append((*rs)[1].Attempts, (*rs)[1].Attempts[0])
			(*rs)[1].Attempts[0].Outcome = "failed"
			(*rs)[1].Attempts[1].Ordinal = 2
		}},
		{"cleanup-failed", "cleanup-unverified", func(r *RunRegistry, rs *[]TestRunEvidence) { (*rs)[1].Cleanup = "failed" }},
		{"missing-control", "negative-control-missing", func(_ *RunRegistry, rs *[]TestRunEvidence) { (*rs)[1].NegativeControls = nil }},
		{"duplicate-run", GapStabilityInvalid, func(r *RunRegistry, _ *[]TestRunEvidence) { r.Aggregates[0].Contributions[1].RunID = "r1" }},
		{"manual-does-not-fill-plan", GapStabilityIncomplete, func(r *RunRegistry, _ *[]TestRunEvidence) {
			r.Aggregates[0].Contributions[1].RunKind = runKindManual
			r.Aggregates[0].Contributions[1].Repetition = 0
		}},
	}
	for _, c := range cases {
		t.Run("AFU-V1-015 "+c.name, func(t *testing.T) {
			r := RunRegistry{Schema: RunRegistrySchema, Policy: RunPolicy{ID: "two-clean", Thresholds: []doccorpus.StabilityThreshold{}}, Aggregates: []RunAggregate{{ID: "a", Scope: "one-spec", TestKey: "k", Project: "chromium", Planned: 2, Contributions: []RunContribution{planned("r1", 1), planned("r2", 2)}}}}
			for _, scope := range stabilityScopes {
				r.Policy.Thresholds = append(r.Policy.Thresholds, doccorpus.StabilityThreshold{Scope: scope, RequiredRepetitions: 2, MinimumPassed: 2})
			}
			rs := []TestRunEvidence{stabilityRecord("r1", "k", "done", "passed"), stabilityRecord("r2", "k", "done", "passed")}
			for i := range rs {
				rs[i].NegativeControls = []NegativeControl{{TestKey: "negative", Expected: "failed", Observed: "failed"}}
			}
			root := stabilityRepo(t, r)
			at := head{commit: gitOut(t, root, "rev-parse", "HEAD"), tree: gitOut(t, root, "rev-parse", "HEAD^{tree}")}
			for i := range rs {
				rs[i].Source = RunSource{Commit: at.commit, Tree: at.tree, Clean: true}
			}
			c.edit(&r, &rs)
			// Commit only the registry mutation, then bind successful records to that evaluated source.
			raw, _ := json.Marshal(r)
			writeRaw(t, root, "runs/registry.json", raw)
			if string(raw) != gitOut(t, root, "show", "HEAD:runs/registry.json") {
				commitAll(t, root, "policy case")
				at = head{commit: gitOut(t, root, "rev-parse", "HEAD"), tree: gitOut(t, root, "rev-parse", "HEAD^{tree}")}
				for i := range rs {
					rs[i].Source = RunSource{Commit: at.commit, Tree: at.tree, Clean: true}
				}
			}
			project := "chromium"
			if strings.HasPrefix(c.name, "wildcard-") {
				project = ""
			}
			s := readFlowStability(context.Background(), root, "runs/registry.json", rs, at)
			state, detail := s.qualify("k", project, rs, []string{"negative"}, at)
			if state != c.want {
				t.Fatalf("want %s, got %s (%s)", c.want, state, detail)
			}
			// Dirty policy bytes must never change the committed qualification.
			writeRaw(t, root, "runs/registry.json", []byte(`{}`))
			dirty := readFlowStability(context.Background(), root, "runs/registry.json", rs, at)
			if got, _ := dirty.qualify("k", project, rs, []string{"negative"}, at); got != state {
				t.Fatalf("dirty registry changed state: %s", got)
			}
			commitAll(t, root, "move HEAD to a different policy")
			pinned := readFlowStability(context.Background(), root, "runs/registry.json", rs, at)
			if got, _ := pinned.qualify("k", project, rs, []string{"negative"}, at); got != state {
				t.Fatalf("moving HEAD changed captured qualification: %s", got)
			}

		})
	}
}

func TestAFUV1SelectionInventoryDefaultProject(t *testing.T) {
	for _, c := range []struct {
		name, project string
		valid         bool
	}{{"empty", `,"project":""`, true}, {"missing", "", false}, {"null", `,"project":null`, false}, {"named", `,"project":"chromium"`, true}, {"invalid", `,"project":"\n"`, false}} {
		t.Run("AFU-V1-019 "+c.name, func(t *testing.T) {
			raw := []byte(`{"schema":"application-flow-selection-provider/1","flows":"flows","runner_config":"playwright.config.js","global_paths":[],"inventory":[{"test_key":"counter","path":"e2e.spec.js"` + c.project + `}]}`)
			var p SelectionProvider
			err := Decode(raw, &p)
			if err == nil {
				err = validSelectionProvider(p)
			}
			if (err == nil) != c.valid {
				t.Fatalf("valid=%v: %v", c.valid, err)
			}
		})
	}
}
