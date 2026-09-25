package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/journal"
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
		reader := journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}
		audited, err := reader.Audit()
		if err != nil {
			return err
		}
		if audited.Identity.HeadSha256 != rc.snap.HeadSha256 || audited.Identity.IntentTreeSha256 != rc.snap.IntentTree {
			return wire.Errorf(wire.CodeSnapshotMoved, "receipt audit", "journal and outer snapshot differ")
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
