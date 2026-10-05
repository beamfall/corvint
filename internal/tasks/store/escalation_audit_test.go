package store_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
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

// forgeEvent rewrites the one event of an escalation receipt and rehashes
// everything that names it consistently: the evidence path and file, the
// ticket's head digest, the ticket post and the outer chain. The immutable
// origin stays as written; the receipt's retained request entry is rebound
// to the edited request's LEASE digest only when rebind is set.
func forgeEvent(t *testing.T, s *leaseStore, name string, rebind bool, edit func(*ticket.EscalationEvent)) {
	t.Helper()
	forgeReceiptWith(t, s.repo, name, func(string, wire.Value) (wire.Value, bool) { return wire.Value{}, true }, func(v wire.Value) {
		pre, _ := v.Obj.Get("pre")
		post, _ := v.Obj.Get("post")
		var oldDigest, newDigest wire.Digest
		var forged ticket.EscalationRequest
		for i, p := range post.Arr {
			dest, _ := p.Obj.Get("path")
			if !strings.HasPrefix(dest.Str, "evidence/") {
				continue
			}
			ev, err := ticket.DecodeEscalationEvent(mustRead(t, filepath.Join(s.repo.StateDir, dest.Str)))
			if err != nil {
				t.Fatal(err)
			}
			edit(&ev)
			forged = ev.OriginalRequest
			req, err := ticket.EncodeEscalationRequest(ev.OriginalRequest)
			if err != nil {
				t.Fatal(err)
			}
			ev.RequestSha256 = wire.Sum(req)
			raw, err := ticket.EncodeEscalationEvent(ev)
			if err != nil {
				t.Fatal(err)
			}
			oldDigest, newDigest = wire.Digest(strings.TrimPrefix(dest.Str, "evidence/")), wire.Sum(raw)
			p.Obj.Set("path", str("evidence/"+string(newDigest))).Set("sha256", str(string(newDigest))).Set("blobSha256", str(string(newDigest)))
			pre.Arr[i].Obj.Set("path", str("evidence/"+string(newDigest)))
			fixture.Write(t, filepath.Join(s.repo.StateDir, "evidence", string(newDigest)), raw)
		}
		if newDigest == "" {
			t.Fatal("no event to forge")
		}
		if rebind {
			rebindRequest(t, s, post, forged)
		}
		for _, p := range post.Arr {
			dest, _ := p.Obj.Get("path")
			if !strings.HasPrefix(dest.Str, "intent/tickets/") {
				continue
			}
			rec, _ := p.Obj.Get("record")
			next := withRefs(t, rec, func(r *ticket.EscalationRefs) {
				for i := range r.Entries {
					if r.Entries[i].HeadSha256 == oldDigest {
						r.Entries[i].HeadSha256 = newDigest
					}
					if r.Entries[i].OriginSha256 == oldDigest {
						r.Entries[i].OriginSha256 = newDigest
					}
				}
			})
			b := wire.EncodeFile(next)
			p.Obj.Set("record", next).Set("sha256", str(string(wire.Sum(b))))
			fixture.Write(t, filepath.Join(s.repo.PrimaryWorktree, ".taskman", strings.TrimPrefix(dest.Str, "intent/")), b)
		}
		order := make([]int, len(post.Arr))
		for i := range order {
			order[i] = i
		}
		pathOf := func(i int) string { d, _ := post.Arr[i].Obj.Get("path"); return d.Str }
		sort.SliceStable(order, func(a, b int) bool { return pathOf(order[a]) < pathOf(order[b]) })
		var sortedPre, sortedPost []wire.Value
		for _, i := range order {
			sortedPre, sortedPost = append(sortedPre, pre.Arr[i]), append(sortedPost, post.Arr[i])
		}
		v.Obj.Set("pre", wire.Array(sortedPre...)).Set("post", wire.Array(sortedPost...))
	})
}

// rebindRequest rewrites the receipt's request entry post, and its published
// file, so its mutation digest is the LEASE digest of the forged request.
func rebindRequest(t *testing.T, s *leaseStore, post wire.Value, q ticket.EscalationRequest) {
	t.Helper()
	raw, err := ticket.EncodeEscalationRequest(q)
	if err != nil {
		t.Fatal(err)
	}
	verb := map[string]string{"OPEN": transaction.LeaseEscalate, "ANSWER": transaction.LeaseAnswer}[q.Operation]
	digest, err := transaction.Digest(transaction.Request{Operation: transaction.Lease, QueueID: q.QueueID, RequestID: q.RequestID, Actor: mutation.Binding{ID: q.Actor, Role: q.ActorRole}, Lease: &transaction.LeaseRequest{Verb: verb, Evidence: string(wire.Sum(raw))}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range post.Arr {
		dest, _ := p.Obj.Get("path")
		if !strings.HasPrefix(dest.Str, "requests/") {
			continue
		}
		rec, _ := p.Obj.Get("record")
		rec.Obj.Set("mutationSha256", str(string(digest)))
		b := wire.EncodeFile(rec)
		p.Obj.Set("record", rec).Set("sha256", str(string(wire.Sum(b))))
		if file := filepath.Join(s.repo.StateDir, dest.Str); published(file) {
			fixture.Write(t, file, b)
		}
		return
	}
	t.Fatal("no request entry to rebind")
}

func published(path string) bool { _, err := os.Lstat(path); return err == nil }

// TestESCV0010_ConsistentlyRehashedEventIsJournalForked: an answer whose text
// or immutable source is rewritten, with every digest that names it rehashed,
// is refused because the event no longer carries the retained request or the
// question's audited source.
func TestESCV0010_ConsistentlyRehashedEventIsJournalForked(t *testing.T) {
	cases := map[string]struct {
		edit func(*ticket.EscalationEvent)
		want string
	}{
		"answer text":       {func(ev *ticket.EscalationEvent) { ev.OriginalRequest.Answer.Text = "forged answer" }, "other than the retained request"},
		"source generation": {func(ev *ticket.EscalationEvent) { ev.Source.Generation = wire.Size("9") }, "changes its immutable source"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, receipts := escalationHistory(t)
			forgeEvent(t, s, receipts[2], false, c.edit)
			_, err := noteAudit(t, s.repo)
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED naming %q", err, c.want)
			}
		})
	}
}

// TestESCV0010_RetainedRequestPreconditionsAreAudited: a forged answer whose
// retained request entry is rebound to it, so the LEASE digest agrees, is
// still refused when the request carries a stale ticket CAS, a role the
// audited pre-policy does not grant the operation, or a shorthand selector
// the writer would refuse: AMBIGUOUS_OPEN_QUESTIONS with two current OPEN
// questions.
func TestESCV0010_RetainedRequestPreconditionsAreAudited(t *testing.T) {
	t.Run("stale ticket CAS", func(t *testing.T) {
		s, _, receipts := escalationHistory(t)
		stale := wire.Count("1")
		forgeEvent(t, s, receipts[2], true, func(ev *ticket.EscalationEvent) { ev.OriginalRequest.ExpectedTicketRevision = &stale })
		_, err := noteAudit(t, s.repo)
		if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "expected ticket revision") {
			t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED naming the expected ticket revision", err)
		}
	})
	t.Run("role without the policy grant", func(t *testing.T) {
		// The default policy names no OPERATOR row, and the default OPERATOR
		// row lacks ANSWER, so the writer refuses this answer as NOT_ALLOWED.
		s, _, receipts := escalationHistory(t)
		forgeEvent(t, s, receipts[2], true, func(ev *ticket.EscalationEvent) {
			ev.ActorRole, ev.OriginalRequest.ActorRole = "OPERATOR", "OPERATOR"
		})
		forgeReceiptWith(t, s.repo, receipts[2], func(string, wire.Value) (wire.Value, bool) { return wire.Value{}, true }, func(v wire.Value) {
			actor, _ := v.Obj.Get("actor")
			actor.Obj.Set("role", str("OPERATOR"))
		})
		_, err := noteAudit(t, s.repo)
		if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "does not grant ANSWER") {
			t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED naming the missing grant", err)
		}
	})
	t.Run("ambiguous shorthand answer", func(t *testing.T) {
		s, id, src := escalationClaim(t)
		committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
		committed(t, escalate(t, s, holder, openRequest(t, "q-2", src, "", ""), 2), "OPEN")
		ans := answer(t, s, holder, answerRequest(t, "a-1", id, holder, "q-2", "1"), 3)
		committed(t, ans, "ANSWER")
		if _, err := noteAudit(t, s.repo); err != nil {
			t.Fatalf("audit before forgery: %v", err)
		}
		forgeEvent(t, s, ans.Receipt, true, func(ev *ticket.EscalationEvent) {
			ev.OriginalRequest.Answer.RequestID, ev.OriginalRequest.Answer.ExpectedRevision = "", ""
		})
		_, err := noteAudit(t, s.repo)
		if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "exactly one current open question") {
			t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED naming the shorthand selector", err)
		}
	})
}

// TestESCV0010_OpenSourceIsARecordedAdmission: an OPEN whose source and
// retained request are consistently rewritten, with every digest that names
// them rehashed and the request entry rebound, is refused unless the source is
// exactly a walked completed claim admission, as the writer audits it.
func TestESCV0010_OpenSourceIsARecordedAdmission(t *testing.T) {
	cases := map[string]func(*ticket.EscalationSource){
		"receipt digest":      func(src *ticket.EscalationSource) { src.ReceiptSha256 = wire.Sum([]byte("forged")) },
		"post attempt digest": func(src *ticket.EscalationSource) { src.PostAttemptSha256 = wire.Sum([]byte("forged")) },
		"generation":          func(src *ticket.EscalationSource) { src.Generation = wire.Size("9") },
		"not an admission":    func(src *ticket.EscalationSource) { src.ReceiptSequence = wire.Size("1") },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, src := escalationClaim(t)
			open := escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1)
			committed(t, open, "OPEN")
			if _, err := noteAudit(t, s.repo); err != nil {
				t.Fatalf("audit before forgery: %v", err)
			}
			forgeEvent(t, s, open.Receipt, true, func(ev *ticket.EscalationEvent) {
				edit(&ev.Source)
				ev.OriginalRequest.Open.Source = ev.Source
			})
			_, err := noteAudit(t, s.repo)
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "not a recorded claim admission") {
				t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED naming the admission", err)
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
