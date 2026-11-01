// SPDX-License-Identifier: Apache-2.0

package mutation_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func progressMark(phase string, kv ...string) wire.Value {
	o := obj("branch", wire.Null(), "head", wire.Null(), "host", wire.Null(), "lane", wire.Null(), "phase", str(phase), "pr", wire.Null(), "reason", wire.Null())
	for i := 0; i+1 < len(kv); i += 2 {
		o.Obj.Set(kv[i], str(kv[i+1]))
	}
	return o
}

var progressHead = strings.Repeat("b", 40)

// TestTLFV0880_ProgressSetLeavesTheRecordUnchanged: PROGRESS_SET appends one
// entry from the trusted binding and clock and changes nothing else; an
// OPERATOR holds it by default and a live attempt does not block it.
func TestTLFV0880_ProgressSetLeavesTheRecordUnchanged(t *testing.T) {
	pre := fixture.Ticket("AT-01")
	live := attempts{fixture.TicketID("AT-01"): true}
	post := step(t, operator, live, pre, mutation.OpProgressSet, progressMark("IN_REVIEW", "lane", "w4", "pr", "42"), "1")
	c := *post
	c.Progress = nil
	c.Revision, c.PreviousRecordSha256, c.UpdatedAt, c.UpdatedBy = pre.Revision, pre.PreviousRecordSha256, pre.UpdatedAt, pre.UpdatedBy
	if !bytes.Equal(c.Encode(), pre.Encode()) {
		t.Fatalf("progress changed the record beyond progress:\n%s\n%s", pre.Encode(), post.Encode())
	}
	if len(post.Progress) != 1 {
		t.Fatalf("entries: %+v", post.Progress)
	}
	e := post.Progress[0]
	if e.Phase != "IN_REVIEW" || e.Actor != operator.ID || e.RecordedAt != now || e.PR == nil || *e.PR != "42" || *e.Lane != "w4" || e.Host != nil {
		t.Fatalf("entry: %+v", e)
	}
	back, err := ticket.Decode(post.Encode())
	if err != nil || !bytes.Equal(back.Encode(), post.Encode()) {
		t.Fatalf("round trip: %v", err)
	}
	post2 := step(t, owner, nil, post, mutation.OpProgressSet, progressMark("MERGED_AWAITING_CLOSEOUT", "head", progressHead), "1")
	if len(post2.Progress) != 2 || post2.EffectivePhase() != "MERGED_AWAITING_CLOSEOUT" {
		t.Fatalf("second entry: %+v", post2.Progress)
	}
}

// TestTLFV0880_ProgressSetRefusals: live native tickets only, per-phase
// members, duplicate and bound refusals, and no role beyond OWNER/OPERATOR.
func TestTLFV0880_ProgressSetRefusals(t *testing.T) {
	ok := progressMark("IN_PROGRESS", "lane", "w4")
	open := fixture.Ticket("AT-01")
	d := wire.Sum([]byte("log"))
	closed := map[string]*ticket.Record{
		"ARCHIVED":  step(t, owner, nil, open, mutation.OpArchive, obj("reason", str("gone")), "1"),
		"COMPLETED": step(t, owner, nil, open, mutation.OpCompleteManual, obj("evidence", wire.Strings([]string{string(d)}), "reason", str("done")), "2"),
	}
	for name, rec := range closed {
		refused(t, owner, rec, mutation.OpProgressSet, ok, mutation.OutcomeBlocked, wire.CodeTicketState)
		_ = name
	}
	held := step(t, owner, nil, open, mutation.OpHold, obj("holdId", str("h"), "reason", str("wait")), "1")
	if got := step(t, owner, nil, held, mutation.OpProgressSet, progressMark("BLOCKED", "reason", "waiting on V1-1"), "1"); got.EffectivePhase() != "BLOCKED" {
		t.Fatalf("HELD ticket: %+v", got.Progress)
	}
	imported := fixture.Ticket("AT-01")
	src := "X-1"
	imported.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:ext:src", SourceItemID: &src}
	refused(t, owner, imported, mutation.OpProgressSet, ok, mutation.OutcomeBlocked, wire.CodeTicketState)

	rev := string(open.Revision)
	bad := map[string]wire.Value{
		"delivered without head": progressMark("DELIVERED_ON_BRANCH", "branch", "claude/x"),
		"review without pr":      progressMark("IN_REVIEW"),
		"merged without head":    progressMark("MERGED_AWAITING_CLOSEOUT"),
		"blocked without reason": progressMark("BLOCKED"),
		"cleared with lane":      progressMark("CLEARED", "lane", "w4"),
		"unknown phase":          progressMark("DONE"),
		"bad head":               progressMark("MERGED_AWAITING_CLOSEOUT", "head", "xyz"),
		"blank reason":           progressMark("BLOCKED", "reason", "  "),
		"missing key":            obj("phase", str("IN_PROGRESS")),
	}
	for name, p := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpProgressSet, p)); err == nil {
				t.Fatalf("payload decoded: %s", wire.Encode(p))
			}
		})
	}
	once := step(t, owner, nil, open, mutation.OpProgressSet, ok, "1")
	refused(t, owner, once, mutation.OpProgressSet, ok, mutation.OutcomeValidationFailed, wire.CodeDuplicateID)

	full := fixture.Ticket("AT-01")
	for i := 0; i < wire.ProgressMaxEntries; i++ {
		lane := "l" + strings.Repeat("x", i%8)
		full.Progress = append(full.Progress, ticket.Progress{ProgressMark: ticket.ProgressMark{Phase: "IN_PROGRESS", Lane: &lane}, Actor: "russell", RecordedAt: now})
	}
	if _, err := ticket.Decode(full.Encode()); err != nil {
		t.Fatalf("64 entries: %v", err)
	}
	refused(t, owner, full, mutation.OpProgressSet, progressMark("BLOCKED", "reason", "r"), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)

	for _, b := range []mutation.Binding{worker, reviewer, importer, system} {
		refused(t, b, open, mutation.OpProgressSet, ok, mutation.OutcomeUnauthorized, "")
	}
	ctx := newCtx(t, owner, nil, open)
	want(t, apply(t, ctx, envelope("cas", owner, "AT-01", "7", mutation.OpProgressSet, ok)), mutation.OutcomeRevisionConflict, "")
}

// TestTLFV0880_AdoptFileRefusesProgress: a hand edit cannot add, drop or
// rewrite progress entries.
func TestTLFV0880_AdoptFileRefusesProgress(t *testing.T) {
	marked := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpProgressSet, progressMark("IN_PROGRESS", "lane", "w4"), "1")
	lane := "w9"
	cases := map[string]struct {
		canonical *ticket.Record
		edit      func(*ticket.Record)
	}{
		"add": {fixture.Ticket("AT-01"), func(r *ticket.Record) {
			r.Progress = []ticket.Progress{{ProgressMark: ticket.ProgressMark{Phase: "IN_PROGRESS", Lane: &lane}, Actor: "russell", RecordedAt: now}}
		}},
		"drop":    {marked, func(r *ticket.Record) { r.Progress = nil }},
		"rewrite": {marked, func(r *ticket.Record) { r.Progress[0].Lane = &lane }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			before := string(c.canonical.Encode())
			ctx := newCtx(t, owner, nil, c.canonical)
			plan := adopt(t, ctx, c.canonical, fileOf(t, c.canonical, c.edit))
			want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
			if !strings.Contains(plan.Detail, "progress") || len(plan.Composed) != 0 {
				t.Fatalf("not refused as a protected field: %q %v", plan.Detail, plan.Composed)
			}
			stillDiverged(t, plan, c.canonical, before)
		})
	}
}
