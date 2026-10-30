package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// RecordRunOutcome commits one RUN_OUTCOME: the outcome document is posted
// under evidence/ and bound to the receipt by its digest (ATR-V0-005).
func RecordRunOutcome(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID, requestID, attemptID string, generation wire.Size, raw []byte, now wire.Timestamp) (*Report, error) {
	lease := transaction.LeaseRequest{Verb: transaction.LeaseRunOutcome, AttemptID: attemptID, Generation: generation, Evidence: string(wire.Sum(raw))}
	request := transaction.Request{Operation: transaction.Lease, QueueID: queueID, RequestID: requestID, Actor: actor, Lease: &lease}
	facts := func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
		return transaction.LeaseFacts{RunOutcome: raw}, nil
	}
	report, _, err := administrativeWriteWith(ctx, repo, request, now, nil, facts)
	return report, err
}

// AttemptRecord reads one attempt's current record from an audited
// snapshot. It writes nothing.
func AttemptRecord(ctx context.Context, repo *intent.Repository, attemptID string) (*snapshot.Attempt, error) {
	proof, err := readLeaseProof(ctx, repo)
	if err != nil {
		return nil, err
	}
	a, ok := lockedAttempt(proof, attemptID)
	if !ok {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "attempt", "attempt %s is not in the audited snapshot", attemptID)
	}
	return a, nil
}

// AttemptRecords reads several attempts' current records from one audited
// snapshot (ATR-V0-015). An attempt the snapshot does not hold, or whose
// record does not decode, is omitted. It writes nothing.
func AttemptRecords(ctx context.Context, repo *intent.Repository, attemptIDs []string) (map[string]*snapshot.Attempt, error) {
	proof, err := readLeaseProof(ctx, repo)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*snapshot.Attempt, len(attemptIDs))
	for _, id := range attemptIDs {
		if a, ok := lockedAttempt(proof, id); ok {
			out[id] = a
		}
	}
	return out, nil
}
