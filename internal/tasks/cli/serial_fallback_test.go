package cli_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// serialFallbackStore is an initialized fixture store whose policy leaves
// tickets eligible, so plan preview selects and defers them.
func serialFallbackStore(t *testing.T) string {
	t.Helper()
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	return r.Root
}

// effectsUnbounded is the CAL-V0-192 warnings of one result.
func effectsUnbounded(x run) []string {
	var out []string
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, "EFFECTS_UNBOUNDED: ") {
			out = append(out, w)
		}
	}
	return out
}

// TestCALV0192_UnboundedEffectsWarn: a fresh CREATE, REFINE or SET_EFFECTS
// whose resulting effects leave the ticket with the WHOLE_REPOSITORY
// fallback scope commits as before and adds one EFFECTS_UNBOUNDED warning;
// a declared PATH scope, an externalUnbounded ticket and a replay add none.
func TestCALV0192_UnboundedEffectsWarn(t *testing.T) {
	root := serialFallbackStore(t)
	create := func(title, effects string) (string, run) {
		t.Helper()
		payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+title+`"`,
			`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`, `"effects":`+effects).Replace(createPayloadJSON)
		x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("create %s: %s", title, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str, x
	}
	const empty = `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`

	whole, x := create("whole", empty)
	w := effectsUnbounded(x)
	if len(w) != 1 || !strings.Contains(w[0], "ticket "+whole+" ") || !strings.Contains(w[0], "coverage QUALIFIED, 0 touchPaths, 0 resources") ||
		!strings.Contains(w[0], "WHOLE_REPOSITORY") || !strings.Contains(w[0], "policy serialFallback BLOCK") {
		t.Fatalf("empty effects warnings = %q", w)
	}
	if _, x := create("scoped", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`); len(effectsUnbounded(x)) != 0 {
		t.Fatalf("declared scope warned: %s", x.stdout)
	}
	// A PATH resource is a declared scope too; coverage UNKNOWN is not.
	if _, x := create("resource", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"lib/"}],"touchPaths":[]}`); len(effectsUnbounded(x)) != 0 {
		t.Fatalf("PATH resource warned: %s", x.stdout)
	}
	if _, x := create("unknown", `{"coverage":"UNKNOWN","externalUnbounded":false,"resources":[],"touchPaths":["docs/"]}`); len(effectsUnbounded(x)) != 1 {
		t.Fatalf("UNKNOWN coverage did not warn: %s", x.stdout)
	}
	if _, x := create("external", `{"coverage":"QUALIFIED","externalUnbounded":true,"resources":[],"touchPaths":[]}`); len(effectsUnbounded(x)) != 0 {
		t.Fatalf("externalUnbounded warned: %s", x.stdout)
	}
	if _, replay := create("whole", empty); !field(replay.res.Items[0], "replayed").Bool || len(effectsUnbounded(replay)) != 0 {
		t.Fatalf("replay: %s", replay.stdout)
	}

	refine := atm(t, root, nil, "ticket", "refine", "--request-id", "refine-whole", "--target", whole, "--expected-revision", "1", "--issued-at", "2026-10-08T12:01:00Z", "--payload", `{"title":"whole refined"}`)
	if refine.res.Outcome != wire.OutcomeOK || len(effectsUnbounded(refine)) != 1 {
		t.Fatalf("refine: %s", refine.stdout)
	}
	setEffects := func(req, rev, effects string) run {
		t.Helper()
		x := atm(t, root, nil, "ticket", "set-effects", "--request-id", req, "--target", whole, "--expected-revision", rev, "--issued-at", "2026-10-08T12:02:00Z",
			"--payload", `{"capabilities":[],"effects":`+effects+`,"executionClass":"AUTONOMOUS"}`)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("set-effects %s: %s", req, x.stdout)
		}
		return x
	}
	if x := setEffects("scope-whole", "2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["app/"]}`); len(effectsUnbounded(x)) != 0 {
		t.Fatalf("set-effects with scope warned: %s", x.stdout)
	}
	if x := setEffects("unscope-whole", "3", empty); len(effectsUnbounded(x)) != 1 {
		t.Fatalf("set-effects to empty did not warn: %s", x.stdout)
	}
}

// TestCALV0193_SerialFallbackDeferralIsReported: plan preview marks an entry
// deferred only by its WHOLE_REPOSITORY fallback scope and lists it, and
// queue status reports the same list; an entry deferred behind a
// WHOLE_REPOSITORY holder, or one that declares its scope, is not listed.
func TestCALV0193_SerialFallbackDeferralIsReported(t *testing.T) {
	root := serialFallbackStore(t)
	planTicket(t, root, "top", "P1", `["src/"]`)
	whole := planTicket(t, root, "whole", "P3", `[]`)

	plan := atm(t, root, nil, "plan", "preview")
	if plan.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plan preview: %s", plan.stdout)
	}
	item := plan.res.Items[0]
	if got := field(item, "serialFallbackDeferred"); got.Kind != wire.KindArray || len(got.Arr) != 1 || got.Arr[0].Str != whole {
		t.Fatalf("serialFallbackDeferred = %s", wire.Encode(got))
	}
	for _, e := range field(item, "entries").Arr {
		fallback, ok := e.Obj.Get("serialFallback")
		if isWhole := field(e, "ticketId").Str == whole; isWhole != ok || (ok && fallback.Str != "WHOLE_REPOSITORY") {
			t.Fatalf("entry serialFallback: %s", wire.Encode(e))
		}
	}
	summary := atm(t, root, nil, "plan", "preview", "--summary").res.Items[0]
	if got := field(summary, "serialFallbackDeferred"); len(got.Arr) != 1 || got.Arr[0].Str != whole {
		t.Fatalf("plan summary: %s", wire.Encode(summary))
	}

	for _, args := range [][]string{{"queue", "status"}, {"queue", "status", "--summary"}} {
		q := atm(t, root, nil, args...)
		if q.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, q.stdout)
		}
		if got := field(q.res.Items[0], "serialFallbackDeferred"); got.Kind != wire.KindArray || len(got.Arr) != 1 || got.Arr[0].Str != whole {
			t.Fatalf("%v serialFallbackDeferred = %s", args, wire.Encode(got))
		}
	}

	// A higher-priority unscoped ticket takes WHOLE_REPOSITORY first: every
	// later entry collides with it whatever it declares, so none is listed.
	planTicket(t, root, "first", "P0", `[]`)
	item = atm(t, root, nil, "plan", "preview").res.Items[0]
	if _, ok := item.Obj.Get("serialFallbackDeferred"); ok {
		t.Fatalf("listed behind a WHOLE_REPOSITORY holder: %s", wire.Encode(item))
	}
	for _, e := range field(item, "entries").Arr {
		if _, ok := e.Obj.Get("serialFallback"); ok {
			t.Fatalf("entry marked behind a WHOLE_REPOSITORY holder: %s", wire.Encode(e))
		}
	}
	q := atm(t, root, nil, "queue", "status").res.Items[0]
	if got := field(q, "serialFallbackDeferred"); got.Kind != wire.KindArray || len(got.Arr) != 0 {
		t.Fatalf("queue status serialFallbackDeferred = %s", wire.Encode(got))
	}
	if !slices.Contains(q.Obj.Keys, "serialFallbackDeferred") {
		t.Fatal("queue status lost serialFallbackDeferred")
	}
}
