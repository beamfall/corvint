package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// escalationHistory commits OPEN q-1, a supersession of q-1 by q-2 and an
// ANSWER of q-2 through the native writer, returning the three receipts.
func escalationHistory(t *testing.T) (*leaseStore, string, []string) {
	t.Helper()
	s, id, src := escalationClaim(t)
	open := escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1)
	committed(t, open, "OPEN")
	sup := escalate(t, s, holder, openRequest(t, "q-2", src, "q-1", "1"), 2)
	if len(sup.EscalationEvents) != 2 {
		t.Fatalf("supersede %+v", sup)
	}
	ans := answer(t, s, holder, answerRequest(t, "a-1", id, holder, "q-2", "1"), 3)
	committed(t, ans, "ANSWER")
	return s, id, []string{open.Receipt, sup.Receipt, ans.Receipt}
}

// withRefs rewrites a ticket post's escalation reference.
func withRefs(t *testing.T, rec wire.Value, edit func(*ticket.EscalationRefs)) wire.Value {
	t.Helper()
	decoded, err := ticket.Decode(wire.EncodeFile(rec))
	if err != nil {
		t.Fatal(err)
	}
	edit(decoded.Escalations)
	v, err := wire.Parse(decoded.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestESCV0010_ReceiptAuditBindsEscalationHistory: a writer-produced OPEN,
// supersession and ANSWER pass the complete receipt audit (ESC-V0-010). The
// claim's ADMIT posts keep this store's coverage UNKNOWN, so the events'
// own coverage is asserted by the journal's internal test.
func TestESCV0010_ReceiptAuditBindsEscalationHistory(t *testing.T) {
	s, _, _ := escalationHistory(t)
	if _, err := noteAudit(t, s.repo); err != nil {
		t.Fatalf("audit: %v", err)
	}
}

// TestESCV0010_ForgedEscalationHistoryIsJournalForked rewrites one escalation
// receipt with a consistent outer hash chain; each forgery must fail the
// escalation binding.
func TestESCV0010_ForgedEscalationHistoryIsJournalForked(t *testing.T) {
	ticketPost := func(path string) bool { return strings.HasPrefix(path, "intent/tickets/") }
	event := func(path string) bool { return strings.HasPrefix(path, "evidence/") }
	cases := map[string]struct {
		receipt int
		edit    func(t *testing.T, path string, rec wire.Value) (wire.Value, bool)
	}{
		"ticket post differs from the replayed write": {receipt: 0, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withField(rec, "title", str("Forged")), true
			}
			return rec, true
		}},
		"reference changed without its event": {receipt: 0, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			return rec, !event(path)
		}},
		"supersession lost one event": {receipt: 1, edit: func() func(*testing.T, string, wire.Value) (wire.Value, bool) {
			dropped := false
			return func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
				if event(path) && !dropped {
					dropped = true
					return rec, false
				}
				return rec, true
			}
		}()},
		"work revision arithmetic": {receipt: 2, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withRefs(t, rec, func(r *ticket.EscalationRefs) { r.WorkRevision = wire.CountOf(r.WorkRevision.Int() + 1) }), true
			}
			return rec, true
		}},
		"opened entry kind differs from its event": {receipt: 0, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withRefs(t, rec, func(r *ticket.EscalationRefs) { r.Entries[0].Kind = "scope" }), true
			}
			return rec, true
		}},
		"answered entry state differs from its event": {receipt: 2, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withRefs(t, rec, func(r *ticket.EscalationRefs) {
					for i := range r.Entries {
						if r.Entries[i].RequestID == "q-2" {
							r.Entries[i].State = "SUPERSEDED"
						}
					}
				}), true
			}
			return rec, true
		}},
		"untouched entry rewritten": {receipt: 2, edit: func(t *testing.T, path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withRefs(t, rec, func(r *ticket.EscalationRefs) { r.Entries[0].Kind = "scope" }), true
			}
			return rec, true
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, receipts := escalationHistory(t)
			if _, err := noteAudit(t, s.repo); err != nil {
				t.Fatalf("audit before forgery: %v", err)
			}
			forgeReceipt(t, s.repo, receipts[c.receipt], func(path string, rec wire.Value) (wire.Value, bool) { return c.edit(t, path, rec) })
			_, err := noteAudit(t, s.repo)
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "escalation") {
				t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED from the escalation binding", err)
			}
		})
	}
}

// TestESCV0010_RedoBindsAPendingEscalationReceipt: a linked-in OPEN whose head
// and projection were not yet published is refused for redo when its ticket
// post was forged, and nothing is published (the unforged redo is
// TestIssue502_RedoRepublishesTheTicket).
func TestESCV0010_RedoBindsAPendingEscalationReceipt(t *testing.T) {
	s, id, src := escalationClaim(t)
	head := filepath.Join(s.repo.StateDir, "head.json")
	headBefore := mustRead(t, head)
	beforeTicket, _ := escalationRefs(t, s, id)
	open := openRequest(t, "q-1", src, "", "")
	first := escalate(t, s, holder, open, 1)
	committed(t, first, "OPEN")
	parsed, _ := wire.ParseTicketID("id", id)
	projection := filepath.Join(s.repo.PrimaryWorktree, ".taskman", "tickets", parsed.Local+".json")
	fixture.Write(t, head, headBefore)
	forgeReceipt(t, s.repo, first.Receipt, func(path string, rec wire.Value) (wire.Value, bool) {
		if strings.HasPrefix(path, "intent/tickets/") {
			return withRefs(t, rec, func(r *ticket.EscalationRefs) { r.Entries[0].Kind = "scope" }), true
		}
		return rec, true
	})
	fixture.Write(t, projection, beforeTicket.Encode())
	_, err := noteAudit(t, s.repo)
	if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "escalation") {
		t.Fatalf("forged pending escalation receipt audit = %v; want JOURNAL_FORKED from the escalation binding", err)
	}
	if after, _ := escalationRefs(t, s, id); after.Escalations != nil {
		t.Fatalf("forged redo published %+v", after.Escalations)
	}
	if raw, err := os.ReadFile(head); err != nil || string(raw) != string(headBefore) {
		t.Fatalf("head moved: %v", err)
	}
}

// TestESCV0010_CheckpointTailNeverReadsThePrefix: a checkpoint-resumed read
// falls back to the complete audit rather than read a pre-checkpoint receipt
// when an escalation write's pre-state precedes the checkpoint (CAL-V0-061).
func TestESCV0010_CheckpointTailNeverReadsThePrefix(t *testing.T) {
	s, id, src := escalationClaim(t)
	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	full, err := noteAudit(t, s.repo)
	if err != nil {
		t.Fatal(err)
	}
	cp := full.Checkpoint()
	committed(t, answer(t, s, holder, answerRequest(t, "a-1", id, holder, "q-1", "1"), 2), "ANSWER")
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	res, err := (journal.Reader{Source: journal.Native{StateDir: s.repo.StateDir, PrimaryWorktree: s.repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: s.repo.PrimaryWorktree, Checkpoint: cp}).Audit()
	if err != nil {
		t.Fatalf("resumed audit: %v", err)
	}
	if res.Mode != journal.ModeFull {
		t.Fatalf("escalation tail = %s, want the complete audit", res.Mode)
	}
}
