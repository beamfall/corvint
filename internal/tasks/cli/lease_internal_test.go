package cli

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
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
