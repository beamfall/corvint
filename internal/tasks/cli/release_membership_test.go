package cli_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TRM-V0-001 to TRM-V0-004 (V1-3193): one release membership mutation adds
// or removes tickets under --expected-revision without restating any other
// release field; every other field is unchanged and a stale revision is
// refused.
func TestTRMV0001_ReleaseMembershipEditsOnlyMembership(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), releaseQueueBytes(false))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	t1 := createTicket(t, r.Root, "member-one")
	t2 := createTicket(t, r.Root, "member-two")
	t3 := createTicket(t, r.Root, "member-three")
	if x := atm(t, r.Root, nil, "release", "create", "--request-id", "rel", "--target", "v1", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1", "Version 1", t1, "")); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %+v", x.res)
	}
	before := releaseShow(t, r.Root)

	// Unsorted repeated --add values are canonicalized at the CLI boundary.
	add := atm(t, r.Root, nil, "release", "membership", "--request-id", "add", "--target", "v1", "--expected-revision", "1", "--issued-at", "2026-09-20T12:02:00Z", "--add", t3, "--add", t2)
	if add.res.Outcome != wire.OutcomeOK || field(add.res.Items[0], "resultingRevision").Str != "2" {
		t.Fatalf("membership add: %+v", add.res)
	}
	afterAdd := releaseShow(t, r.Root)
	assertMembers(t, afterAdd, t1, t2, t3)
	assertOtherFieldsEqual(t, before, afterAdd)

	stale := atm(t, r.Root, nil, "release", "membership", "--request-id", "stale", "--target", "v1", "--expected-revision", "1", "--issued-at", "2026-09-20T12:03:00Z", "--remove", t2)
	if stale.res.Outcome == wire.OutcomeOK || !hasCode(stale.res, wire.CodeStaleTicket) {
		t.Fatalf("stale membership edit was not refused STALE: %+v", stale.res)
	}
	assertMembers(t, releaseShow(t, r.Root), t1, t2, t3)

	for name, args := range map[string][]string{
		"existing member added": {"--add", t1},
		"non-member removed":    {"--remove", "V1-9999"},
		"add and remove same":   {"--add", t2, "--remove", t2},
	} {
		x := atm(t, r.Root, nil, append([]string{"release", "membership", "--request-id", "bad-" + name, "--target", "v1", "--expected-revision", "2", "--issued-at", "2026-09-20T12:03:30Z"}, args...)...)
		if x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s: membership edit succeeded: %+v", name, x.res)
		}
	}
	if x := atm(t, r.Root, nil, "release", "membership", "--request-id", "none", "--target", "v1", "--expected-revision", "2"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("membership without --add/--remove succeeded: %+v", x.res)
	}

	remove := atm(t, r.Root, nil, "release", "membership", "--request-id", "remove", "--target", "v1", "--expected-revision", "2", "--issued-at", "2026-09-20T12:04:00Z", "--remove", t1)
	if remove.res.Outcome != wire.OutcomeOK || field(remove.res.Items[0], "resultingRevision").Str != "3" {
		t.Fatalf("membership remove: %+v", remove.res)
	}
	afterRemove := releaseShow(t, r.Root)
	assertMembers(t, afterRemove, t2, t3)
	assertOtherFieldsEqual(t, before, afterRemove)
	if x := atm(t, r.Root, nil, "receipt", "audit"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %+v", x.res)
	}
}

func releaseShow(t *testing.T, root string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "release", "show", "v1")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("release show: %+v", x.res)
	}
	return x.res.Items[0]
}

func assertMembers(t *testing.T, item wire.Value, want ...string) {
	t.Helper()
	var got []string
	for _, v := range field(item, "ticketIds").Arr {
		got = append(got, v.Str)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("ticketIds = %v, want %v", got, want)
	}
}

// assertOtherFieldsEqual compares every release field but membership and
// the revision bookkeeping a write must change.
func assertOtherFieldsEqual(t *testing.T, before, after wire.Value) {
	t.Helper()
	skip := map[string]bool{"ticketIds": true, "revision": true, "updatedAt": true, "readiness": true, "blockers": true}
	for _, k := range before.Obj.SortedKeys() {
		if skip[k] {
			continue
		}
		b, _ := before.Obj.Get(k)
		a, ok := after.Obj.Get(k)
		if !ok || string(wire.Encode(a)) != string(wire.Encode(b)) {
			t.Fatalf("release field %q changed: %s -> %s", k, wire.Encode(b), wire.Encode(a))
		}
	}
}
