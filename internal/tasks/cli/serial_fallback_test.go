package cli_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// serialFallbackStore is an initialized fixture store whose policy leaves
// tickets eligible and admits two attempts, so plan preview selects and
// defers them with spare capacity.
func serialFallbackStore(t *testing.T) string {
	t.Helper()
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	capacity, _ := policy.Obj.Get("capacity")
	capacity.Obj.Set("maxActiveAttempts", wire.String("2"))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	return r.Root
}

// effectsTicket creates one ticket with the given priority and effects.
func effectsTicket(t *testing.T, root, title, priority, effects string) (string, run) {
	t.Helper()
	payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+title+`"`, `"priority":"P2"`, `"priority":"`+priority+`"`,
		`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`, `"effects":`+effects).Replace(createPayloadJSON)
	x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("create %s: %s", title, x.stdout)
	}
	return field(x.res.Items[0], "ticketId").Str, x
}

// fallbackDeferred is plan preview's serialFallbackDeferred list, nil when
// the member is absent, after checking that exactly those entries carry
// serialFallback WHOLE_REPOSITORY.
func fallbackDeferred(t *testing.T, root string) []string {
	t.Helper()
	plan := atm(t, root, nil, "plan", "preview")
	if plan.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plan preview: %s", plan.stdout)
	}
	item := plan.res.Items[0]
	var listed []string
	if got, ok := item.Obj.Get("serialFallbackDeferred"); ok {
		if got.Kind != wire.KindArray || len(got.Arr) == 0 {
			t.Fatalf("serialFallbackDeferred = %s", wire.Encode(got))
		}
		for _, v := range got.Arr {
			listed = append(listed, v.Str)
		}
	}
	var marked []string
	for _, e := range field(item, "entries").Arr {
		if fallback, ok := e.Obj.Get("serialFallback"); ok {
			if fallback.Str != "WHOLE_REPOSITORY" || field(e, "reason").Str != "RESOURCE_COLLISION" {
				t.Fatalf("entry serialFallback: %s", wire.Encode(e))
			}
			marked = append(marked, field(e, "ticketId").Str)
		}
	}
	if !slices.Equal(listed, marked) {
		t.Fatalf("listed %q, marked %q", listed, marked)
	}
	return listed
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
		return effectsTicket(t, root, title, "P2", effects)
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
// queue status reports the same list. An entry is not listed when capacity is
// exhausted, when a kept non-PATH resource collides, or when it waits behind a
// WHOLE_REPOSITORY holder, since declaring a PATH scope would not admit it.
func TestCALV0193_SerialFallbackDeferralIsReported(t *testing.T) {
	const scoped = `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`
	const empty = `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]}`
	queueList := func(t *testing.T, root string, args ...string) []string {
		t.Helper()
		q := atm(t, root, nil, append([]string{"queue", "status"}, args...)...)
		if q.res.Outcome != wire.OutcomeOK {
			t.Fatalf("queue status %v: %s", args, q.stdout)
		}
		got := field(q.res.Items[0], "serialFallbackDeferred")
		if got.Kind != wire.KindArray {
			t.Fatalf("queue status %v serialFallbackDeferred = %s", args, wire.Encode(got))
		}
		out := []string{}
		for _, v := range got.Arr {
			out = append(out, v.Str)
		}
		return out
	}

	t.Run("spare capacity then exhausted", func(t *testing.T) {
		root := serialFallbackStore(t)
		effectsTicket(t, root, "top", "P1", scoped)
		whole, _ := effectsTicket(t, root, "whole", "P3", empty)
		if got := fallbackDeferred(t, root); !slices.Equal(got, []string{whole}) {
			t.Fatalf("plan serialFallbackDeferred = %q", got)
		}
		summary := atm(t, root, nil, "plan", "preview", "--summary").res.Items[0]
		if got := field(summary, "serialFallbackDeferred"); len(got.Arr) != 1 || got.Arr[0].Str != whole {
			t.Fatalf("plan summary: %s", wire.Encode(summary))
		}
		for _, args := range [][]string{nil, {"--summary"}} {
			if got := queueList(t, root, args...); !slices.Equal(got, []string{whole}) {
				t.Fatalf("queue status %v = %q", args, got)
			}
		}
		// A second scoped selection uses the last slot: a declared scope
		// would still wait for capacity, so whole is no longer listed.
		effectsTicket(t, root, "mid", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["lib/"]}`)
		if got := fallbackDeferred(t, root); got != nil {
			t.Fatalf("listed with capacity exhausted: %q", got)
		}
		if got := queueList(t, root); len(got) != 0 {
			t.Fatalf("queue status with capacity exhausted = %q", got)
		}
	})

	t.Run("kept non-PATH resource collides", func(t *testing.T) {
		root := serialFallbackStore(t)
		effectsTicket(t, root, "holder", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"DATABASE","key":"db"}],"touchPaths":["src/"]}`)
		effectsTicket(t, root, "dbonly", "P3", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"DATABASE","key":"db"}],"touchPaths":[]}`)
		if got := fallbackDeferred(t, root); got != nil {
			t.Fatalf("listed behind a DATABASE collision: %q", got)
		}
	})

	t.Run("behind a WHOLE_REPOSITORY holder", func(t *testing.T) {
		root := serialFallbackStore(t)
		effectsTicket(t, root, "first", "P0", empty)
		effectsTicket(t, root, "top", "P1", scoped)
		effectsTicket(t, root, "whole", "P3", empty)
		if got := fallbackDeferred(t, root); got != nil {
			t.Fatalf("listed behind a WHOLE_REPOSITORY holder: %q", got)
		}
		if got := queueList(t, root); len(got) != 0 {
			t.Fatalf("queue status behind a WHOLE_REPOSITORY holder = %q", got)
		}
	})
}

// exactDirectoryKeys is the CAL-V0-211 warnings of one result.
func exactDirectoryKeys(x run) []string {
	var out []string
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, "EXACT_KEY_NAMES_DIRECTORY: ") {
			out = append(out, w)
		}
	}
	return out
}

// TestCALV0211_SetEffectsWarnsOnExactDirectoryKey: a fresh SET_EFFECTS whose
// touchPaths or PATH resources hold a key without a trailing "/" that names a
// directory at the checkout's HEAD commits as before and adds one warning per
// such key, naming it and the directory key; file keys, directory keys
// ending in "/" and a replay add none (V1-1118).
func TestCALV0211_SetEffectsWarnsOnExactDirectoryKey(t *testing.T) {
	root := serialFallbackStore(t)
	git(t, root, "init", "-q", "-b", "main")
	fixture.Write(t, filepath.Join(root, "src", "a.go"), []byte("a"))
	fixture.Write(t, filepath.Join(root, "lib", "b.go"), []byte("b"))
	git(t, root, "add", "src", "lib")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
	id, _ := effectsTicket(t, root, "keys", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	setEffects := func(req, rev, effects string) run {
		t.Helper()
		x := atm(t, root, nil, "ticket", "set-effects", "--request-id", req, "--target", id, "--expected-revision", rev, "--issued-at", "2026-10-10T12:00:00Z",
			"--payload", `{"capabilities":[],"effects":`+effects+`,"executionClass":"AUTONOMOUS"}`)
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("set-effects %s: %s", req, x.stdout)
		}
		return x
	}
	exact := `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"lib"}],"touchPaths":["absent","src","src/a.go"]}`
	w := exactDirectoryKeys(setEffects("exact", "1", exact))
	if len(w) != 2 || !strings.Contains(w[0], "PATH key lib names a directory") || !strings.HasSuffix(w[0], "use lib/ to cover them") ||
		!strings.Contains(w[1], "PATH key src names a directory") || !strings.HasSuffix(w[1], "use src/ to cover them") {
		t.Fatalf("warnings = %q", w)
	}
	if replay := setEffects("exact", "1", exact); !field(replay.res.Items[0], "replayed").Bool || len(exactDirectoryKeys(replay)) != 0 {
		t.Fatalf("replay: %s", replay.stdout)
	}
	dirs := `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"lib/"}],"touchPaths":["lib/b.go","src/","src/a.go"]}`
	if x := setEffects("dirs", "2", dirs); len(exactDirectoryKeys(x)) != 0 {
		t.Fatalf("file and directory keys warned: %s", x.stdout)
	}
	// A claim of the exact keys commits as before and carries the same warnings.
	setEffects("exact-again", "3", exact)
	claim := atm(t, root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-exact")
	if claim.res.Outcome != wire.OutcomeOK || len(claim.res.Codes) != 0 || field(claim.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("claim: %s", claim.stdout)
	}
	if w := exactDirectoryKeys(claim); len(w) != 2 || !strings.Contains(w[0], "PATH key lib ") || !strings.Contains(w[1], "PATH key src ") {
		t.Fatalf("claim warnings = %q", w)
	}
}

// TestCALV0214_CreateAndRefineWarnOnExactDirectoryKey covers the advisory
// warning on fresh CREATE and REFINE (including batch entries), without
// rewriting effects or warning on replay (V1-1125).
func TestCALV0214_CreateAndRefineWarnOnExactDirectoryKey(t *testing.T) {
	for _, tc := range []struct {
		name, effects string
		warn          bool
	}{
		{"touch-directory", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src"]}`, true},
		{"resource-directory", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"src"}],"touchPaths":[]}`, true},
		{"duplicate-directory", `{"coverage":"UNKNOWN","externalUnbounded":true,"resources":[{"class":"PATH","key":"src"}],"touchPaths":["src"]}`, true},
		{"files", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"src/a.go"}],"touchPaths":["src/a.go"]}`, false},
		{"trailing-slash", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"PATH","key":"src/"}],"touchPaths":["src/"]}`, false},
		{"absent-and-non-path", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[{"class":"DATABASE","key":"src"},{"class":"PATH","key":"absent"}],"touchPaths":["absent"]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := serialFallbackStore(t)
			git(t, root, "init", "-q", "-b", "main")
			fixture.Write(t, filepath.Join(root, "src", "a.go"), []byte("a"))
			git(t, root, "add", "src")
			git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
			check := func(x run, replay bool) {
				t.Helper()
				if x.res.Outcome != wire.OutcomeOK || len(x.res.Codes) != 0 {
					t.Fatalf("mutation: %s", x.stdout)
				}
				want := 0
				if tc.warn && !replay {
					want = 1
				}
				w := exactDirectoryKeys(x)
				if len(w) != want || (want == 1 && (!strings.HasPrefix(w[0], "EXACT_KEY_NAMES_DIRECTORY: PATH key src ") || !strings.HasSuffix(w[0], "use src/ to cover them"))) {
					t.Fatalf("warnings = %q, want %d", w, want)
				}
			}
			id, created := effectsTicket(t, root, "keys", "P2", tc.effects)
			check(created, false)
			if field(created.res.Items[0], "replayed").Bool {
				t.Fatal("create was not fresh")
			}
			record := func() wire.Value {
				t.Helper()
				x := atm(t, root, nil, "ticket", "show", id)
				if x.res.Outcome != wire.OutcomeOK {
					t.Fatalf("show: %s", x.stdout)
				}
				rec := field(x.res.Items[0], "record")
				if got := string(wire.Encode(field(rec, "effects"))); got != tc.effects {
					t.Fatalf("stored effects = %s, want %s", got, tc.effects)
				}
				return rec
			}
			before := wire.Encode(record())
			_, replay := effectsTicket(t, root, "keys", "P2", tc.effects)
			check(replay, true)
			if !field(replay.res.Items[0], "replayed").Bool || !bytes.Equal(before, wire.Encode(record())) {
				t.Fatal("create replay changed the stored ticket or did not replay")
			}
			for _, batch := range []bool{false, true} {
				args := []string{"ticket", "refine", "--request-id", "refine-keys", "--target", id, "--expected-revision", "1",
					"--issued-at", "2026-10-10T12:00:00Z", "--payload", `{"title":"refined"}`}
				if batch {
					args = []string{"ticket", "refine", "--batch", "--request-id", "batch-keys", "--issued-at", "2026-10-10T12:01:00Z",
						"--payload", `[{"target":"` + id + `","expectedRevision":"2","payload":{"title":"batch refined"}}]`}
				}
				fresh := atm(t, root, nil, args...)
				check(fresh, false)
				entry := fresh.res.Items[0]
				if batch {
					entry = field(entry, "entries").Arr[0]
				}
				if field(entry, "outcome").Str != "COMPLETED" || field(entry, "replayed").Bool {
					t.Fatalf("refine was not a fresh transaction: %s", fresh.stdout)
				}
				before = wire.Encode(record())
				again := atm(t, root, nil, args...)
				check(again, true)
				entry = again.res.Items[0]
				if batch {
					entry = field(entry, "entries").Arr[0]
				}
				if !field(entry, "replayed").Bool || !bytes.Equal(before, wire.Encode(record())) {
					t.Fatalf("refine replay changed the stored ticket or did not replay: %s", again.stdout)
				}
			}
		})
	}
}
