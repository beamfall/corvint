package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func accountingAttempt() *Attempt {
	id, _ := wire.ParseTicketID("ticket", "ticket:acme:main:AT-0001")
	d := wire.Sum(nil)
	budget := map[string]BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		budget[name] = BudgetField{State: "NOT_OBSERVED"}
	}
	return &Attempt{AttemptID: testAttempt, TicketID: id, TicketRevision: "1", TicketRecordSha256: d, Generation: "1", Phase: "RUNNING", PhaseSinceSeq: "1", Mode: "DEVELOPMENT", PolicySha256: d, ConfigSha256: d, RuntimeID: RuntimeExternalAgent, CapabilityProfileSha256: d, BaseCommit: strings.Repeat("a", 40), Branch: "test", Quiescence: "UNPROVED", SpawnNoExecCount: "0", RetryCount: "0", RepairRound: "0", Budget: budget, ScopeCheck: "UNKNOWN", Lease: &Lease{Holder: "agent", GrantedSeq: "1", ExpiresAt: "2026-10-01T00:00:00Z"}, Scope: &Scope{Source: "REQUESTED"}}
}

// CAL-V0-044: old records keep their exact encoding; new metadata is closed,
// canonical and cannot claim clean disposition with inconsistent terminal facts.
func TestCALV0044_AccountingSchema(t *testing.T) {
	t.Run("CAL-V0-044 AccountingSchema", func(t *testing.T) {
		a := accountingAttempt()
		legacy, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		b, err := DecodeAttempt(legacy)
		if err != nil || b.RetryAccounting != nil {
			t.Fatalf("legacy: %v", err)
		}
		again, err := b.Encode()
		if err != nil || !bytes.Equal(legacy, again) {
			t.Fatal("legacy bytes changed")
		}
		a.RetryAccounting = &RetryAccounting{Disposition: "NONE"}
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		b, err = DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		again, err = b.Encode()
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatal("accounting bytes changed")
		}
		for _, bad := range [][]byte{
			bytes.Replace(raw, []byte(ProfileRetryAccounting), []byte("taskman-retry-accounting/99"), 1),
			bytes.Replace(raw, []byte(`"failedOrUnknown":false`), []byte(`"failedOrUnknown":"false"`), 1),
			bytes.Replace(raw, []byte(`"disposition":"NONE"`), []byte(`"disposition":"HANDOFF"`), 1),
			bytes.Replace(raw, []byte(`"disposition":"NONE"`), []byte(`"disposition":"NONE","extra":true`), 1),
		} {
			if _, err := DecodeAttempt(bad); err == nil {
				t.Fatalf("accepted malformed record: %s", bad)
			}
		}
		tree := strings.Repeat("b", 40)
		reason := wire.CodeHandoff
		a.CandidateTreeOid = &tree
		a.ScopeCheck = "WITHIN"
		a.Cause = &reason
		a.Stage = "implement"
		a.Phase = "CANCELLED"
		a.Quiescence = "FENCED"
		a.RetryAccounting.Disposition = reason
		if _, err := a.Encode(); err != nil {
			t.Fatal(err)
		}
		a.RetryAccounting.FailedOrUnknown = true
		if _, err := a.Encode(); err == nil {
			t.Fatal("failed generation encoded as clean")
		}
	})
}

func TestCALV0046_NoTreeHandoffSchema(t *testing.T) {
	t.Run("CAL-V0-046 no-tree terminal schema", func(t *testing.T) {
		fresh := func() *Attempt {
			a := accountingAttempt()
			reason := wire.CodeHandoff
			a.RetryAccounting = &RetryAccounting{Disposition: reason}
			a.HandoffEvidence = "local:review-1"
			a.Stage = "integrate"
			a.Phase = "CANCELLED"
			a.Quiescence = "FENCED"
			a.Cause = &reason
			return a
		}
		a := fresh()
		raw, e := a.Encode()
		if e != nil {
			t.Fatal(e)
		}
		decoded, e := DecodeAttempt(raw)
		if e != nil || decoded.HandoffEvidence != a.HandoffEvidence {
			t.Fatalf("decode: %v", e)
		}
		again, e := decoded.Encode()
		if e != nil || !bytes.Equal(raw, again) {
			t.Fatal("roundtrip changed bytes")
		}
		for name, change := range map[string]func(*Attempt){
			"live":                func(a *Attempt) { a.Phase = "RUNNING" },
			"no-accounting":       func(a *Attempt) { a.RetryAccounting = nil },
			"none":                func(a *Attempt) { a.RetryAccounting.Disposition = "NONE" },
			"failed":              func(a *Attempt) { a.RetryAccounting.FailedOrUnknown = true },
			"supervised":          func(a *Attempt) { a.RuntimeID = SupervisedProfile },
			"ordinary-cancel":     func(a *Attempt) { s := wire.CodeGateFailed; a.Cause = &s },
			"candidate":           func(a *Attempt) { s := strings.Repeat("b", 40); a.CandidateTreeOid = &s; a.ScopeCheck = "WITHIN" },
			"scope":               func(a *Attempt) { a.ScopeCheck = "WITHIN" },
			"gate":                func(a *Attempt) { a.GateResults = []string{"gate"} },
			"effects":             func(a *Attempt) { a.PendingEffects = []string{"pending"} },
			"unfenced":            func(a *Attempt) { a.Quiescence = "UNPROVED" },
			"unstaged":            func(a *Attempt) { a.Stage = "" },
			"returned-integrator": func(a *Attempt) { s := wire.CodeReviewReturned; a.Cause = &s; a.RetryAccounting.Disposition = s },
		} {
			t.Run(name, func(t *testing.T) {
				a := fresh()
				change(a)
				if _, e := a.Encode(); e == nil {
					t.Fatal("invalid terminal binding encoded")
				}
			})
		}
		for _, value := range []string{`null`, `""`, `"bad\nref"`, `"` + strings.Repeat("x", 129) + `"`} {
			bad := bytes.Replace(raw, []byte(`"handoffEvidence":"local:review-1"`), []byte(`"handoffEvidence":`+value), 1)
			if _, e := DecodeAttempt(bad); e == nil {
				t.Fatalf("accepted %s", value)
			}
		}
		bad := bytes.Replace(raw, []byte(`"handoffEvidence":"local:review-1"`), []byte(`"handoffEvidence":"local:review-1","handoffExtra":true`), 1)
		if _, e := DecodeAttempt(bad); e == nil {
			t.Fatal("accepted unknown handoff member")
		}

	})
}
