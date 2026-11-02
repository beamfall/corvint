package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// readLeaseProof lets a gate's pre-read and eventual write share the same
// audit when its command leaves the store unchanged. It acquires no lock.
func readLeaseProof(ctx context.Context, repo *intent.Repository) (*journal.Result, error) {
	return readLeaseProofWithAudit(ctx, repo, (journal.Reader).AuditForWriteObserved)
}

// Operation-local seam keeps fault injection independent of concurrent reads.
func readLeaseProofWithAudit(ctx context.Context, repo *intent.Repository, audit func(journal.Reader) (*journal.Result, journal.PhysicalObservation, error)) (*journal.Result, error) {
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g, err := authority.WatchChanges(repo)
		if err != nil {
			if wire.CodeOf(err) == wire.CodeSnapshotMoved || os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		proof, err, auditCleanup := func() (*journal.Result, error, error) {
			inv, err := inventory(repo)
			if err != nil {
				return nil, err, nil
			}
			head, _, _, err := journalBytes(repo)
			if err != nil {
				return nil, err, nil
			}
			return leaseAuditObserved(repo, g, inv, head, nil, audit)
		}()
		if moved := g.Check(); moved != nil && (err == nil || wire.CodeOf(err) == wire.CodeSnapshotMoved) {
			err = moved
		}
		// Native close failures are fatal even alongside SNAPSHOT_MOVED.
		// Keep them separate until the retry decision, then retain every error.
		if cleanup := g.Close(); cleanup != nil || auditCleanup != nil {
			return nil, errors.Join(err, auditCleanup, cleanup)
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		if err != nil {
			return nil, err
		}
		return proof, proof.IntentError
	}
	return nil, wire.Errorf(wire.CodeSnapshotMoved, "gate", "store changed during all read attempts")
}

// checkpointedLeaseProof is the gate pre-read's audit (TCX-V0-973): the
// read-side journal audit resumed from the retained read checkpoint
// (CAL-V0-059), else from the bound writer checkpoint (LGL-V0-807), else
// complete, selecting the queue, the policy, the program and pool state,
// every attempt record and every ticket record, as the complete pre-read
// retained them, and bound to the head it observed. It builds no inventory
// and retains nothing in the lease audit cache: the lease write that
// follows audits the store again on its own route and rechecks under the
// lock everything decided from this read. A read that fails, is pending or
// finds staging present falls back to readLeaseProof, so every refusal is
// the complete audit's, as before.
func checkpointedLeaseProof(ctx context.Context, repo *intent.Repository) (*journal.Result, error) {
	for round := 0; round < 4; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		headRaw, _, _, err := journalBytes(repo)
		if err != nil {
			return readLeaseProof(ctx, repo)
		}
		head, err := snapshot.DecodeHead(headRaw)
		if err != nil {
			return readLeaseProof(ctx, repo)
		}
		paths, err := preReadPaths(repo)
		if err != nil {
			return readLeaseProof(ctx, repo)
		}
		// The pre-read captures the tree once and hands it to the audit
		// as the CLI readers hand theirs (TM-V0-008): the audit takes
		// each file's bytes from it while the size matches and lists the
		// rest. The lease write that records the run captures and rechecks
		// the tree again on its own route (TCX-V0-974).
		tree, err := captureWriteTree(ctx, repo)
		if err != nil {
			return readLeaseProof(ctx, repo)
		}
		reader := journalReader(repo, head)
		reader.IntentTree = &tree
		reader.RetainIntentRaw = batchWrites(ctx) != nil
		reader.SelectState = true
		reader.Checkpoint = readAuditCheckpoint(repo)
		reader.WriterCheckpoint = func() *journal.WriterCheckpoint { return ReadWriterCheckpoint(repo) }
		proof, err := reader.Audit(paths...)
		if err != nil || proof == nil || proof.Pending || proof.StagingPresent || proof.Head == nil {
			return readLeaseProof(ctx, repo)
		}
		if proof.Identity.HeadSha256 != wire.Sum(headRaw) {
			// A writer committed between the head read and the audit: the
			// next round reads the new head.
			continue
		}
		if reader.RetainIntentRaw {
			mutationStage(ctx, "batch.pre-read.audit")
			// Projection bytes came from our native capture, so validate the
			// complete live tree again before the gate may use them. Keep the
			// audit's independent listings and the complete-read fallback.
			current, err := captureWriteTree(ctx, repo)
			if err != nil || current.Sha256 != proof.Identity.IntentTreeSha256 {
				return readLeaseProof(ctx, repo)
			}
			raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes)
			if err != nil || wire.Sum(raw) != proof.Identity.HeadSha256 {
				return readLeaseProof(ctx, repo)
			}
		}
		return proof, proof.IntentError
	}
	return readLeaseProof(ctx, repo)
}

// preReadPaths lists what checkpointedLeaseProof selects beyond the state
// the reader retains on its own: the queue, the policy, programs.json and
// every ticket record (a gate substitutes the attempt's ticket touch paths).
func preReadPaths(repo *intent.Repository) ([]string, error) {
	paths := []string{"intent/queue.json", "intent/policy.json", "programs.json"}
	entries, err := os.ReadDir(filepath.Join(repo.IntentRoot(), intent.Dir, intent.TicketsDir))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, "intent/"+intent.TicketsDir+"/"+e.Name())
		}
	}
	return paths, nil
}

// readAuditCheckpoint returns the retained read checkpoint, or nil when it
// is absent, unreadable or malformed; the journal reader rebinds whatever it
// returns and runs the complete audit when it does not serve (CAL-V0-059).
func readAuditCheckpoint(repo *intent.Repository) *journal.Checkpoint {
	raw, err := intent.ReadFile(journal.CheckpointPath(repo.StateDir), journal.MaxCheckpointBytes)
	if err != nil {
		return nil
	}
	cp, err := journal.DecodeCheckpoint(raw)
	if err != nil {
		return nil
	}
	return cp
}
