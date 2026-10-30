package gitrun

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func TestDefaultBudgetUsesFiniteHangDetector(t *testing.T) {
	t.Run("GPK-V0-003 default Git budget keeps 1024 operations and a 30 minute hang detector", func(t *testing.T) {
		budget := NewDefaultBudget()
		if DefaultOperations != 1024 || budget.remaining != DefaultOperations {
			t.Fatalf("operations constant=%d remaining=%d", DefaultOperations, budget.remaining)
		}
		if DefaultTotalBudget != 30*time.Minute {
			t.Fatalf("total budget=%v", DefaultTotalBudget)
		}
	})
}

func TestReserveOperationPreservesBudgetOrder(t *testing.T) {
	t.Run("PLE-V0-003 logical cache operations retain count and wall limits", func(t *testing.T) {
		b := NewBudget(2, time.Minute)
		span, err := b.ReserveOperation(0)
		if err != nil || span != DefaultPerOpTimeout || b.remaining != 1 {
			t.Fatalf("default %v %v", span, err)
		}
		span, err = b.ReserveOperation(time.Millisecond)
		if err != nil || span != time.Millisecond || b.remaining != 0 {
			t.Fatalf("custom %v %v", span, err)
		}
		b.deadline = time.Now().Add(-time.Second)
		if _, err := b.ReserveOperation(0); cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
			t.Fatalf("count priority %v", err)
		}
		expired := NewBudget(1, -time.Second)
		if _, err := expired.ReserveOperation(0); cemcode.CodeOf(err) != cemcode.GitTimeout || expired.remaining != 0 {
			t.Fatalf("deadline charge %v", err)
		}
		short := NewBudget(1, time.Second)
		span, err = short.ReserveOperation(0)
		if err != nil || span <= 0 || span > time.Second {
			t.Fatalf("wall clamp %v %v", span, err)
		}
	})
}

func TestScaledOperationsGrowWithMapRecords(t *testing.T) {
	t.Run("DCW-V0-373 the verifier operation bound scales with map records and stays finite", func(t *testing.T) {
		if ScaledOperations(0) != DefaultOperations || ScaledOperations(-3) != DefaultOperations {
			t.Fatalf("floor changed: %d %d", ScaledOperations(0), ScaledOperations(-3))
		}
		// The 583-path replay map: 1,687 hunks and 1,687 evidence records.
		if got := ScaledOperations(1687 + 1687); got != DefaultOperations+OperationsPerRecord*3374 || got < 5123 {
			t.Fatalf("replay bound=%d", got)
		}
		b := NewDefaultBudget()
		deadline := b.deadline
		b.ScaleForRecords(10)
		b.ScaleForRecords(0)
		b.ScaleForRecords(-5)
		if b.remaining != DefaultOperations+40 || b.operations != DefaultOperations+40 || !b.deadline.Equal(deadline) {
			t.Fatalf("scaled remaining=%d operations=%d", b.remaining, b.operations)
		}
		if err := b.exhausted(); err.Code != cemcode.GitBudgetExceeded {
			t.Fatalf("exhaustion code %v", err)
		}
		var absent *Budget
		absent.ScaleForRecords(3)
		stable := NewStableBudget(8, time.Now().Add(time.Minute), func() {}, Seam{})
		stable.ScaleForRecords(100)
		if stable.remaining != 8 || stable.operations != 8 {
			t.Fatalf("stable budget grew: %d %d", stable.remaining, stable.operations)
		}
	})
}
