package cli_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// keysOf lists an item's keys in the canonical (sorted) wire order.
func keysOf(v wire.Value) string { return strings.Join(slices.Sorted(slices.Values(v.Obj.Keys)), ",") }

func compactOK(t *testing.T, root string, args ...string) run {
	t.Helper()
	x := atm(t, root, nil, args...)
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("%v: %s", args, x.stdout)
	}
	return x
}

func compactRefused(t *testing.T, root string, args ...string) run {
	t.Helper()
	x := atm(t, root, nil, args...)
	if x.code == 0 || x.res.Outcome == wire.OutcomeOK || len(x.res.Items) != 0 || strings.Join(x.res.Codes, ",") != wire.CodeMalformed {
		t.Fatalf("%v was not refused MALFORMED without items: %s", args, x.stdout)
	}
	return x
}

// liveAttempt returns one live attempt's ID and ticket from queue status.
func liveAttempt(t *testing.T, root string) (string, string) {
	t.Helper()
	live := field(compactOK(t, root, "queue", "status").res.Items[0], "liveAttempts").Arr
	if len(live) == 0 {
		t.Fatal("no live attempt")
	}
	return field(live[0], "attemptId").Str, field(live[0], "ticketId").Str
}

// TestCALV0165_FieldsProjectEachItem: --fields keeps the envelope and each
// item's named keys, in request order, one level deep, as a pure read.
func TestCALV0165_FieldsProjectEachItem(t *testing.T) {
	t.Run("CAL-V0-165", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 2, time.Now().UTC().Add(-time.Minute))
		attemptID, ticketID := liveAttempt(t, root)
		before := fixture.TreeSnapshot(t, root)

		full := compactOK(t, root, "ticket", "show", ticketID)
		x := compactOK(t, root, "ticket", "show", "--fields", "status,ticketId,retries.remaining", ticketID)
		item := x.res.Items[0]
		if keysOf(item) != "retries,status,ticketId" || keysOf(field(item, "retries")) != "remaining" ||
			field(item, "ticketId").Str != ticketID || field(item, "status").Str != field(full.res.Items[0], "status").Str {
			t.Fatalf("ticket show projection: %s", x.stdout)
		}
		if x.res.Snapshot == nil || full.res.Snapshot == nil || fmt.Sprint(*x.res.Snapshot.HeadSeq, *x.res.Snapshot.HeadReceiptSha256) != fmt.Sprint(*full.res.Snapshot.HeadSeq, *full.res.Snapshot.HeadReceiptSha256) {
			t.Fatalf("projection changed the envelope snapshot: %s", x.stdout)
		}

		x = compactOK(t, root, "queue", "status", "--fields=headSeq,liveAttempts.attemptId,liveAttempts.holderStatus")
		live := field(x.res.Items[0], "liveAttempts").Arr
		if keysOf(x.res.Items[0]) != "headSeq,liveAttempts" || len(live) != 2 || keysOf(live[0]) != "attemptId,holderStatus" {
			t.Fatalf("queue status projection: %s", x.stdout)
		}

		x = compactOK(t, root, "attempt", "show", attemptID, "--fields", "phase,lease.holder")
		if keysOf(x.res.Items[0]) != "lease,phase" || keysOf(field(x.res.Items[0], "lease")) != "holder" {
			t.Fatalf("attempt show projection: %s", x.stdout)
		}

		x = compactOK(t, root, "plan", "preview", "--fields", "headSeq,entries.ticketId,entries.state")
		for _, e := range field(x.res.Items[0], "entries").Arr {
			if k := keysOf(e); k != "state,ticketId" {
				t.Fatalf("plan entry projection %q: %s", k, x.stdout)
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
			t.Fatal("a projected read wrote files")
		}
	})
}

// TestCALV0166_FieldsRefuseBeforeOutput: malformed selectors refuse
// MALFORMED before any store access; a name the item does not carry refuses
// after the read with no item emitted.
func TestCALV0166_FieldsRefuseBeforeOutput(t *testing.T) {
	t.Run("CAL-V0-166", func(t *testing.T) {
		empty := t.TempDir()
		many := make([]string, 33)
		for i := range many {
			many[i] = fmt.Sprintf("k%d", i)
		}
		for _, args := range [][]string{
			{"queue", "status", "--fields"},
			{"queue", "status", "--fields", ""},
			{"queue", "status", "--fields", "a-b"},
			{"queue", "status", "--fields", "a.b.c"},
			{"queue", "status", "--fields", "a,a"},
			{"queue", "status", "--fields", "a,"},
			{"queue", "status", "--fields", strings.Join(many, ",")},
			{"queue", "status", "--fields", "a", "--fields", "b"},
			{"queue", "status", "--fields", "a", "--summary"},
			{"queue", "status", "--summary", "--summary"},
			{"ticket", "show", "X", "--fields", "--summary"},
			{"attempt", "show", "X", "--fields", "1a"},
			{"plan", "preview", "--summary", "--selected-only"},
		} {
			x := compactRefused(t, empty, args...)
			if x.res.Snapshot != nil || !strings.Contains(strings.Join(x.res.Warnings, " "), "fields") {
				t.Fatalf("%v read the store or lost its location: %s", args, x.stdout)
			}
		}

		root, _ := leaseCLIStore(t, 1, time.Now().UTC().Add(-time.Minute))
		_, ticketID := liveAttempt(t, root)
		before := fixture.TreeSnapshot(t, root)
		for _, args := range [][]string{
			{"queue", "status", "--fields", "queueId,nope"},
			{"ticket", "show", ticketID, "--fields", "status.sub"},
			{"ticket", "show", ticketID, "--fields", "retries.nope"},
			{"plan", "preview", "--fields", "entries.nope"},
		} {
			if x := compactRefused(t, root, args...); x.res.Snapshot == nil {
				t.Fatalf("%v: a post-read refusal keeps the read's snapshot: %s", args, x.stdout)
			}
		}
		x := compactRefused(t, root, "queue", "status", "--fields", "retries")
		if !strings.Contains(strings.Join(x.res.Warnings, " "), "--retries") {
			t.Fatalf("retries refusal names no remedy: %s", x.stdout)
		}
		if y := compactOK(t, root, "queue", "status", "--retries", "--fields", "retries.ticketId"); len(field(y.res.Items[0], "retries").Arr) == 0 {
			t.Fatalf("--retries --fields retries: %s", y.stdout)
		}
		// A non-OK read passes through unchanged.
		bad := atm(t, root, nil, "ticket", "show", "V1-NOPE", "--fields", "status")
		plain := atm(t, root, nil, "ticket", "show", "V1-NOPE")
		if bad.res.Outcome == wire.OutcomeOK || string(bad.stdout) != string(plain.stdout) {
			t.Fatalf("refused read changed under --fields: %s vs %s", bad.stdout, plain.stdout)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
			t.Fatal("a refused projection wrote files")
		}
	})
}

// TestCALV0167_SummaryShapes pins the documented summary item of each read.
func TestCALV0167_SummaryShapes(t *testing.T) {
	t.Run("CAL-V0-167", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 2, time.Now().UTC().Add(-time.Minute))
		attemptID, ticketID := liveAttempt(t, root)
		before := fixture.TreeSnapshot(t, root)

		q := compactOK(t, root, "queue", "status", "--summary").res.Items[0]
		if keysOf(q) != "attempts,barrier,blocked,byStatus,headSeq,liveAttempts,queueId,retriesExhausted,tickets,writeBarrier" ||
			field(q, "retriesExhausted").Str != "0" || field(q, "tickets").Str != "2" {
			t.Fatalf("queue status summary: %s", wire.Encode(q))
		}
		for _, a := range field(q, "liveAttempts").Arr {
			if keysOf(a) != "attemptId,expiresAt,holder,holderStatus,phase,ticketId" {
				t.Fatalf("live attempt summary: %s", wire.Encode(a))
			}
		}
		if y := compactOK(t, root, "queue", "status", "--summary", "--retries").res.Items[0]; keysOf(y) != keysOf(q) {
			t.Fatalf("--summary --retries changed the shape: %s", wire.Encode(y))
		}

		a := compactOK(t, root, "attempt", "show", attemptID, "--summary").res.Items[0]
		if keysOf(a) != "attemptId,expiresAt,generation,holder,holderStatus,lastHeartbeatAt,phase,retryCount,ticketId" ||
			field(a, "attemptId").Str != attemptID || field(a, "holder").Str != "holder" {
			t.Fatalf("attempt summary: %s", wire.Encode(a))
		}

		ts := compactOK(t, root, "ticket", "show", ticketID, "--summary").res.Items[0]
		if keysOf(ts) != "claimabilityReason,claimable,eligibility,nextAction,nextStage,priority,retries,revision,status,ticketId,title" ||
			keysOf(field(ts, "retries")) != "exhausted,remaining" {
			t.Fatalf("ticket summary: %s", wire.Encode(ts))
		}

		p := compactOK(t, root, "plan", "preview", "--summary").res.Items[0]
		if keysOf(p) != "entries,headSeq,mutationAuthority,planningProfile,profile,queueId" || field(p, "profile").Str != "taskman-plan-summary/0" {
			t.Fatalf("plan summary: %s", wire.Encode(p))
		}
		for _, e := range field(p, "entries").Arr {
			if k := keysOf(e); k != "nextStage,reason,state,ticketId" && k != "nextAction,nextStage,reason,state,ticketId" {
				t.Fatalf("plan entry summary %q", k)
			}
		}
		// A pool label spelled like the exclusive flag is a value, not
		// --selected-only.
		if x := atm(t, root, nil, "plan", "preview", "--pool", "--selected-only", "--summary"); strings.Contains(string(x.stdout), "cannot be combined") {
			t.Fatalf("pool value taken as --selected-only: %s", x.stdout)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, root)) {
			t.Fatal("a summary read wrote files")
		}
	})
}

// TestCALV0169_QueueStatusRetriesOptIn: the retry map appears only on
// request; any other argument still refuses.
func TestCALV0169_QueueStatusRetriesOptIn(t *testing.T) {
	t.Run("CAL-V0-169", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 1, time.Now().UTC().Add(-time.Minute))
		plain := compactOK(t, root, "queue", "status").res.Items[0]
		if _, ok := plain.Obj.Get("retries"); ok {
			t.Fatal("queue status emitted retries by default")
		}
		with := compactOK(t, root, "queue", "status", "--retries").res.Items[0]
		if len(field(with, "retries").Arr) == 0 || len(with.Obj.Keys) != len(plain.Obj.Keys)+1 {
			t.Fatalf("--retries: %s", wire.Encode(with))
		}
		for _, args := range [][]string{{"queue", "status", "--retries", "--retries"}, {"queue", "status", "extra"}} {
			compactRefused(t, root, args...)
		}
	})
}

// TestCALV0171_CompactOutputBytesOnLargeQueue measures the default, opt-in
// and compact sizes on a 358-ticket queue and the help sizes; the numbers are
// recorded in the V1-0935 build-log entry.
func TestCALV0171_CompactOutputBytesOnLargeQueue(t *testing.T) {
	t.Run("CAL-V0-171", func(t *testing.T) {
		r := fixture.TempRepo(t)
		fixture.WriteState(t, r)
		policy := fixture.PolicyValue()
		capacity, _ := policy.Obj.Get("capacity")
		capacity.Obj.Set("maxActiveAttempts", wire.String("5"))
		budgets, _ := policy.Obj.Get("budgets")
		budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
		fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), wire.EncodeFile(fixture.QueueValue()))
		fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
		for i := 0; i < 358; i++ {
			local := fmt.Sprintf("L%03d", i)
			rec := fixture.Ticket(local)
			rec.Order = wire.CountOf(int64(i))
			rec.Effects.TouchPaths = []string{fmt.Sprintf("src/%03d.go", i)}
			fixture.Write(t, filepath.Join(r.IntentDir, "tickets", local+".json"), rec.Encode())
		}
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		size := func(args ...string) int { return len(compactOK(t, r.Root, args...).stdout) }
		qRetries, qDefault, qSummary := size("queue", "status", "--retries"), size("queue", "status"), size("queue", "status", "--summary")
		pDefault, pSummary := size("plan", "preview"), size("plan", "preview", "--summary")
		hVerbose, hTerse := size("release", "--help", "--verbose"), size("release", "--help")
		sameStore(t, r, state, intents, "compact reads")

		// ticket show and attempt show are per-item; measure them on a
		// store with live attempts.
		root, _ := leaseCLIStore(t, 2, time.Now().UTC().Add(-time.Minute))
		attemptID, ticketID := liveAttempt(t, root)
		size = func(args ...string) int { return len(compactOK(t, root, args...).stdout) }
		tDefault, tSummary := size("ticket", "show", ticketID), size("ticket", "show", ticketID, "--summary")
		aDefault, aSummary := size("attempt", "show", attemptID), size("attempt", "show", attemptID, "--summary")
		t.Logf("358 tickets: queue status --retries=%d default=%d --summary=%d; plan preview default=%d --summary=%d; ticket show default=%d --summary=%d; attempt show default=%d --summary=%d; release --help --verbose=%d --help=%d",
			qRetries, qDefault, qSummary, pDefault, pSummary, tDefault, tSummary, aDefault, aSummary, hVerbose, hTerse)
		// Each compact form is at most a quarter of its full form (the
		// ticket show item is already small, so at most half).
		if qDefault*4 > qRetries || qSummary*4 > qRetries || pSummary*4 > pDefault || tSummary*2 > tDefault || aSummary*2 > aDefault || hTerse*4 > hVerbose {
			t.Fatal("compact output did not shrink as required")
		}
	})
}

// compactListRepo is a journaled 360-ticket queue: every third ticket is
// COMPLETED and every even one carries the label agent-memory.
func compactListRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	recs := make([]*ticket.Record, 0, 360)
	reason := "done"
	for i := 0; i < 360; i++ {
		rec := fixture.Ticket(fmt.Sprintf("L%03d", i))
		rec.Order = wire.CountOf(int64(i))
		rec.Effects.TouchPaths = []string{fmt.Sprintf("src/%03d.go", i)}
		if i%2 == 0 {
			rec.Labels = []string{"agent-memory"}
		}
		if i%3 == 0 {
			rec.Status = ticket.StatusCompleted
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}
		recs = append(recs, rec)
	}
	cpWrite(t, r, true, recs...)
	return r
}

// TestCALV0173_ListItemsDropTerminalAndStoreWideNoise: list items carry no
// record member, terminal items no blockers or unknowns, and the reader-wide
// attempt-liveness unknown appears once as an envelope warning.
func TestCALV0173_ListItemsDropTerminalAndStoreWideNoise(t *testing.T) {
	t.Run("CAL-V0-173", func(t *testing.T) {
		r := compactListRepo(t)
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		for _, args := range [][]string{{"ticket", "search", "--label", "agent-memory"}, {"ticket", "list"}} {
			x := atm(t, r.Root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 100 {
				t.Fatalf("%v: %s", args, x.stdout)
			}
			sawTerminal, sawOpen, hoistedUnknown := false, false, false
			for _, item := range x.res.Items {
				if _, ok := item.Obj.Get("record"); ok {
					t.Fatalf("%v: list item carries record", args)
				}
				_, hasBlockers := item.Obj.Get("blockers")
				_, hasUnknowns := item.Obj.Get("unknowns")
				switch field(item, "status").Str {
				case ticket.StatusCompleted:
					sawTerminal = true
					if hasBlockers || hasUnknowns {
						t.Fatalf("%v: terminal item carries blockers or unknowns: %s", args, wire.Encode(item))
					}
				case ticket.StatusOpen:
					sawOpen = true
					if !hasBlockers || !hasUnknowns {
						t.Fatalf("%v: open item lost blockers or unknowns: %s", args, wire.Encode(item))
					}
					for _, u := range field(item, "unknowns").Arr {
						if field(u, "code").Str == wire.CodeAttemptLive && field(u, "ticketId").Kind == wire.KindNull {
							t.Fatalf("%v: store-wide unknown repeated on an item", args)
						}
					}
					hoistedUnknown = hoistedUnknown || field(item, "eligibility").Str == "UNKNOWN"
				}
			}
			warned := 0
			for _, w := range x.res.Warnings {
				if strings.Contains(w, "store-wide unknown "+wire.CodeAttemptLive) {
					warned++
				}
			}
			if !sawTerminal || !sawOpen || warned > 1 || (args[1] == "search" && warned != 1) {
				t.Fatalf("%v: terminal=%v open=%v warnings=%d unknownEligibility=%v %v", args, sawTerminal, sawOpen, warned, hoistedUnknown, x.res.Warnings)
			}
		}
		sameStore(t, r, state, intents, "list reads")
	})
}

// TestCALV0174_SearchAndRoadmapCompact: ticket list, ticket search and
// roadmap take --fields and --summary over every paged item; the bytes are
// recorded in the V1-0935 build-log entry.
func TestCALV0174_SearchAndRoadmapCompact(t *testing.T) {
	t.Run("CAL-V0-174", func(t *testing.T) {
		r := compactListRepo(t)
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		search := []string{"ticket", "search", "--label", "agent-memory"}
		s := compactPage(t, r.Root, append(search, "--summary")...)
		for _, item := range s.res.Items {
			if keysOf(item) != "eligibility,nextAction,priority,status,ticketId,title" {
				t.Fatalf("search summary: %s", wire.Encode(item))
			}
		}
		if s.res.Page == nil || s.res.Page.Total == nil || *s.res.Page.Total != "180" {
			t.Fatalf("summary lost the page: %s", s.stdout)
		}
		rm := compactPage(t, r.Root, "roadmap", "--summary")
		for _, item := range rm.res.Items {
			if keysOf(item) != "milestone,nextAction,priority,status,ticketId,title" {
				t.Fatalf("roadmap summary: %s", wire.Encode(item))
			}
		}
		// A key some items lack (terminal blockers) is projected where present.
		f := compactPage(t, r.Root, append(search, "--fields", "ticketId,blockers.code")...)
		with := 0
		for _, item := range f.res.Items {
			if k := keysOf(item); k == "blockers,ticketId" {
				with++
			} else if k != "ticketId" {
				t.Fatalf("search projection %q", k)
			}
		}
		if with == 0 || with == len(f.res.Items) {
			t.Fatalf("expected mixed projection, %d of %d with blockers", with, len(f.res.Items))
		}
		compactRefused(t, r.Root, append(search, "--fields", "record")...)
		compactRefused(t, r.Root, "roadmap", "--fields", "nope")
		// A value of the read's own flag is passed through, not taken as a
		// projection option: this searches for the literal text.
		for _, text := range []string{"--summary", "--fields", "--fields=ticketId"} {
			x := atm(t, r.Root, nil, "ticket", "search", "--text", text)
			if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 0 {
				t.Fatalf("--text %s: %s", text, x.stdout)
			}
		}

		size := func(args ...string) int { return len(compactPage(t, r.Root, args...).stdout) }
		sDefault, sSummary := size(search...), size(append(search, "--summary")...)
		rDefault, rSummary := size("roadmap"), size("roadmap", "--summary")
		lDefault := size("ticket", "list")
		t.Logf("360 tickets, page 100: ticket search --label default=%d --summary=%d; roadmap default=%d --summary=%d; ticket list default=%d",
			sDefault, sSummary, rDefault, rSummary, lDefault)
		if sSummary*3 > sDefault || rSummary*3 > rDefault*2 {
			t.Fatal("list summaries did not shrink as required")
		}
		sameStore(t, r, state, intents, "compact list reads")
	})
}

func compactPage(t *testing.T, root string, args ...string) run {
	t.Helper()
	x := atm(t, root, nil, args...)
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 {
		t.Fatalf("%v: %s", args, x.stdout)
	}
	return x
}
