package cli

import (
	"context"
	"fmt"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func (q dispatchQueue) PoolSweepActor() (string, string, error) {
	actor, e := initActor("OPERATOR")
	return actor.ID, actor.Role, e
}

// PoolSweep directly calls the native engine using the dispatcher's operation
// context. It adds no signal handler, command wrapper or alternative confirmation.
func (q dispatchQueue) PoolSweep(ctx context.Context, r dispatch.PoolSweepRequest) (dispatch.PoolSweepResult, error) {
	out := dispatch.PoolSweepResult{}
	actor, e := initActor(r.ActorRole)
	if e != nil {
		return out, e
	}
	if actor.ID != r.Actor {
		return out, fmt.Errorf("pool sweep original actor changed")
	}
	repo, e := intent.Resolve(q.env.Cwd)
	if e != nil {
		return out, e
	}
	work, e := intent.Resolve(r.WorkRoot)
	if e != nil || work.StateDir != repo.StateDir {
		return out, fmt.Errorf("pool sweep original work root differs")
	}
	report, e := store.PoolSweep(ctx, repo, actor, store.PoolSweepChoice{QueueID: r.Queue, RequestID: r.RequestID, Root: r.WorkRoot, Member: r.Member, Allocation: r.Allocation, ExpectedDefinition: wire.Digest(r.Definition), TimeoutSeconds: wire.CountOf(int64(r.TimeoutSeconds))})
	if report != nil {
		out.Pending = report.Pending
		out.Evidence = string(report.Evidence)
		if report.Report != nil {
			out.Receipt = report.Report.Receipt
			out.Outcome = report.Report.Outcome.Outcome
			if report.Report.Outcome.ReceiptSeq != nil {
				out.ReceiptSeq = string(*report.Report.Outcome.ReceiptSeq)
			}
		}
	}
	return out, e
}
