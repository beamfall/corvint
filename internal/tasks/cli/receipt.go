package cli

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func receiptAudit(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "audit"}
	witnesses := false
	var through uint64
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--witness-receipts" && !witnesses:
			witnesses = true
		case a == "--through" && through == 0:
			// TCL-V0-820: a decimal receipt sequence of at least 1.
			var n uint64
			var err error
			if i+1 < len(args) {
				n, err = strconv.ParseUint(args[i+1], 10, 64)
			}
			if i+1 >= len(args) || err != nil || n == 0 || strconv.FormatUint(n, 10) != args[i+1] {
				return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "--through", "receipt audit --through needs a decimal receipt sequence of at least 1"))
			}
			through, i = n, i+1
		default:
			return usage(cmd, "receipt audit accepts only --witness-receipts or --through SEQ")
		}
	}
	if witnesses && through != 0 {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "--through", "receipt audit --through cannot be combined with --witness-receipts"))
	}
	var observed *journal.Result
	var scan *witnessReceiptScan
	var stored []evidencestore.Entry
	var listErr error
	var verified, manual int
	body := func(rc *readCtx) error {
		observed = nil
		verified, manual = 0, 0
		reader := journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree, IntentTree: rc.tree}
		reader.ReceiptFold = func() func(*snapshot.Receipt, wire.Digest) error { return store.ReceiptBindingFold(rc.repo) }
		var audited *journal.Result
		var err error
		if through != 0 {
			audited, err = reader.AuditThrough(through)
		} else {
			audited, err = reader.Audit()
		}
		if err != nil {
			return err
		}
		// A bounded read hashes no intent tree of its own (TCL-V0-820).
		if audited.Identity.HeadSha256 != rc.snap.HeadSha256 || through == 0 && audited.Identity.IntentTreeSha256 != rc.snap.IntentTree {
			return wire.Errorf(wire.CodeSnapshotMoved, "receipt audit", "journal and outer snapshot differ")
		}
		if err := receiptMaterialBindings(rc.repo, audited); err != nil {
			return err
		}
		observed = audited
		// TCD-V0-190: gated (VERIFIED) and manual completions are counted
		// apart, so a manual batch close is never read as a gated one.
		for _, id := range rc.store.Inventory.IDs() {
			if rec, ok := rc.store.Inventory.Get(id); ok && rec.Status == ticket.StatusCompleted && rec.Completion != nil {
				switch rec.Completion.Kind {
				case "VERIFIED":
					verified++
				case "MANUAL":
					manual++
				}
			}
		}
		if witnesses {
			// TOL-V0-138: every witnessed report digest must resolve.
			if scan, err = scanWitnessReceipts(rc); err != nil {
				return err
			}
			// A store that cannot be listed is reported as a finding,
			// never in place of the findings above.
			stored, listErr = evidencestore.List(rc.repo.StateDir)
		}
		return nil
	}
	var rc *readCtx
	var err error
	if through != 0 {
		if rc, err = boundedAuditCtx(env); err == nil {
			err = body(rc)
		}
	} else {
		rc, err = withStore(env, body)
	}
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
	if through != 0 {
		item.Set("throughSeq", wire.String(string(wire.SizeOf(through))))
	}
	// The audit root (JRT-V0-005): the seal's anchor S on a sealed store,
	// whose receipts 1..S-1 the audit did not walk; 0 and 0 otherwise.
	anchor, retired := observed.RetiredThrough(), uint64(0)
	if anchor != 0 {
		retired = anchor - 1
	}
	item.Set("anchorSeq", wire.String(string(wire.SizeOf(anchor))))
	item.Set("retiredThrough", wire.String(string(wire.SizeOf(retired))))
	item.Set("completions", wire.ObjectValue(wire.NewObject().Set("verified", count(verified)).Set("manual", count(manual))))
	result := success(cmd, rc)
	result.Items = []wire.Value{wire.ObjectValue(item)}
	if len(observed.OrphanStageSlots) > 0 {
		// CTS-V0-026: a killed writer's orphans, set aside under a free
		// writer lock; the next writer removes them.
		result.Warnings = append(result.Warnings, prose("staging holds a killed writer's orphan slot(s) the audit set aside; the next writer removes them: "+strings.Join(observed.OrphanStageSlots, ", ")))
	}
	if witnesses {
		receiptAuditWitnesses(result, item, scan, stored, listErr)
	}
	return result
}

// boundedAuditCtx is the read context of `receipt audit --through SEQ`
// (TCL-V0-820): head.json is decoded, but the head receipt and every other
// receipt after SEQ need not verify, so the outer snapshot check that
// withStore applies is not taken; the bounded walk checks receipts 1..SEQ.
func boundedAuditCtx(env Env) (*readCtx, error) {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, err
	}
	rc := &readCtx{repo: repo}
	s, err := snapshot.Probe(repo.StateDir)
	if s == nil || s.Head == nil {
		return rc, err
	}
	rc.snap = s
	if s.Head.PrimaryWorktree != repo.PrimaryWorktree {
		return rc, wire.Errorf(wire.CodeUnsupportedFilesystem, repo.StateDir, "head.primaryWorktree %q differs from the resolved primary worktree %q (TRL-V0-001)", s.Head.PrimaryWorktree, repo.PrimaryWorktree)
	}
	if rc.store, err = intent.Load(repo.IntentRoot()); err != nil {
		return rc, err
	}
	if rc.store.Queue.QueueID.Raw != s.Head.QueueID.Raw {
		return rc, wire.Errorf(wire.CodeMalformed, repo.StateDir+"/head.json/queueId", "head queue %s differs from queue.json %s", s.Head.QueueID.Raw, rc.store.Queue.QueueID.Raw)
	}
	return rc, nil
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

// receiptRetire runs `receipt retire --archive FILE [--dry-run]` or
// `receipt unretire --archive FILE` (JRT-V0-002, JRT-V0-011).
func receiptRetire(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"receipt", verb}
	archivePath, dryRun := "", false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dry-run" && verb == "retire" && !dryRun:
			dryRun = true
		case args[i] == "--archive" && archivePath == "" && i+1 < len(args) && args[i+1] != "":
			archivePath = args[i+1]
			i++
		default:
			return usage(cmd, "receipt retire accepts --archive FILE and --dry-run; receipt unretire accepts --archive FILE")
		}
	}
	if archivePath == "" && !dryRun {
		return usage(cmd, "receipt "+verb+" needs --archive FILE")
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	if archivePath != "" && !filepath.IsAbs(archivePath) {
		archivePath = filepath.Join(env.Cwd, archivePath)
	}
	var report *store.RetireReport
	if verb == "retire" {
		report, err = store.RetirePrefix(writerContext(), repo, archivePath, dryRun)
	} else {
		report, err = store.UnretirePrefix(writerContext(), repo, archivePath)
	}
	if err != nil {
		return failure(cmd, &readCtx{repo: repo}, err)
	}
	item := wire.NewObject()
	item.Set("dryRun", wire.Bool(report.DryRun))
	if report.Recovered != "" {
		item.Set("recovered", wire.String(report.Recovered))
	}
	item.Set("anchorSeq", wire.String(string(wire.SizeOf(report.AnchorSeq))))
	item.Set("retired", wire.String(string(wire.CountOf(int64(report.Retired)))))
	item.Set("receiptBytesBefore", wire.String(string(wire.SizeOf(report.ReceiptBytesBefore))))
	item.Set("receiptBytesAfter", wire.String(string(wire.SizeOf(report.ReceiptBytesAfter))))
	item.Set("sealBytes", wire.String(string(wire.SizeOf(report.SealBytes))))
	if report.ArchiveSha256 != "" {
		item.Set("archiveSha256", wire.String(string(report.ArchiveSha256)))
		item.Set("archiveBytes", wire.String(string(wire.SizeOf(report.ArchiveBytes))))
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
}

// journalSeal runs `journal seal` or `journal unseal` (JSG-V0-013).
func journalSeal(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"journal", verb}
	if len(args) != 0 {
		return usage(cmd, "journal "+verb+" accepts no arguments")
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	migrate := store.SealJournal
	if verb == "unseal" {
		migrate = store.UnsealJournal
	}
	report, err := migrate(writerContext(), repo)
	if err != nil {
		return failure(cmd, &readCtx{repo: repo}, err)
	}
	item := wire.NewObject()
	item.Set("headSeq", wire.String(string(wire.SizeOf(report.HeadSeq))))
	item.Set("sealedThroughBefore", wire.String(string(wire.SizeOf(report.ThroughBefore))))
	item.Set("sealedThroughAfter", wire.String(string(wire.SizeOf(report.ThroughAfter))))
	item.Set("segments", wire.String(string(wire.CountOf(int64(report.SegmentsAfter)))))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}}
}
