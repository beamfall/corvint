package cli

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0011_ReapSurveyReportsChildTransactions checks that the receiptless
// survey remains visible without describing its completed child reaps as a no-op.
func TestCALV0011_ReapSurveyReportsChildTransactions(t *testing.T) {
	t.Run("CAL-V0-011 empty survey retains no-change warning", func(t *testing.T) {
		res := leaseResult([]string{"reap"}, &store.Report{Kind: "NoChange", Outcome: mutation.Outcome{Outcome: mutation.OutcomeCompleted}})
		if !containsWarning(res.Warnings, "the mutation changed nothing; no receipt was written") {
			t.Fatalf("warnings = %v", res.Warnings)
		}
		if _, ok := res.Items[0].Obj.Get("reapReceipts"); ok {
			t.Fatal("an empty survey reported child receipts")
		}
	})

	t.Run("CAL-V0-011 completed child transactions have aligned receipts", func(t *testing.T) {
		reaped := []transaction.ExpiredLease{{AttemptID: "attempt-1", Generation: "1"}, {AttemptID: "attempt-2", Generation: "3"}}
		receipts := []store.ReapReceipt{{ExpiredLease: reaped[0], Receipt: "000000000010.json"}, {ExpiredLease: reaped[1], Receipt: "000000000011.json"}}
		res := leaseResult([]string{"reap"}, &store.Report{Kind: "NoChange", Outcome: mutation.Outcome{Outcome: mutation.OutcomeCompleted}, Reaped: reaped, ReapReceipts: receipts})
		if containsWarning(res.Warnings, "changed nothing") || !containsWarning(res.Warnings, "2 per-attempt reap transactions completed") {
			t.Fatalf("warnings = %v", res.Warnings)
		}
		item, ok := res.Items[0].Obj.Get("reapReceipts")
		if !ok || len(item.Arr) != 2 {
			t.Fatalf("reapReceipts = %+v", item)
		}
		for i, want := range receipts {
			if objectString(item.Arr[i], "attemptId") != want.AttemptID || objectString(item.Arr[i], "generation") != string(want.Generation) || objectString(item.Arr[i], "receipt") != want.Receipt {
				t.Fatalf("reapReceipts[%d] = %+v, want %+v", i, item.Arr[i], want)
			}
		}
	})

	t.Run("CAL-V0-011 one child uses a singular completed summary", func(t *testing.T) {
		reaped := transaction.ExpiredLease{AttemptID: "attempt-1", Generation: "1"}
		res := leaseResult([]string{"reap"}, &store.Report{
			Kind: "NoChange", Outcome: mutation.Outcome{Outcome: mutation.OutcomeCompleted},
			Reaped:       []transaction.ExpiredLease{reaped},
			ReapReceipts: []store.ReapReceipt{{ExpiredLease: reaped, Receipt: "000000000010.json"}},
		})
		if !containsWarning(res.Warnings, "1 per-attempt reap transaction completed") {
			t.Fatalf("warnings = %v", res.Warnings)
		}
	})
}

func containsWarning(warnings []string, part string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, part) {
			return true
		}
	}
	return false
}

func objectString(value wire.Value, name string) string {
	field, _ := value.Obj.Get(name)
	return field.Str
}

// TestONV0007_ClaimResultCarriesTheDeliveredNote: a claim result always
// names its note state beside the admitted ticket digest; an unresolvable
// pinned event renders UNAVAILABLE with a retry-by-replay warning, never NONE.
func TestONV0007_ClaimResultCarriesTheDeliveredNote(t *testing.T) {
	ok := mutation.Outcome{Outcome: mutation.OutcomeCompleted}
	digest := wire.Digest("ab" + strings.Repeat("0", 62))
	none := leaseResult([]string{"claim"}, &store.Report{Kind: "Claim", Outcome: ok, Delivery: &store.ClaimDelivery{TicketRecordSha256: digest}})
	note, found := none.Items[0].Obj.Get("operatorNote")
	if !found || objectString(note, "state") != "NONE" || objectString(note, "sourceTicketRecordSha256") != string(digest) || !none.Untrusted || containsWarning(none.Warnings, "operator note") {
		t.Fatalf("NONE delivery = %+v untrusted=%v warnings=%v", note, none.Untrusted, none.Warnings)
	}
	ref := &ticket.OperatorNoteReference{Revision: "1", Head: wire.Digest(strings.Repeat("c", 64))}
	missing := wire.Errorf(wire.CodeMissingEvidence, "/operatorNote/head", "absent")
	res := leaseResult([]string{"claim"}, &store.Report{Kind: "Claim", Outcome: ok, Delivery: &store.ClaimDelivery{TicketRecordSha256: digest, OperatorNote: store.ClaimedNote{Reference: ref, Err: missing}}})
	note, _ = res.Items[0].Obj.Get("operatorNote")
	if objectString(note, "state") != "UNAVAILABLE" || objectString(note, "code") != wire.CodeMissingEvidence || !containsWarning(res.Warnings, "replay the exact claim request") {
		t.Fatalf("UNAVAILABLE delivery = %+v warnings=%v", note, res.Warnings)
	}
	if plain := leaseResult([]string{"release"}, &store.Report{Kind: "Release", Outcome: ok}); plain.Untrusted {
		t.Fatal("a non-claim result became untrusted")
	}
}

// TestESCV0005_ClaimResultCarriesPinnedAnswers: a claim result always names
// escalationAnswers beside operatorNote: CURRENT and empty without pins, and
// UNAVAILABLE with the pinned references and a retry-by-replay warning when
// the pinned material does not resolve, never an empty answer list.
func TestESCV0005_ClaimResultCarriesPinnedAnswers(t *testing.T) {
	ok := mutation.Outcome{Outcome: mutation.OutcomeCompleted}
	digest := wire.Digest("ab" + strings.Repeat("0", 62))
	none := leaseResult([]string{"claim"}, &store.Report{Kind: "Claim", Outcome: ok, Delivery: &store.ClaimDelivery{TicketRecordSha256: digest}})
	answers, found := none.Items[0].Obj.Get("escalationAnswers")
	list, _ := answers.Obj.Get("answers")
	if !found || objectString(answers, "state") != "CURRENT" || list.Kind != wire.KindArray || len(list.Arr) != 0 || objectString(answers, "sourceTicketRecordSha256") != string(digest) || containsWarning(none.Warnings, "escalation answers") {
		t.Fatalf("empty delivery = %+v warnings=%v", answers, none.Warnings)
	}
	refs := []snapshot.EscalationAnswerRef{{RequestID: "q-1", OriginSha256: wire.Digest(strings.Repeat("c", 64)), HeadSha256: wire.Digest(strings.Repeat("d", 64))}}
	missing := &transaction.EscalationRefusal{Code: "MISSING_EVIDENCE"}
	res := leaseResult([]string{"claim"}, &store.Report{Kind: "Claim", Outcome: ok, Delivery: &store.ClaimDelivery{TicketRecordSha256: digest, EscalationAnswers: store.ClaimedAnswers{Refs: refs, Err: missing}}})
	answers, _ = res.Items[0].Obj.Get("escalationAnswers")
	list, _ = answers.Obj.Get("answers")
	pinned, _ := answers.Obj.Get("references")
	if objectString(answers, "state") != "UNAVAILABLE" || objectString(answers, "code") != wire.CodeMissingEvidence || list.Kind != wire.KindNull || len(pinned.Arr) != 1 || !containsWarning(res.Warnings, "escalation answers unavailable (MISSING_EVIDENCE)") || !res.Untrusted {
		t.Fatalf("UNAVAILABLE delivery = %+v warnings=%v", answers, res.Warnings)
	}
	forked := leaseResult([]string{"claim"}, &store.Report{Kind: "Claim", Outcome: ok, Delivery: &store.ClaimDelivery{TicketRecordSha256: digest, EscalationAnswers: store.ClaimedAnswers{Refs: refs, Err: &transaction.EscalationRefusal{Code: "ANSWER_BINDING"}}}})
	answers, _ = forked.Items[0].Obj.Get("escalationAnswers")
	if objectString(answers, "code") != wire.CodeJournalForked {
		t.Fatalf("mismatched material = %+v", answers)
	}
}
