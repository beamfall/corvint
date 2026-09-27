package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Cutover commits the authority switch (CAL-V0-004, TCP-00 §5.4 A5): one
// AUTHORITY_SWITCH receipt whose requestId names the owner's decision posts
// intent/queue.json with canonicalWriter NATIVE and a CUTOVER write barrier.
// Imported records keep their bytes and provenance and read eligible from then.
func Cutover(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID, decision string, now wire.Timestamp) (*Report, error) {
	request := transaction.Request{Operation: transaction.AuthoritySwitch, QueueID: queueID, RequestID: decision, Actor: actor}
	report, _, err := administrativeWrite(ctx, repo, request, now, nil)
	return report, err
}
