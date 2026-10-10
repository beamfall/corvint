package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// primaryBranch reads the branch of the intent root's HEAD: the primary's
// HEAD unless CTW-V0-002 selected a linked intent worktree. The messages keep
// their TM-V0-007 wording so the intent-branch primary is unchanged.
func primaryBranch(repo *intent.Repository) (string, error) {
	raw, err := intent.ReadFile(repo.IntentHEAD(), 4096)
	if err != nil {
		return "", wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "primary branch could not be observed: %v", err)
	}
	const prefix = "ref: refs/heads/"
	text := strings.TrimSuffix(string(raw), "\n")
	if !strings.HasPrefix(text, prefix) {
		return "", wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "primary HEAD is not a local symbolic branch")
	}
	branch := strings.TrimPrefix(text, prefix)
	if _, err := wire.ParseLabel("HEAD", branch); err != nil {
		return "", wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "invalid primary branch")
	}
	return branch, nil
}

func requireBranch(repo *intent.Repository, expected string) error {
	branch, err := primaryBranch(repo)
	if err != nil {
		return err
	}
	if branch != expected {
		return wire.Errorf(wire.CodeIntentBranchMismatch, "HEAD", "primary intent branch differs")
	}
	return nil
}

// leaseCancel is the guard operation of a lease release or reap, which an
// ALL barrier lets through as it does cancel (TCP-00 §3.4).
const leaseCancel = "LEASE_CANCEL"

func guardOperation(r transaction.Request) string {
	if transaction.Cancels(r) {
		return leaseCancel
	}
	return r.Operation
}

// These guards precede recovery as well as new transactions. A marker of any
// type prevents writes, including a dangling symlink or unreadable marker.
func writerGuards(repo *intent.Repository, operation string) (*snapshot.Head, error) {
	marker := filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE")
	if _, err := os.Lstat(marker); !errors.Is(err, fs.ErrNotExist) {
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, marker, "restore marker present or unobservable")
	}
	version, err := intent.ReadFile(filepath.Join(repo.StateDir, "VERSION"), 4096)
	if err != nil {
		return nil, err
	}
	if string(version) != snapshot.VersionBytes {
		return nil, wire.Errorf(wire.CodeUnsupported, "VERSION", "unsupported store version")
	}
	head, err := readHead(repo)
	if err != nil {
		return nil, err
	}
	raw, err := optional(filepath.Join(repo.StateDir, "barrier.json"), 4096)
	if err != nil {
		return nil, err
	}
	if raw != nil {
		barrier, err := snapshot.DecodeBarrier(raw)
		if err != nil {
			return nil, err
		}
		if barrier.QueueID != head.QueueID {
			return nil, wire.Errorf(wire.CodeJournalForked, "barrier.json", "barrier queue differs")
		}
		if barrier.Scope == "ALL" && operation != transaction.KeepJournal && operation != transaction.AdoptFile && operation != transaction.Unpause && operation != leaseCancel {
			return nil, wire.Errorf(wire.CodePaused, "barrier.json", "ALL barrier forbids mutation")
		}
	}
	return head, nil
}

func journalReader(repo *intent.Repository, head *snapshot.Head) journal.Reader {
	return journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.IntentRoot()}, QueueID: head.QueueID, PrimaryWorktree: repo.PrimaryWorktree}
}

// lockedJournalReader is journalReader for a caller that holds the writer
// lock: no other writer can be publishing, so a descriptor-less staging slot
// is refused at once instead of waited for while every other writer is
// blocked behind this one (CTS-V0-008).
func lockedJournalReader(repo *intent.Repository, head *snapshot.Head) journal.Reader {
	r := journalReader(repo, head)
	r.WriterLocked = true
	return r
}

// retainCheckpoint records what a writer's complete settled audit just
// established so reads can resume from it (CAL-V0-060). The file is derived
// state outside the state directory: it is never an input to a mutation, and
// a failed or lost write only costs the next read one complete audit. The
// caller holds the writer lock, so one fixed temporary name cannot collide
// and a temporary left by a crash is replaced by the next writer.
func retainCheckpoint(repo *intent.Repository, proof *journal.Result) {
	cp := proof.Checkpoint()
	if cp == nil {
		return
	}
	path := journal.CheckpointPath(repo.StateDir)
	tmp := path + ".tmp"
	os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return
	}
	_, err = f.Write(cp.Encode())
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
}

// Recheck observations immediately before effects. This is cooperative-editor
// protection; it does not qualify atomic CAS against hostile concurrent editors.
func bindObservation(repo *intent.Repository, identity journal.Identity, operation string) error {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return err
	}
	tree, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		return err
	}
	if wire.Sum(raw) != identity.HeadSha256 || tree.Sha256 != identity.IntentTreeSha256 {
		return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head or intent changed")
	}
	_, err = writerGuards(repo, operation)
	return err
}

func guardFailure(report *Report, requestID string, err error) (*Report, error) {
	code := wire.CodeOf(err)
	if code != wire.CodePaused && code != wire.CodeIntentBranchMismatch {
		return report, err
	}
	report.Kind = "Refused"
	report.Detail = err.Error()
	report.Outcome = mutation.Outcome{RequestID: requestID, Outcome: mutation.OutcomeBlocked, Codes: []string{code}}
	report.Coverage = transaction.Coverage{ActorAuthentication: transaction.NotObserved, AdministrativeAuthorization: transaction.NotObserved, InventoryObservation: transaction.NotObserved, Durability: transaction.NotObserved, RuntimeQualification: transaction.NotObserved}
	return report, nil
}
