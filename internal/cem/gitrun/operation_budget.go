package gitrun

import (
	"context"
	"errors"
	"sync"
	"time"
)

// OperationBudget is opt-in aggregate accounting. Unlike the legacy logical
// Budget, it charges at the physical spawn boundary, including RunReserved.
// ALO-V0-017: nested NewDefaultBudget calls cannot replenish this context budget.
type OperationBudget struct {
	mu                     sync.Mutex
	limit, used, delegated int
	deadline               time.Time
	inheritedGroup         bool
	now                    func() time.Time
}

type operationBudgetKey struct{}

// WithInheritedWorkerGroup is startup-selected by the contained native verifier.
// Verification is outside aggregate acquisition accounting, but its Git children
// must still remain inside the parent's owned process group.
type inheritedWorkerGroupKey struct{}

func WithInheritedWorkerGroup(ctx context.Context) context.Context {
	return context.WithValue(ctx, inheritedWorkerGroupKey{}, true)
}
func inheritsWorkerGroup(ctx context.Context) bool {
	inherited, _ := ctx.Value(inheritedWorkerGroupKey{}).(bool)
	return inherited
}

func NewOperationBudget(limit int, deadline time.Time, inheritedGroup bool) (*OperationBudget, error) {
	if limit < 1 || limit > 64 || deadline.IsZero() || time.Until(deadline) > 300*time.Second {
		return nil, errors.New("aggregate-worker-invalid")
	}
	return &OperationBudget{limit: limit, deadline: deadline, inheritedGroup: inheritedGroup, now: time.Now}, nil
}

func WithOperationBudget(ctx context.Context, budget *OperationBudget) context.Context {
	return context.WithValue(ctx, operationBudgetKey{}, budget)
}

func OperationBudgetFrom(ctx context.Context) *OperationBudget {
	budget, _ := ctx.Value(operationBudgetKey{}).(*OperationBudget)
	return budget
}

func (budget *OperationBudget) reserve(ctx context.Context, perOp time.Duration) (time.Duration, error) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	remaining := budget.deadline.Sub(budget.now())
	if remaining <= 0 {
		return 0, errors.New("aggregate-operation-deadline")
	}
	if budget.used+budget.delegated >= budget.limit {
		return 0, errors.New("aggregate-git-operation-limit")
	}
	if caller, ok := ctx.Deadline(); ok && time.Until(caller) < remaining {
		remaining = time.Until(caller)
	}
	if remaining <= 0 {
		return 0, errors.New("aggregate-operation-deadline")
	}
	if perOp <= 0 || perOp > 10*time.Second {
		perOp = 10 * time.Second
	}
	if remaining < perOp {
		perOp = remaining
	}
	budget.used++
	return perOp, nil
}

// Delegate leaves keep admissions for the parent closing identity checks. An
// unused quota is not a physical spawn; only a verified consumed count settles it.
func (budget *OperationBudget) Delegate(keep int) (int, time.Duration, error) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	remaining := budget.deadline.Sub(budget.now())
	quota := budget.limit - budget.used - keep
	if keep < 0 || budget.delegated != 0 || quota <= 0 || remaining <= 0 {
		return 0, 0, errors.New("aggregate-operation-deadline")
	}
	budget.delegated = quota
	return quota, remaining, nil
}

func (budget *OperationBudget) Settle(consumed int) error {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	if budget.delegated == 0 || consumed < 0 || consumed > budget.delegated {
		return errors.New("aggregate-worker-invalid")
	}
	budget.used += consumed
	budget.delegated = 0
	return nil
}

func (budget *OperationBudget) Used() int {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.used
}

func (budget *OperationBudget) Deadline() time.Time { return budget.deadline }

// OperationOutputFailure preserves the overflowing stream for callers adapting
// the shared executor to an existing error contract. The legacy typed cause is
// still available through errors.As/Unwrap.
type OperationOutputFailure struct {
	Stream string
	Cause  error
}

func (e *OperationOutputFailure) Error() string { return e.Cause.Error() }
func (e *OperationOutputFailure) Unwrap() error { return e.Cause }
