package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ExecutionCutover commits the execution cutover (CAL-V0-020): one
// QUALIFICATION receipt whose requestId names the owner's decision posts the
// passing CAL-V0-019 run as evidence and sets the queue's executionCutover,
// after which a non-fixture queue admits claims.
func ExecutionCutover(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID, decision string, run []byte, now wire.Timestamp) (*Report, error) {
	request := transaction.Request{Operation: transaction.Qualification, QueueID: queueID, RequestID: decision, Actor: actor, File: run}
	report, _, err := administrativeWrite(ctx, repo, request, now, nil)
	return report, err
}
