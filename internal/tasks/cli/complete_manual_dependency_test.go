package cli_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0186_CompleteManualWarnsOnUnmetCompletedDependency: complete-manual
// of a ticket whose COMPLETED-obligation dependency is OPEN still commits and
// adds one envelope warning naming the dependency and its status; an exact
// replay does not repeat it, and a satisfied dependency adds none.
func TestCALV0186_CompleteManualWarnsOnUnmetCompletedDependency(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	create := func(req, title, dep string) string {
		t.Helper()
		payload := strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"`+title+`"`, 1)
		if dep != "" {
			payload = strings.Replace(payload, `"dependencies":[]`, `"dependencies":[{"gateId":null,"obligation":"COMPLETED","ticketId":"`+dep+`"}]`, 1)
		}
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-10-07T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", title, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	complete := func(req, target string) run {
		t.Helper()
		x := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", req, "--target", target, "--expected-revision", "1", "--issued-at", "2026-10-07T12:01:00Z", "--payload", `{"evidence":[],"reason":"manual"}`)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("complete-manual %s: %s", target, x.stdout)
		}
		return x
	}
	depWarnings := func(x run) []string {
		var out []string
		for _, w := range x.res.Warnings {
			if strings.HasPrefix(w, "COMPLETE_MANUAL overrode an unsatisfied dependency") {
				out = append(out, w)
			}
		}
		return out
	}

	a := create("create-a", "a", "")
	b := create("create-b", "b", a)
	first := complete("complete-b", b)
	want := []string{"COMPLETE_MANUAL overrode an unsatisfied dependency: " + a + " (obligation COMPLETED) is OPEN"}
	if got := depWarnings(first); !slices.Equal(got, want) {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
	if field(first.res.Items[0], "replayed").Bool {
		t.Fatalf("first completion replayed: %s", first.stdout)
	}
	if replay := complete("complete-b", b); !field(replay.res.Items[0], "replayed").Bool || len(depWarnings(replay)) != 0 {
		t.Fatalf("replay: %s", replay.stdout)
	}

	if x := complete("complete-a", a); len(depWarnings(x)) != 0 {
		t.Fatalf("no dependencies, yet warned: %s", x.stdout)
	}
	c := create("create-c", "c", a)
	if x := complete("complete-c", c); len(depWarnings(x)) != 0 {
		t.Fatalf("satisfied dependency, yet warned: %s", x.stdout)
	}
}
