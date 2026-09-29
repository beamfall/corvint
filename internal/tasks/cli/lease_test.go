package cli_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0011_CLIReapReportsActualChildTransactions exercises the real CLI
// boundary over disposable stores containing zero, one, or multiple expired leases.
func TestCALV0011_CLIReapReportsActualChildTransactions(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(map[int]string{0: "CAL-V0-011 zero expired", 1: "CAL-V0-011 one expired", 2: "CAL-V0-011 multiple expired"}[count], func(t *testing.T) {
			root, claimed := expiredCLIStore(t, count)
			result := atm(t, root, nil, "reap", "--request-id", "reap-all")
			if result.res.Outcome != wire.OutcomeOK {
				t.Fatalf("reap: %+v", result.res)
			}
			item := result.res.Items[0]
			if got := field(item, "receipt").Str; got != "" {
				t.Fatalf("parent receipt = %q", got)
			}
			reaped := field(item, "reaped").Arr
			if len(reaped) != count {
				t.Fatalf("reaped = %s, want %d", wire.Encode(item), count)
			}
			receipts, hasReceipts := item.Obj.Get("reapReceipts")
			if count == 0 {
				if hasReceipts || !warningsContain(result.res.Warnings, "the mutation changed nothing; no receipt was written") {
					t.Fatalf("empty reap: item=%s warnings=%v", wire.Encode(item), result.res.Warnings)
				}
			} else {
				if !hasReceipts || len(receipts.Arr) != count || warningsContain(result.res.Warnings, "changed nothing") || !warningsContain(result.res.Warnings, "per-attempt reap transaction") {
					t.Fatalf("completed reap: item=%s warnings=%v", wire.Encode(item), result.res.Warnings)
				}
				want := map[string]string{}
				for _, claim := range claimed {
					want[claim.AttemptID] = string(claim.Generation)
				}
				for _, receipt := range receipts.Arr {
					id, generation, name := field(receipt, "attemptId").Str, field(receipt, "generation").Str, field(receipt, "receipt").Str
					if want[id] != generation || name == "" {
						t.Fatalf("child receipt = %s, want=%v", wire.Encode(receipt), want)
					}
					delete(want, id)
				}
				if len(want) != 0 {
					t.Fatalf("missing child receipts for %v", want)
				}
			}

			retry := atm(t, root, nil, "reap", "--request-id", "reap-retry")
			if retry.res.Outcome != wire.OutcomeOK || len(field(retry.res.Items[0], "reaped").Arr) != 0 || !warningsContain(retry.res.Warnings, "the mutation changed nothing; no receipt was written") {
				t.Fatalf("retry: item=%s warnings=%v", wire.Encode(retry.res.Items[0]), retry.res.Warnings)
			}
			if _, ok := retry.res.Items[0].Obj.Get("reapReceipts"); ok {
				t.Fatal("empty retry reported child receipts")
			}
		})
	}
}

func expiredCLIStore(t *testing.T, count int) (string, []*store.Report) {
	t.Helper()
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	capacity := wire.NewObject().Set("classes", wire.Array()).Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4"))
	policy.Obj.Set("capacity", wire.ObjectValue(capacity))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	baseTime := time.Now().UTC().Add(-20 * time.Minute).Truncate(time.Second)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	initializedAt, err := wire.ParseTimestamp("recordedAt", baseTime.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	actor := mutation.Binding{ID: "tester", Role: "OWNER"}
	if report, err := store.Init(context.Background(), repo, actor, "init-reap", initializedAt); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("init: %+v %v", report, err)
	}
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	tickets := make([]string, 0, count)
	for i := 0; i < count; i++ {
		payload, err := wire.Parse([]byte(createPayloadJSON + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		requestID := "create-reap-" + string(rune('a'+i))
		envelope := wire.NewObject().Set("profile", wire.String(mutation.Profile)).Set("requestId", wire.String(requestID)).Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(actor.ID)).Set("role", wire.String(actor.Role)))).Set("queueId", wire.String(fixture.QueueID)).Set("targetId", wire.Null()).Set("expectedRevision", wire.Null()).Set("operation", wire.String(mutation.OpCreate)).Set("payload", payload).Set("issuedAt", wire.String(string(initializedAt)))
		createdAt, err := wire.ParseTimestamp("recordedAt", baseTime.Add(time.Duration(i+1)*time.Minute).Format("2006-01-02T15:04:05Z"))
		if err != nil {
			t.Fatal(err)
		}
		created, err := store.Mutate(context.Background(), repo, actor, wire.EncodeFile(wire.ObjectValue(envelope)), createdAt)
		if err != nil || created.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("create %d: %+v %v", i, created, err)
		}
		tickets = append(tickets, created.Ticket)
	}
	claimed := make([]*store.Report, 0, count)
	for i, ticketID := range tickets {
		at, err := wire.ParseTimestamp("recordedAt", baseTime.Add(time.Duration(10+i)*time.Minute).Format("2006-01-02T15:04:05Z"))
		if err != nil {
			t.Fatal(err)
		}
		lease := transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: ticketID, Holder: "holder", LeaseMinutes: "5", Scope: []string{"src/" + string(rune('a'+i))}}
		report, err := store.Lease(context.Background(), repo, actor, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-reap-" + string(rune('a'+i)), Root: r.Root, Lease: lease, Derive: store.NoScopeDeriver}, at)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("claim %d: %+v %v", i, report, err)
		}
		claimed = append(claimed, report)
	}
	return r.Root, claimed
}

func warningsContain(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
