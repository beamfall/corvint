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

func primaryBranch(repo *intent.Repository) (string, error) {
	raw, err := intent.ReadFile(filepath.Join(repo.CommonDir, "HEAD"), 4096)
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
		if barrier.Scope == "ALL" && operation != transaction.KeepJournal && operation != transaction.AdoptFile && operation != transaction.Unpause {
			return nil, wire.Errorf(wire.CodePaused, "barrier.json", "ALL barrier forbids mutation")
		}
	}
	return head, nil
}

func journalReader(repo *intent.Repository, head *snapshot.Head) journal.Reader {
	return journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: head.QueueID, PrimaryWorktree: repo.PrimaryWorktree}
}

// Recheck observations immediately before effects. This is cooperative-editor
// protection; it does not qualify atomic CAS against hostile concurrent editors.
func bindObservation(repo *intent.Repository, identity journal.Identity, operation string) error {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return err
	}
	tree, err := intent.TreeDigest(repo.PrimaryWorktree)
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
