package journal

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0917: a tail receipt that posts a request already posted before the
// read checkpoint is refused exactly as the complete audit refuses it.
func TestPSRV0190_CheckpointTailRefusesPrefixRequestDuplicate(t *testing.T) {
	repo, r := setup(t)
	id := "r1"
	path, _ := snapshot.RequestPath(id)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 2, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	cp := checkpointed(t, repo, r)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 3, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	_, full := r.Audit()
	requireCode(t, full, wire.CodeJournalForked)
	r.Checkpoint = cp
	res, err := r.Audit()
	requireCode(t, err, wire.CodeJournalForked)
	if res != nil && res.ProjectionAgreement == "AGREES" {
		t.Fatal("checkpoint read agreed with a duplicated request")
	}
}
