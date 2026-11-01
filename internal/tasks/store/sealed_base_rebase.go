package store

import (
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// sealedBaseRebase returns wc, a complete audit's checkpoint at head, re-based
// on the sealed base of the checkpoint bound under token (LGL-V0-891), or nil
// when there is none or the re-base refuses: a write that takes the complete
// route then keeps FullSeq at the sealed sequence S, so the next pool verb
// still resumes from the sealed base (LGL-V0-009, V1-2621).
func sealedBaseRebase(repo *intent.Repository, head *snapshot.Head, wc *journal.WriterCheckpoint, token writerToken) *journal.WriterCheckpoint {
	if head == nil || head.Segments == nil || wc == nil || !token.ok {
		return nil
	}
	old, bound := boundWriterCheckpoint(repo)
	if old == nil || bound != token {
		return nil
	}
	return rebaseOnSealed(head, wc, old)
}

// rebaseOnSealed is sealedBaseRebase for an already bound checkpoint old.
func rebaseOnSealed(head *snapshot.Head, wc, old *journal.WriterCheckpoint) *journal.WriterCheckpoint {
	if head == nil || head.Segments == nil || wc == nil || old == nil {
		return nil
	}
	rebased, err := wc.RebaseOnSealedBase(old, head.Segments.Through.Uint64())
	if err != nil {
		return nil
	}
	return rebased
}

// mutateWriterOff holds the state directories a test set to send every Mutate
// to the complete route while the writer checkpoint stays bound, the state a
// declined writer route leaves (V1-2621). It is empty in production.
var mutateWriterOff sync.Map // state directory -> struct{}
