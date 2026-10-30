package store

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
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

// errSealUnsettled is what an unlocked guard read reports for a seal marker
// it must not settle (V1-3007).
var errSealUnsettled = errors.New("a seal marker is present and only a writer holding the lock may settle it")

// These guards precede recovery as well as new transactions. A marker of any
// type prevents writes, including a dangling symlink or unreadable marker.
// The caller holds the writer lock: the guards settle an interrupted seal
// (JSG-V0-008), and only the lock proves no seal is still in flight
// (V1-3007). A read without the lock uses unlockedWriterGuards.
func writerGuards(repo *intent.Repository, operation string) (*snapshot.Head, error) {
	return guardsWith(repo, operation, true)
}

// unlockedWriterGuards is writerGuards for a read that holds no writer lock,
// leaseWrite's pre-round guard read (V1-3007). A seal marker it observes may
// belong to a seal another writer has in flight, so it removes nothing: it
// takes the writer lock and runs the guards, which settle the marker, under
// it. A clean store takes no lock, as before.
func unlockedWriterGuards(ctx context.Context, repo *intent.Repository, operation string) (_ *snapshot.Head, err error) {
	head, err := guardsWith(repo, operation, false)
	if !errors.Is(err, errSealUnsettled) {
		return head, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{CallerWait: leaseLockWaitOf(ctx)})
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanup := lock.Close(); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	return writerGuards(repo, operation)
}

func guardsWith(repo *intent.Repository, operation string, locked bool) (*snapshot.Head, error) {
	marker := filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE")
	if _, err := os.Lstat(marker); !errors.Is(err, fs.ErrNotExist) {
		return nil, wire.Errorf(wire.CodeRestoreIncomplete, marker, "restore marker present or unobservable")
	}
	// An interrupted retire (JRT-V0-009) blocks writes until `receipt
	// retire` or `receipt unretire` finishes it under the lock.
	if marker := filepath.Join(repo.StateDir, journal.RetireIncompleteFile); operation != "RETIRE" && operation != "UNRETIRE" {
		if _, err := os.Lstat(marker); !errors.Is(err, fs.ErrNotExist) {
			return nil, wire.Errorf(wire.CodeRestoreIncomplete, marker, "interrupted retire marker present or unobservable; run `receipt retire` or `receipt unretire` to finish it")
		}
	}
	// A seal a stop interrupted is finished or undone before anything else
	// reads the journal (JSG-V0-008), and only under the writer lock.
	if !locked {
		if on, err := present(segmentPath(repo, snapshot.SealMarkerPath)); err != nil || on {
			if err == nil {
				err = errSealUnsettled
			}
			return nil, err
		}
	} else if err := settleSegmentSeal(repo); err != nil {
		return nil, err
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
	raw := cp.Encode()
	_, err = f.Write(raw)
	noteDerived(path, len(raw))
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
	now, err := observedSnapshot(repo)
	if err != nil {
		return err
	}
	if !sameSnapshot(now, identity) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head or intent changed")
	}
	_, err = writerGuards(repo, operation)
	return err
}

// observedSnapshot is the part of a journal identity bindObservation checks:
// the head.json digest and the intent tree digest.
func observedSnapshot(repo *intent.Repository) (journal.Identity, error) {
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return journal.Identity{}, err
	}
	tree, err := intent.TreeDigest(repo.IntentRoot())
	if err != nil {
		return journal.Identity{}, err
	}
	return journal.Identity{HeadSha256: wire.Sum(raw), IntentTreeSha256: tree.Sha256}, nil
}

// sameSnapshot compares the members observedSnapshot reads.
func sameSnapshot(a, b journal.Identity) bool {
	return a.HeadSha256 == b.HeadSha256 && a.IntentTreeSha256 == b.IntentTreeSha256
}

// bindLiveness is bindObservation for a HEARTBEAT or RENEW (JSG-V0-001):
// head.json, the queue and policy files the model decoded, reservations.json
// and every private file a plan replaces must be the bytes the audit
// observed; a refusal plans nothing and binds the rest. No ticket, release or intent tree digest is read or bound.
func bindLiveness(repo *intent.Repository, w *writerObservation, plan *transaction.Plan, operation string) error {
	moved := func(p string) error {
		return wire.Errorf(wire.CodeSnapshotMoved, p, "validated liveness input changed")
	}
	head, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return err
	}
	if wire.Sum(head) != w.proof.Identity.HeadSha256 {
		return moved("head.json")
	}
	for _, name := range []string{intent.QueueFile, intent.PolicyFile} {
		bound, _ := intent.BoundFor(name)
		raw, err := intent.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, name), bound)
		if err != nil {
			return err
		}
		record, ok := w.proof.Records["intent/"+name]
		if !ok || record.Sha256 == nil || wire.Sum(raw) != *record.Sha256 {
			return moved("intent/" + name)
		}
	}
	reservations, err := optional(filepath.Join(repo.StateDir, "reservations.json"), wire.MaxReservationSetBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(reservations, w.reservations) || (reservations == nil) != (w.reservations == nil) {
		return moved("reservations.json")
	}
	// The model read pools.json and programs.json whole and the attempts
	// its plan rewrites; each is bound to the bytes the model read.
	private := []string{"pools.json", "programs.json"}
	if plan != nil {
		for _, artifact := range plan.Artifacts() {
			if strings.HasPrefix(artifact.Target, "attempts/") {
				private = append(private, artifact.Target)
			}
		}
	}
	for _, p := range private {
		bound, err := snapshot.PostBound(p)
		if err != nil {
			return err
		}
		raw, err := optional(filepath.Join(repo.StateDir, filepath.FromSlash(p)), bound)
		if err != nil {
			return err
		}
		record, observed := w.proof.Records[p]
		if !observed || record.Sha256 == nil {
			if raw != nil {
				return moved(p)
			}
			continue
		}
		if raw == nil || wire.Sum(raw) != *record.Sha256 {
			return moved(p)
		}
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
