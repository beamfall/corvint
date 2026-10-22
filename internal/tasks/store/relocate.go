package store

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// RelocateRequest names one explicit relocation (TRL-V0-001): the store is
// re-pinned from From, the primary worktree head.json records, to the
// resolved primary worktree the command runs from.
type RelocateRequest struct {
	QueueID, RequestID, From string
}

// Relocate commits one RELOCATE receipt (TRL-V0-001..004). It reads and
// guards the journal under the recorded primary worktree From names, posts a
// pinned taskman-relocate/0 descriptor naming both paths, and re-pins
// head.primaryWorktree to the resolved primary. Relocation is never inferred:
// a head naming neither From nor the resolved primary refuses, and the model
// refuses while any attempt holds a reservation.
func Relocate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, req RelocateRequest, now wire.Timestamp) (*Report, error) {
	if repo == nil {
		return &Report{}, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	request := transaction.Request{Operation: transaction.Relocate, QueueID: req.QueueID, RequestID: req.RequestID, Actor: actor, FromPrimaryWorktree: req.From, PrimaryWorktree: repo.PrimaryWorktree}
	report, _, err := administrativeWrite(ctx, repo, request, now, nil)
	return report, RelocationRefusal(err)
}

// RelocationRefusal names the quiescence requirement on a relocation refused
// because the store's journal does not audit (TRL-V0-006): a copy taken while
// a writer was publishing is not a consistent snapshot. Every other error,
// and nil, is returned unchanged with its code.
func RelocationRefusal(err error) error {
	if wire.CodeOf(err) != wire.CodeJournalForked {
		return err
	}
	return wire.Errorf(wire.CodeJournalForked, "state", "%v; a relocated or copied store must be a consistent snapshot: copy it only while no corvint-tasks writer, dispatcher or live attempt holds the store (TRL-V0-006)", err)
}

// relocationIdentity is the repository identity a RELOCATE reads the store
// under: the resolved repository when head.json already records it (a retry
// replays, anything else the model refuses), or a copy naming the explicit
// From as the primary worktree while every file is still read from the
// resolved intent root. Any other recorded primary refuses: relocation is
// never inferred (TRL-V0-001).
func relocationIdentity(repo *intent.Repository, from string) (*intent.Repository, error) {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return nil, err
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		return nil, err
	}
	switch head.PrimaryWorktree {
	case repo.PrimaryWorktree:
		return repo, nil
	case from:
		id := *repo
		id.IntentWorktree = repo.IntentRoot()
		id.PrimaryWorktree = from
		return &id, nil
	}
	return nil, wire.Errorf(wire.CodeUnsupported, "head.json", "head.primaryWorktree %q is neither --from %q nor the resolved primary worktree %q", head.PrimaryWorktree, from, repo.PrimaryWorktree)
}

// relocatedPrimary is the destination a pending RELOCATE receipt's pinned
// descriptor names, checked to continue the head it advances.
func relocatedPrimary(repo *intent.Repository, head *snapshot.Head, receipt *snapshot.Receipt) (string, error) {
	var found *snapshot.Relocate
	for _, entry := range receipt.Post {
		if !strings.HasPrefix(entry.Path, "pinned/") {
			continue
		}
		raw, err := postBytes(repo, entry)
		if err != nil {
			return "", err
		}
		d, err := snapshot.DecodeRelocate(raw)
		if err != nil || found != nil {
			return "", wire.Errorf(wire.CodeJournalForked, entry.Path, "a RELOCATE receipt posts exactly one relocation descriptor")
		}
		found = d
	}
	if found == nil || found.FromPrimaryWorktree != head.PrimaryWorktree || found.QueueID != head.QueueID {
		return "", wire.Errorf(wire.CodeJournalForked, "head.json", "the RELOCATE descriptor does not continue the recorded primary worktree")
	}
	return found.ToPrimaryWorktree, nil
}
