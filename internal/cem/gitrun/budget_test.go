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
