package cli_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// listIDs pages through `ticket list` with extra arguments and returns every
// ticket ID in order and the reported total of the first page.
func listIDs(t *testing.T, root string, extra ...string) ([]string, string) {
	t.Helper()
	var ids []string
	total := ""
	for offset := 0; ; offset += 100 {
		x := atm(t, root, nil, append([]string{"ticket", "list", "--offset", fmt.Sprint(offset)}, extra...)...)
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK || x.res.Page == nil || x.res.Page.Total == nil {
			t.Fatalf("list %v at %d: %s", extra, offset, x.stdout)
		}
		if total == "" {
			total = string(*x.res.Page.Total)
		} else if string(*x.res.Page.Total) != total {
			t.Fatalf("list %v: total moved from %s to %s", extra, total, *x.res.Page.Total)
		}
		for _, item := range x.res.Items {
			ids = append(ids, field(item, "ticketId").Str)
		}
		if !x.res.Page.Truncated {
			return ids, total
		}
	}
}

// TestCALV0181_TicketListStatusFilter: --status keeps only the listed
// native statuses, before paging, so pages are stable slices of the same
// order; an unknown, empty, repeated or mis-cased status refuses MALFORMED
// and the read never writes.
func TestCALV0181_TicketListStatusFilter(t *testing.T) {
	t.Run("CAL-V0-181", func(t *testing.T) {
		r := compactListRepo(t)
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		all, total := listIDs(t, r.Root)
		if total != "360" || len(all) != 360 {
			t.Fatalf("unfiltered list: %s %d", total, len(all))
		}
		status := map[string]string{}
		for i, id := range all {
			status[id] = ticket.StatusOpen
			if i%3 == 0 {
				status[id] = ticket.StatusCompleted
			}
		}
		for _, c := range []struct {
			arg   string
			total string
		}{{"COMPLETED", "120"}, {"OPEN", "240"}, {"COMPLETED,OPEN", "360"}, {"OPEN,COMPLETED", "360"}, {"HELD", "0"}, {"DRAFT,HELD,ARCHIVED", "0"}} {
			got, n := listIDs(t, r.Root, "--status", c.arg)
			want := slices.DeleteFunc(slices.Clone(all), func(id string) bool { return !slices.Contains(strings.Split(c.arg, ","), status[id]) })
			if n != c.total || !slices.Equal(got, want) {
				t.Fatalf("--status %s: total %s, %d ids, want %s and the unfiltered order", c.arg, n, len(got), c.total)
			}
		}
		// Every item on a filtered page has a listed status.
		x := atm(t, r.Root, nil, "ticket", "list", "--status=COMPLETED", "--offset", "100")
		if len(x.res.Items) != 20 || x.res.Page.Truncated {
			t.Fatalf("second COMPLETED page: %s", x.stdout)
		}
		for _, item := range x.res.Items {
			if field(item, "status").Str != ticket.StatusCompleted {
				t.Fatalf("filtered item %s", wire.Encode(item))
			}
		}
		// The filter composes with the compact projection.
		f := atm(t, r.Root, nil, "ticket", "list", "--status", "COMPLETED", "--fields", "ticketId,status", "--limit", "5")
		if f.res.Outcome != wire.OutcomeOK || len(f.res.Items) != 5 || keysOf(f.res.Items[0]) != "status,ticketId" || field(f.res.Items[0], "status").Str != ticket.StatusCompleted {
			t.Fatalf("--status with --fields: %s", f.stdout)
		}
		for _, args := range [][]string{
			{"--status", "DONE"},
			{"--status", ""},
			{"--status", "completed"},
			{"--status", "COMPLETED,"},
			{"--status", "COMPLETED,COMPLETED"},
			{"--status", "OPEN", "--status", "COMPLETED"},
			{"--status=OPEN", "--status", "OPEN"},
		} {
			compactRefused(t, r.Root, append([]string{"ticket", "list"}, args...)...)
		}
		sameStore(t, r, state, intents, "ticket list --status")
	})
}

// TestCALV0182_ListTransitionTimesFromTheRecord: completedAt is the
// completion time; statusChangedAt is exact only where the record proves
// its last status transition and UNKNOWN everywhere else.
func TestCALV0182_ListTransitionTimesFromTheRecord(t *testing.T) {
	t.Run("CAL-V0-182", func(t *testing.T) {
		const t2, t3 = wire.Timestamp("2026-09-07T08:00:00Z"), wire.Timestamp("2026-09-08T09:30:00Z")
		reason := "done"
		done := func(at wire.Timestamp) *ticket.Completion {
			return &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: at}
		}
		hold := func(id string, at wire.Timestamp) ticket.Hold {
			return ticket.Hold{HoldID: id, Actor: "op", Reason: "x", PlacedAt: at}
		}
		mk := func(local, status string, rev wire.Count, updated wire.Timestamp, c *ticket.Completion, holds ...ticket.Hold) *ticket.Record {
			rec := fixture.Ticket(local)
			rec.Status, rec.Revision, rec.UpdatedAt, rec.Completion, rec.Holds = status, rev, updated, c, holds
			if rev != "1" {
				d := wire.Sum([]byte("revision 1"))
				rec.PreviousRecordSha256 = &d
			}
			return rec
		}
		recs := []*ticket.Record{
			mk("FRESH", ticket.StatusOpen, "1", fixture.Timestamp, nil),
			mk("DONE", ticket.StatusCompleted, "3", t2, done(t2)),
			mk("EDITED", ticket.StatusCompleted, "4", t3, done(t2)),
			mk("HELD1", ticket.StatusHeld, "2", t2, nil, hold("a", t2)),
			mk("HELD2", ticket.StatusHeld, "3", t3, nil, hold("a", t2), hold("b", t3)),
			mk("TOUCHED", ticket.StatusOpen, "4", t3, nil),
			// Revision 1 proves the creation time only for a status a
			// create can write; this record's completion came later.
			mk("ODD", ticket.StatusCompleted, "1", fixture.Timestamp, done(t2)),
		}
		r := fixture.TempRepo(t)
		cpWrite(t, r, true, recs...)
		x := atm(t, r.Root, nil, "ticket", "list")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != len(recs) {
			t.Fatalf("list: %s", x.stdout)
		}
		want := map[string][2]string{
			"FRESH":   {"null", fixture.Timestamp},
			"DONE":    {string(t2), string(t2)},
			"EDITED":  {string(t2), "UNKNOWN"},
			"HELD1":   {"null", string(t2)},
			"HELD2":   {"null", "UNKNOWN"},
			"TOUCHED": {"null", "UNKNOWN"},
			"ODD":     {string(t2), "UNKNOWN"},
		}
		text := func(v wire.Value) string {
			if v.Kind == wire.KindNull {
				return "null"
			}
			return v.Str
		}
		for _, item := range x.res.Items {
			id := field(item, "ticketId").Str
			w, ok := want[localOf(id)]
			if !ok {
				t.Fatalf("unexpected item %s", id)
			}
			if got := [2]string{text(field(item, "completedAt")), text(field(item, "statusChangedAt"))}; got != w {
				t.Errorf("%s: completedAt/statusChangedAt = %v, want %v", id, got, w)
			}
		}
	})
}

func localOf(id string) string { return id[strings.LastIndex(id, ":")+1:] }

// TestCALV0183_ListLastAttemptEndedAt: a ticket whose latest attempt ended
// carries the recordedAt of the receipt that ended it; a ticket with only a
// live attempt, or none, is null. (ticket list still requires a journal.)
func TestCALV0183_ListLastAttemptEndedAt(t *testing.T) {
	t.Run("CAL-V0-183", func(t *testing.T) {
		// Claims land 10 and 11 minutes after the base with 5-minute
		// leases: both are live now and the release is the newest write.
		root, _ := leaseCLIStore(t, 2, time.Now().UTC().Add(-12*time.Minute))
		attemptID, ticketID := liveAttempt(t, root)
		gen := field(compactOK(t, root, "attempt", "show", attemptID).res.Items[0], "generation").Str
		before := time.Now().UTC().Truncate(time.Second)
		if x := atm(t, root, nil, "release", "--attempt", attemptID, "--generation", gen, "--request-id", "release-x"); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release: %s", x.stdout)
		}
		after := time.Now().UTC()
		x := atm(t, root, nil, "ticket", "list")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 2 {
			t.Fatalf("list: %s", x.stdout)
		}
		for _, item := range x.res.Items {
			v := field(item, "lastAttemptEndedAt")
			if field(item, "ticketId").Str != ticketID {
				if v.Kind != wire.KindNull {
					t.Fatalf("live-only ticket lastAttemptEndedAt = %s", wire.Encode(v))
				}
				continue
			}
			at, err := time.Parse(time.RFC3339, v.Str)
			if err != nil || at.Before(before) || at.After(after) {
				t.Fatalf("released ticket lastAttemptEndedAt = %s (between %s and %s)", wire.Encode(v), before, after)
			}
		}
		// Search items keep their shape: the field is a list-only member.
		s := atm(t, root, nil, "ticket", "search", "--status", "OPEN")
		if s.res.Outcome == wire.OutcomeOK {
			for _, item := range s.res.Items {
				if _, ok := item.Obj.Get("lastAttemptEndedAt"); ok {
					t.Fatalf("search item carries lastAttemptEndedAt: %s", wire.Encode(item))
				}
			}
		}

	})
}

// TestCALV0184_QueueStatusLastCompletionAndWindows: queue status reports
// the newest completion with its receipt and the completions in the last
// hour and 24 hours; a store with none reports null, a receipt it cannot
// prove is UNKNOWN, and an absent journal is NOT_OBSERVED.
func TestCALV0184_QueueStatusLastCompletionAndWindows(t *testing.T) {
	t.Run("CAL-V0-184", func(t *testing.T) {
		status := func(root string, args ...string) wire.Value {
			t.Helper()
			return compactOK(t, root, append([]string{"queue", "status"}, args...)...).res.Items[0]
		}

		// No completion: null, never a guess; zero counts.
		r := fixture.TempRepo(t)
		cpWrite(t, r, true, fixture.Ticket("A"), fixture.Ticket("B"))
		q := status(r.Root)
		if field(q, "lastCompletion").Kind != wire.KindNull || field(field(q, "completions"), "lastHour").Str != "0" || field(field(q, "completions"), "last24Hours").Str != "0" {
			t.Fatalf("no completions: %s", wire.Encode(q))
		}

		// Completions at one instant: the tie goes to the larger ticket ID;
		// the receipt is the one that wrote that record; a month-old
		// completion is outside both windows.
		reason := "done"
		old := func(local string) *ticket.Record {
			rec := fixture.Ticket(local)
			rec.Status = ticket.StatusCompleted
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
			return rec
		}
		tie := fixture.TempRepo(t)
		cpWrite(t, tie, true, old("A"), old("C"), old("B"), fixture.Ticket("D"))
		state, intents := fixture.TreeSnapshot(t, tie.StateDir), fixture.TreeSnapshot(t, tie.IntentDir)
		for _, args := range [][]string{nil, {"--summary"}} {
			q = status(tie.Root, args...)
			lc := field(q, "lastCompletion")
			if keysOf(lc) != "at,receipt,ticketId" || field(lc, "ticketId").Str != fixture.TicketID("C") || field(lc, "at").Str != fixture.Timestamp || field(lc, "receipt").Str != field(q, "headSeq").Str {
				t.Fatalf("%v lastCompletion: %s", args, wire.Encode(q))
			}
			c := field(q, "completions")
			if keysOf(c) != "last24Hours,lastHour,observedAt" || field(c, "lastHour").Str != "0" || field(c, "last24Hours").Str != "0" {
				t.Fatalf("%v completions: %s", args, wire.Encode(c))
			}
		}
		sameStore(t, tie, state, intents, "queue status completions")

		// Archived in the same second it completed: the archive write is the
		// record's last write, so the completion receipt is UNKNOWN, never
		// the archive's (regression for the same-second review finding).
		arch := fixture.TempRepo(t)
		archived := old("Z")
		from := ticket.StatusCompleted
		archived.Status, archived.ArchivedFrom = ticket.StatusArchived, &from
		cpWrite(t, arch, true, old("A"), archived)
		if lc := field(status(arch.Root), "lastCompletion"); field(lc, "ticketId").Str != fixture.TicketID("Z") || field(lc, "receipt").Str != "UNKNOWN" {
			t.Fatalf("archived lastCompletion: %s", wire.Encode(lc))
		}

		// A journal the receipt audit cannot read (this fixture commits 361
		// inline posts in one receipt) leaves the receipt UNKNOWN and the
		// read otherwise unchanged.
		big := compactListRepo(t)
		if lc := field(status(big.Root), "lastCompletion"); field(lc, "ticketId").Str != fixture.TicketID("L357") || field(lc, "receipt").Str != "UNKNOWN" {
			t.Fatalf("unaudited lastCompletion: %s", wire.Encode(lc))
		}

		// Windows: 30 minutes, 5 hours and 30 hours ago. The newest
		// record was edited after it completed, so its receipt is UNKNOWN.
		now := time.Now().UTC()
		ts := func(d time.Duration) wire.Timestamp {
			return wire.Timestamp(now.Add(-d).Format("2006-01-02T15:04:05Z"))
		}
		completed := func(local string, at, updated wire.Timestamp) *ticket.Record {
			rec := fixture.Ticket(local)
			rec.Status, rec.Revision, rec.UpdatedAt = ticket.StatusCompleted, "2", updated
			d := wire.Sum([]byte("revision 1"))
			rec.PreviousRecordSha256 = &d
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: at}
			return rec
		}
		w := fixture.TempRepo(t)
		cpWrite(t, w, true, completed("H", ts(30*time.Minute), ts(10*time.Minute)), completed("D", ts(5*time.Hour), ts(5*time.Hour)), completed("M", ts(30*time.Hour), ts(30*time.Hour)), fixture.Ticket("O"))
		q = status(w.Root)
		lc, c := field(q, "lastCompletion"), field(q, "completions")
		if field(lc, "ticketId").Str != fixture.TicketID("H") || field(lc, "at").Str != string(ts(30*time.Minute)) || field(lc, "receipt").Str != "UNKNOWN" {
			t.Fatalf("windowed lastCompletion: %s", wire.Encode(lc))
		}
		observed, err := time.Parse(time.RFC3339, field(c, "observedAt").Str)
		if err != nil || observed.Before(now.Truncate(time.Second)) || observed.After(time.Now().UTC()) || field(c, "lastHour").Str != "1" || field(c, "last24Hours").Str != "2" {
			t.Fatalf("windowed counts: %s", wire.Encode(c))
		}

		// Without a journal the receipt is NOT_OBSERVED; the record times
		// still count.
		n := fixture.TempRepo(t)
		cpWrite(t, n, false, completed("H", ts(30*time.Minute), ts(30*time.Minute)))
		q = status(n.Root)
		if lc := field(q, "lastCompletion"); field(lc, "receipt").Str != string(ticket.NotObserved) || field(field(q, "completions"), "lastHour").Str != "1" {
			t.Fatalf("no-journal lastCompletion: %s", wire.Encode(q))
		}
	})
}
