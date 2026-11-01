package store

import (
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
)

// sealedBaseRebase returns the complete audit proof's checkpoint re-based on
// the sealed base of the checkpoint bound under token (LGL-V0-891), or nil
// when there is none or the journal refuses it: a write that takes the
// complete route then keeps FullSeq at the sealed sequence S, so the next
// pool verb still resumes from the sealed base (LGL-V0-009, V1-2621). The
// journal checks the base's entries and aggregates against proof first.
func sealedBaseRebase(repo *intent.Repository, proof *journal.Result, physical map[string]journal.PhysicalFile, token writerToken) *journal.WriterCheckpoint {
	if proof == nil || proof.Head == nil || proof.Head.Segments == nil || !token.ok {
		return nil
	}
	old, bound := boundWriterCheckpoint(repo)
	if old == nil || bound != token {
		return nil
	}
	rebased, err := proof.SealedBaseCheckpoint(physical, old, proof.Head.Segments.Through.Uint64())
	if err != nil {
		return nil
	}
	return rebased
}

// mutateWriterOff holds the state directories a test set to send every Mutate
// to the complete route while the writer checkpoint stays bound, the state a
// declined writer route leaves (V1-2621). It is empty in production.
var mutateWriterOff sync.Map // state directory -> struct{}
