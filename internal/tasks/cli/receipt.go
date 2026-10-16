package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func receiptAudit(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "audit"}
	if len(args) != 0 {
		return usage(cmd, "receipt audit accepts no arguments")
	}
	var observed *journal.Result
	rc, err := withStore(env, func(rc *readCtx) error {
		observed = nil
		reader := journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree, IntentTree: rc.tree}
		reader.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error { return store.ReceiptBindingFold(rc.repo) }
		audited, err := reader.Audit()
		if err != nil {
			return err
		}
		if audited.Identity.HeadSha256 != rc.snap.HeadSha256 || audited.Identity.IntentTreeSha256 != rc.snap.IntentTree {
			return wire.Errorf(wire.CodeSnapshotMoved, "receipt audit", "journal and outer snapshot differ")
		}
		if err := receiptMaterialBindings(rc.repo, audited); err != nil {
			return err
		}
		observed = audited
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	item := wire.NewObject()
	item.Set("headSeq", wire.String(string(observed.LastSeq)))
	item.Set("lastReceiptSha256", wire.String(string(observed.LastReceiptSha256)))
	item.Set("structuralConsistency", wire.String(observed.StructuralConsistency))
	item.Set("projectionAgreement", wire.String(observed.ProjectionAgreement))
	item.Set("semanticCoverage", wire.String(observed.SemanticCoverage))
	item.Set("historicalAcceptance", wire.String(observed.HistoricalAcceptance))
	item.Set("actorAuthentication", wire.String(observed.ActorAuthentication))
	item.Set("liveness", wire.String(observed.Liveness))
	item.Set("runtimeQualification", wire.String(observed.RuntimeQualification))
	item.Set("stagingPresent", wire.Bool(observed.StagingPresent))
	result := success(cmd, rc)
	result.Items = []wire.Value{wire.ObjectValue(item)}
	return result
}

// receiptCompact rewrites superseded liveness receipts as their elided
// taskman-receipt/1 form (CAL-V0-302). It is the only command that does;
// nothing runs it implicitly.
func receiptCompact(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "compact"}
	dryRun := false
	for _, a := range args {
		if a != "--dry-run" || dryRun {
			return usage(cmd, "receipt compact accepts only --dry-run")
		}
		dryRun = true
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	report, err := store.CompactLiveness(writerContext(), repo, dryRun)
	if err != nil {
		return failure(cmd, &readCtx{repo: repo}, err)
	}
	item := wire.NewObject()
	item.Set("dryRun", wire.Bool(report.DryRun))
	item.Set("headSeq", wire.String(string(wire.SizeOf(report.HeadSeq))))
	item.Set("elided", wire.String(string(wire.CountOf(int64(report.Elided)))))
	item.Set("alreadyElided", wire.String(string(wire.CountOf(int64(report.AlreadyElided)))))
	item.Set("receiptBytesBefore", wire.String(string(wire.SizeOf(uint64(report.BytesBefore)))))
	item.Set("receiptBytesAfter", wire.String(string(wire.SizeOf(uint64(report.BytesAfter)))))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
}
