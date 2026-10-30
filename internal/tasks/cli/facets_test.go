package cli_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// facetPayload is createPayloadJSON with the facet-relevant fields replaced;
// an empty milestone stays null.
func facetPayload(milestone, priority, kind string, labels ...string) string {
	p := createPayloadJSON
	if milestone != "" {
		p = strings.Replace(p, `"milestone":null`, `"milestone":"`+milestone+`"`, 1)
	}
	p = strings.Replace(p, `"priority":"P2"`, `"priority":"`+priority+`"`, 1)
	p = strings.Replace(p, `"kind":"FEATURE"`, `"kind":"`+kind+`"`, 1)
	if len(labels) > 0 {
		p = strings.Replace(p, `"labels":[]`, `"labels":["`+strings.Join(labels, `","`)+`"]`, 1)
	}
	return p
}

// facetRepo is an initialized store with five tickets: two OPEN in v1 (one
// BUG P1 labelled x, one FEATURE P2 labelled x and y), one OPEN in v2, one
// OPEN without a milestone and one DRAFT in v1. It returns the root and the
// created ticket ids in creation order.
func facetRepo(t *testing.T) (string, []string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	var ids []string
	for i, payload := range []string{
		facetPayload("v1", "P1", "BUG", "x"),
		facetPayload("v1", "P2", "FEATURE", "x", "y"),
		facetPayload("v2", "P3", "CHORE"),
		facetPayload("", "P2", "FEATURE"),
		strings.Replace(facetPayload("v1", "P0", "DOC"), `"acceptanceCriteria":["it exists"]`, `"acceptanceCriteria":[]`, 1),
	} {
		x := atm(t, r.Root, nil, "ticket", "create", "--request-id", fmt.Sprintf("facet-%d", i), "--issued-at", "2026-10-09T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %d: %s", i, x.stdout)
		}
		ids = append(ids, field(x.res.Items[0], "ticketId").Str)
	}
	return r.Root, ids
}

// facetsOf returns the encoded facets member of a raw envelope, or nil when
// the envelope has none.
func facetsOf(t *testing.T, stdout []byte) []byte {
	t.Helper()
	v, err := wire.Parse(stdout)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	f, ok := v.Obj.Get("facets")
	if !ok {
		return nil
	}
	return wire.EncodeFile(f)
}

func facetValue(t *testing.T, stdout []byte) wire.Value {
	t.Helper()
	raw := facetsOf(t, stdout)
	if raw == nil {
		t.Fatalf("no facets member: %s", stdout)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	return v
}

// CAL-V0-206: --facets counts the whole match set, independent of the page
// window, deterministically, with every closed enum value present.
func TestCALV0206_FacetsCountTheWholeMatchSet(t *testing.T) {
	root, _ := facetRepo(t)
	search := []string{"ticket", "search", "--milestone", "v1"}
	whole := atm(t, root, nil, append(search, "--facets")...)
	if whole.res.Outcome != wire.OutcomeOK || len(whole.res.Items) != 3 {
		t.Fatalf("search --facets: %s", whole.stdout)
	}
	f := facetValue(t, whole.stdout)
	for key, want := range map[string]string{"total": "3", "withoutMilestone": "0", "milestonesOmitted": "0", "labelsOmitted": "0"} {
		if got := field(f, key).Str; got != want {
			t.Fatalf("%s = %q, want %s: %s", key, got, want, whole.stdout)
		}
	}
	for path, want := range map[[2]string]string{
		{"byStatus", "OPEN"}: "2", {"byStatus", "DRAFT"}: "1", {"byStatus", "ARCHIVED"}: "0",
		{"byPriority", "P0"}: "1", {"byPriority", "P1"}: "1", {"byPriority", "P2"}: "1", {"byPriority", "P3"}: "0",
		{"byKind", "BUG"}: "1", {"byKind", "FEATURE"}: "1", {"byKind", "DOC"}: "1", {"byKind", "SPIKE"}: "0",
		{"byExecutionClass", "AUTONOMOUS"}: "3", {"byExecutionClass", "NEVER"}: "0",
		{"byMilestone", "v1"}: "3", {"byLabel", "x"}: "2", {"byLabel", "y"}: "1",
	} {
		if got := field(field(f, path[0]), path[1]).Str; got != want {
			t.Fatalf("%s.%s = %q, want %s: %s", path[0], path[1], got, want, whole.stdout)
		}
	}
	if n := len(field(f, "byStatus").Obj.Keys); n != 5 {
		t.Fatalf("byStatus lists %d statuses, want all 5", n)
	}
	if n := len(field(f, "byMilestone").Obj.Keys); n != 1 {
		t.Fatalf("byMilestone lists non-matching milestones: %s", whole.stdout)
	}
	elig := field(f, "byEligibility")
	if field(elig, "BLOCKED").Str == "" || field(elig, "UNKNOWN").Str == "" {
		t.Fatalf("byEligibility misses a closed value: %s", whole.stdout)
	}
	// Paging independence and determinism: every window and a repeat run
	// report byte-identical facets.
	want := facetsOf(t, whole.stdout)
	for _, window := range [][]string{{"--limit", "1"}, {"--offset", "1", "--limit", "1"}, {"--offset", "9"}} {
		x := atm(t, root, nil, append(append(append([]string{}, search...), window...), "--facets")...)
		if !bytes.Equal(facetsOf(t, x.stdout), want) {
			t.Fatalf("facets differ under %v:\n%s\nwant %s", window, x.stdout, want)
		}
	}
	// The summary is the same on list and roadmap over the whole store.
	list := atm(t, root, nil, "ticket", "list", "--limit", "2", "--facets")
	road := atm(t, root, nil, "roadmap", "--offset", "3", "--limit", "1", "--facets")
	if field(facetValue(t, list.stdout), "total").Str != "5" || !bytes.Equal(facetsOf(t, list.stdout), facetsOf(t, road.stdout)) {
		t.Fatalf("list and roadmap facets disagree:\n%s\n%s", list.stdout, road.stdout)
	}
	if got := field(field(facetValue(t, road.stdout), "byMilestone"), "v2").Str; got != "1" || field(facetValue(t, road.stdout), "withoutMilestone").Str != "1" {
		t.Fatalf("roadmap milestone facets: %s", road.stdout)
	}
	// Without the flag the envelope keeps its earlier key set.
	for _, args := range [][]string{search, {"ticket", "list"}, {"roadmap"}, append(append([]string{}, search...), "--summary")} {
		x := atm(t, root, nil, args...)
		if facetsOf(t, x.stdout) != nil {
			t.Fatalf("%v rendered facets without the flag: %s", args, x.stdout)
		}
	}
}

// CAL-V0-207: --count returns the summary alone; it refuses a page window
// and a repeated flag, and leaves a filter value spelled --count alone.
func TestCALV0207_CountReturnsOnlyTheSummary(t *testing.T) {
	root, _ := facetRepo(t)
	for _, args := range [][]string{
		{"ticket", "search", "--milestone", "v1", "--status", "OPEN"},
		{"ticket", "list", "--status", "OPEN,DRAFT"},
		{"roadmap"},
	} {
		facets := atm(t, root, nil, append(append([]string{}, args...), "--facets")...)
		count := atm(t, root, nil, append(append([]string{}, args...), "--count")...)
		if count.res.Outcome != wire.OutcomeOK || len(count.res.Items) != 0 || count.res.Page != nil || !count.res.Untrusted {
			t.Fatalf("%v --count: %s", args, count.stdout)
		}
		if !bytes.Equal(facetsOf(t, count.stdout), facetsOf(t, facets.stdout)) {
			t.Fatalf("%v --count facets differ from --facets:\n%s\n%s", args, count.stdout, facets.stdout)
		}
		both := atm(t, root, nil, append(append([]string{}, args...), "--count", "--facets")...)
		if !bytes.Equal(both.stdout, count.stdout) {
			t.Fatalf("%v --count --facets differs from --count: %s", args, both.stdout)
		}
	}
	open := atm(t, root, nil, "ticket", "search", "--milestone", "v1", "--status", "OPEN", "--count")
	if field(facetValue(t, open.stdout), "total").Str != "2" {
		t.Fatalf("open v1 count: %s", open.stdout)
	}
	for _, args := range [][]string{
		{"ticket", "search", "--milestone", "v1", "--count", "--limit", "1"},
		{"ticket", "list", "--count", "--offset", "0"},
		{"roadmap", "--count", "--count"},
		{"roadmap", "--facets", "--facets"},
	} {
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("%v was not refused MALFORMED: %s", args, x.stdout)
		}
	}
	text := atm(t, root, nil, "ticket", "search", "--text", "--count")
	if text.res.Outcome != wire.OutcomeOK || facetsOf(t, text.stdout) != nil || text.res.Page == nil {
		t.Fatalf("--text --count must search for the text: %s", text.stdout)
	}
	empty := atm(t, root, nil, "ticket", "search", "--milestone", "none", "--count")
	if empty.res.Outcome != wire.OutcomeOK || empty.res.Untrusted || field(facetValue(t, empty.stdout), "total").Str != "0" {
		t.Fatalf("empty --count: %s", empty.stdout)
	}
}

// CAL-V0-208 and CAL-V0-209: release readiness reports member counts and
// milestone drift without changing membership, and a candidate-less release
// is assessed without observing the source (this fixture is not a Git
// repository, so a source observation would fail).
func TestCALV0208_ReadinessMemberCountsAndMilestoneDrift(t *testing.T) {
	root, _ := facetRepo(t)
	var ids []string
	for i, payload := range []string{facetPayload("r1", "P1", "FEATURE"), facetPayload("r1", "P2", "BUG"), facetPayload("", "P0", "FEATURE")} {
		x := atm(t, root, nil, "ticket", "create", "--request-id", fmt.Sprintf("release-%d", i), "--issued-at", "2026-10-09T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %d: %s", i, x.stdout)
		}
		ids = append(ids, field(x.res.Items[0], "ticketId").Str)
	}
	payload := fmt.Sprintf(`{"acceptanceCriteria":["release criterion"],"predecessorReleaseIds":[],"requiredGates":[],"ticketIds":[%q,%q],"title":"R1","version":"1"}`, ids[0], ids[2])
	if x := atm(t, root, nil, "release", "create", "--request-id", "r1", "--target", "r1", "--issued-at", "2026-10-09T12:01:00Z", "--payload", payload); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %s", x.stdout)
	}
	before := atm(t, root, nil, "release", "show", "r1")
	ready := atm(t, root, nil, "release", "readiness", "r1")
	if ready.res.Outcome != wire.OutcomeOK {
		t.Fatalf("readiness: %s", ready.stdout)
	}
	item := ready.res.Items[0]
	if field(item, "readiness").Str != "BLOCKED" || len(field(item, "missing").Arr) != 1 || field(item, "missing").Arr[0].Str != "candidate" {
		t.Fatalf("candidate-less readiness: %s", ready.stdout)
	}
	counts := field(item, "memberCounts")
	if field(counts, "total").Str != "2" || field(counts, "absent").Str != "0" || field(field(counts, "byStatus"), "OPEN").Str != "2" ||
		field(field(counts, "byPriority"), "P0").Str != "1" || field(field(counts, "byPriority"), "P1").Str != "1" || field(field(counts, "byPriority"), "P2").Str != "0" {
		t.Fatalf("memberCounts: %s", ready.stdout)
	}
	drift := field(item, "milestoneDrift")
	unfinished := field(drift, "unfinishedNonMembers")
	outside := field(drift, "membersOutsideMilestone")
	if field(drift, "milestone").Str != "r1" ||
		field(unfinished, "count").Str != "1" || len(field(unfinished, "ticketIds").Arr) != 1 || field(unfinished, "ticketIds").Arr[0].Str != ids[1] ||
		field(outside, "count").Str != "1" || len(field(outside, "ticketIds").Arr) != 1 || field(outside, "ticketIds").Arr[0].Str != ids[2] {
		t.Fatalf("milestoneDrift: %s", ready.stdout)
	}
	after := atm(t, root, nil, "release", "show", "r1")
	if !bytes.Equal(before.stdout, after.stdout) {
		t.Fatalf("readiness changed the release:\n%s\n%s", before.stdout, after.stdout)
	}
}
