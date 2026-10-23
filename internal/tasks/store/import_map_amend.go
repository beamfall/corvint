package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ImportMapRequest is one OWNER import map amendment (CTS-V0-021). ImportMap
// is the complete amended canonical map; ExpectedImportMapSha256 is the
// importMapSha256 the caller read from the queue.
type ImportMapRequest struct {
	QueueID, RequestID      string
	ExpectedImportMapSha256 wire.Digest
	ImportMap               []byte
}

// ImportMapAmend commits an amended import map through the §5.2 writer: one
// IMPORT_MAP_AMEND receipt posting the map and the queue that binds it. The
// model refuses a stale expected digest (REVISION_CONFLICT), a non-owner, a
// barrier and an amendment that breaks a ticket identity (DUPLICATE_ID), and
// an identical retry replays (CTS-V0-021..023).
func ImportMapAmend(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice ImportMapRequest, now wire.Timestamp) (out *Report, outErr error) {
	defer func() { out, outErr = intentFix(repo, out, outErr) }()
	request := transaction.Request{Operation: transaction.ImportMapAmend, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, ImportMap: choice.ImportMap, ExpectedImportMapSha256: choice.ExpectedImportMapSha256}
	report, _, err := administrativeWrite(ctx, repo, request, now, nil)
	return report, err
}
