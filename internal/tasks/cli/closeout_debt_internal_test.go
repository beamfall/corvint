package cli

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0004_CitationMatching pins the TCD-V0-004 rules: whole-word short
// and long IDs, case-folded; longer or prefixed tokens never match; issue
// links need a closing keyword and, in owner/repo form, the origin slug; one
// commit citing several tickets is umbrella.
func TestTCDV0004_CitationMatching(t *testing.T) {
	rec := func(local, title, body string) *ticket.Record {
		id, err := wire.ParseTicketID("id", "ticket:corvint:main:"+local)
		if err != nil {
			t.Fatal(err)
		}
		r := &ticket.Record{TicketID: id, Title: title}
		if body != "" {
			r.Body = &body
		}
		return r
	}
	x := &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}, origin: "beamfall/corvint"}
	x.add(rec("V1-1234", "plain", ""))
	x.add(rec("V1-2000", "linked", "Tracks issue #77 and Beamfall/Corvint#88; other/repo#99 is elsewhere."))
	x.add(rec("V1-3000", "Fix it (issue #0123)", ""))
	const a, b, c = "ticket:corvint:main:V1-1234", "ticket:corvint:main:V1-2000", "ticket:corvint:main:V1-3000"
	cases := []struct {
		message string
		want    []closeoutCite
	}{
		{"feat: thing\n\nRefs V1-1234", []closeoutCite{{a, citeDirect, ""}}},
		{"see v1-1234.", []closeoutCite{{a, citeDirect, ""}}},
		{"Refs: ticket:corvint:main:V1-1234:", []closeoutCite{{a, citeDirect, ""}}},
		{"V1-12345 CEM-V1-1234 V1-1234x ticket:other:main:V1-1234 claude/v1-1234-x", nil},
		{"Closes #77", []closeoutCite{{b, citeIssue, "77"}}},
		{"fixed: beamfall/corvint#88", []closeoutCite{{b, citeIssue, "88"}}},
		{"Resolves #123", []closeoutCite{{c, citeIssue, "123"}}},
		{"Closes other/repo#77, Closes #99, mentions #77", nil},
		{"Refs V1-2000\n\nCloses #77", []closeoutCite{{b, citeDirect, "77"}}},
		{"batch V1-1234 and fixes #77", []closeoutCite{{a, citeUmbrella, ""}, {b, citeUmbrella, "77"}}},
	}
	for _, tc := range cases {
		got := x.cites(tc.message)
		sort.Slice(got, func(i, j int) bool { return got[i].ticketID < got[j].ticketID })
		if len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: %+v, want %+v", tc.message, got, tc.want)
		}
	}
	// Without an origin remote only `issue #N` and a bare `#N` count.
	y := &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}
	y.add(rec("V1-2000", "linked", "Tracks beamfall/corvint#88 only."))
	if got := y.cites("Closes #88"); len(got) != 0 {
		t.Errorf("slug link without origin matched: %+v", got)
	}
	if got := truncateUTF8("abé", 3); got != "ab" {
		t.Errorf("truncateUTF8 cut a rune: %q", got)
	}
}

// TestTCDV0007_FanOutKeepsFirstRowsOnly: a fan-out of many tickets linking
// one issue times many commits closing it is counted in full, but the
// collector never holds more than the row bound and keeps the first rows in
// (ticket ID, commit) order whatever the arrival order (TCD-V0-007).
func TestTCDV0007_FanOutKeepsFirstRowsOnly(t *testing.T) {
	const tickets, commits = 40, 50
	var all []closeoutRow
	for c := 0; c < commits; c++ {
		// Commits arrive newest first, in no particular hash order.
		commit := fmt.Sprintf("%040x", (c*7919)%commits)
		for k := tickets - 1; k >= 0; k-- {
			all = append(all, closeoutRow{ticketID: fmt.Sprintf("ticket:corvint:main:V1-%04d", k), commit: commit, kind: citeUmbrella, issue: "77"})
		}
	}
	want := append([]closeoutRow(nil), all...)
	sort.Slice(want, func(i, j int) bool { return closeoutRowLess(want[i], want[j]) })
	for _, max := range []int{0, 1, 7, len(all) - 1, len(all), len(all) + 1} {
		c := closeoutRows{max: max}
		for _, r := range all {
			c.add(r)
			if len(c.kept) > max {
				t.Fatalf("max %d: collector holds %d rows", max, len(c.kept))
			}
		}
		keep := min(max, len(all))
		if got := c.sorted(); c.total != len(all) || len(got) != keep || (keep > 0 && !reflect.DeepEqual(got, want[:keep])) {
			t.Errorf("max %d: total %d, kept %d, want total %d and the first %d rows in order", max, c.total, len(got), len(all), keep)
		}
	}
}
