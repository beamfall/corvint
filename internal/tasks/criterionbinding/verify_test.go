package criterionbinding

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

func fixtureCaptures(t *testing.T, base string) (tw.CriterionCapture, *ticket.Record, *snapshot.Attempt) {
	t.Helper()
	rec := fixture.Ticket("AT-0001")
	rec.AcceptanceCriteria = []string{"accepted criterion"}
	d := tw.Sum(fixture.PolicyBytes())
	budget := map[string]snapshot.BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		budget[name] = snapshot.BudgetField{State: "NOT_OBSERVED"}
	}
	a := &snapshot.Attempt{AttemptID: "attempt:acme:main:" + strings.Repeat("a", 32), TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, TicketRecordSha256: rec.FileDigest(), Generation: "1", Phase: "RUNNING", PhaseSinceSeq: "1", Mode: "DEVELOPMENT", PolicySha256: d, ConfigSha256: d, RuntimeID: snapshot.RuntimeExternalAgent, CapabilityProfileSha256: d, BaseCommit: base, Branch: "main", Quiescence: "UNPROVED", SpawnNoExecCount: "0", RetryCount: "0", RepairRound: "0", Budget: budget, ScopeCheck: "UNKNOWN", Lease: &snapshot.Lease{Holder: "test", GrantedSeq: "1", ExpiresAt: "2026-01-01T00:00:00Z"}, Scope: &snapshot.Scope{Source: "REQUESTED", Resources: []ticket.Resource{{Class: "PATH", Key: "sample"}}}}
	return encodeCaptures(t, rec, a, "NONE"), rec, a
}
func encodeCaptures(t *testing.T, rec *ticket.Record, a *snapshot.Attempt, barrier string) tw.CriterionCapture {
	t.Helper()
	d := tw.Digest(strings.Repeat("b", 64))
	seq := tw.Size("1")
	snap := &tw.Snapshot{HeadSeq: &seq, HeadReceiptSha256: &d, IntentTreeSha256: &d, PrimaryWorktreeSha256: &d}
	wrap := func(cmd []string, item tw.Value) string {
		r := tw.Result{Command: cmd, Outcome: tw.OutcomeOK, Snapshot: snap, Items: []tw.Value{item}}
		b, e := r.Encode()
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	raw, e := a.Encode()
	if e != nil {
		t.Fatal(e)
	}
	av, e := tw.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	policy, e := intent.DecodePolicy(fixture.PolicyBytes())
	if e != nil {
		t.Fatal(e)
	}
	q := tw.ObjectValue(tw.NewObject().Set("queueId", tw.String(rec.TicketID.QueueID())).Set("policySha256", tw.String(string(policy.PolicySha256()))).Set("barrier", tw.Null()).Set("writeBarrier", tw.String(barrier)).Set("executionCutover", tw.Bool(true)))
	return tw.CriterionCapture{ClaimedTicket: string(tw.EncodeFile(rec.Value())), Producer: tw.CriterionIdentity{ExecutableSHA256: tw.Digest(strings.Repeat("a", 64)), Version: "dev", Build: "0"}, Ticket: wrap([]string{"ticket", "show"}, tw.ObjectValue(tw.NewObject().Set("record", rec.Value()))), Queue: wrap([]string{"queue", "status"}, q), Attempt: wrap([]string{"attempt", "show"}, av), Policy: string(policy.Raw)}
}

func TestNativeCaptureVerification(t *testing.T) {
	c, _, _ := fixtureCaptures(t, strings.Repeat("a", 40))
	raw := tw.EncodeFile(c.Value())
	v, e := Verify(raw, c.Producer)
	if e != nil {
		t.Fatal(e)
	}
	if v.Binding.TicketID.Raw == "" || v.CaptureSHA256 != tw.Sum(raw) {
		t.Fatal("binding")
	}
	for name, mutate := range map[string]func(*tw.CriterionCapture){"command": func(c *tw.CriterionCapture) { c.Ticket = strings.Replace(c.Ticket, `"show"`, `"delete"`, 1) }, "snapshot": func(c *tw.CriterionCapture) { c.Queue = strings.Replace(c.Queue, `"headSeq":"1"`, `"headSeq":"2"`, 1) }, "pending": func(c *tw.CriterionCapture) {
		c.Queue = strings.Replace(c.Queue, `"pendingRedo":false`, `"pendingRedo":true`, 1)
	}, "policy": func(c *tw.CriterionCapture) { c.Policy = "{}\n" }, "record": func(c *tw.CriterionCapture) {
		c.Attempt = strings.Replace(c.Attempt, `"generation":"1"`, `"generation":"0x1"`, 1)
	}} {
		t.Run(name, func(t *testing.T) {
			bad := c
			mutate(&bad)
			if _, e := Verify(tw.EncodeFile(bad.Value()), c.Producer); e == nil {
				t.Fatal("admitted malformed capture")
			}
		})
	}
}

func TestClaimedAcceptanceAndSequenceBindings(t *testing.T) {
	c, rec, a := fixtureCaptures(t, strings.Repeat("a", 40))
	claimed := c.ClaimedTicket
	for name, change := range map[string]func(){
		"criteria": func() { rec.AcceptanceCriteria = []string{"substituted"} },
		"phase":    func() { a.PhaseSinceSeq = "999" },
		"lease":    func() { a.Lease.GrantedSeq = "999" },
	} {
		t.Run(name, func(t *testing.T) {
			c, rec, a = fixtureCaptures(t, strings.Repeat("a", 40))
			change()
			bad := encodeCaptures(t, rec, a, "NONE")
			bad.ClaimedTicket = claimed
			if _, e := Verify(tw.EncodeFile(bad.Value()), c.Producer); e == nil {
				t.Fatal("contradictory claim admitted")
			}
		})
	}
	c, rec, a = fixtureCaptures(t, strings.Repeat("a", 40))
	body := "legitimate body edit"
	rec.Body = &body
	rec.Revision = "2"
	previous := a.TicketRecordSha256
	rec.PreviousRecordSha256 = &previous
	good := encodeCaptures(t, rec, a, "NONE")
	good.ClaimedTicket = claimed
	if _, e := Verify(tw.EncodeFile(good.Value()), c.Producer); e != nil {
		t.Fatalf("body-only drift refused: %v", e)
	}
	good.ClaimedTicket = ""
	if _, e := Verify(tw.EncodeFile(good.Value()), c.Producer); e == nil {
		t.Fatal("missing claimed record admitted")
	}
	good.ClaimedTicket = string(tw.EncodeFile(rec.Value()))
	if _, e := Verify(tw.EncodeFile(good.Value()), c.Producer); e == nil {
		t.Fatal("wrong claim digest admitted")
	}
}
