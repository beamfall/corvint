package cli

import (
	"sort"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// maxWitnessReceiptRows bounds the findings and removed digests one result
// lists; the totals are always complete.
const maxWitnessReceiptRows = 256

// retainedValue is the `retained` object a witness or put reports.
func retainedValue(p *evidencestore.PutResult) wire.Value {
	return wire.ObjectValue(wire.NewObject().
		Set("sha256", wire.String(string(p.Sha256))).
		Set("bytes", wire.String(string(wire.SizeOf(uint64(p.Bytes))))).
		Set("stored", wire.String(p.Stored)).
		Set("path", wire.String(p.Path)))
}

// receiptPut stores one file in the witness receipt store under its SHA-256
// (TOL-V0-134). It writes nothing under the journal.
func receiptPut(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "put"}
	if len(args) != 1 || args[0] == "" || args[0][0] == '-' {
		return usage(cmd, "receipt put takes exactly one FILE")
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	raw, err := readPutFile(args[0])
	if err != nil {
		return failure(cmd, nil, err)
	}
	put, err := evidencestore.Put(repo.StateDir, raw)
	if err != nil {
		return failure(cmd, nil, err)
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{retainedValue(put)}}
}

// readPutFile reads a regular file under the store's per-file bound.
func readPutFile(p string) ([]byte, error) {
	raw, err := readBounded(p, evidencestore.MaxBlobBytes)
	if err != nil {
		if _, ok := err.(*wire.Error); ok {
			return nil, err
		}
		return nil, wire.Errorf(wire.CodeMissingEvidence, "FILE", "the file is not a readable regular file: %v", err)
	}
	return raw, nil
}

// receiptDigest parses the one SHA256 argument of get and path.
func receiptDigest(cmd []string, args []string) (wire.Digest, *wire.Result) {
	if len(args) != 1 {
		return "", usage(cmd, cmd[0]+" "+cmd[1]+" takes exactly one SHA256")
	}
	d, err := wire.ParseDigest("SHA256", args[0])
	if err != nil {
		return "", failure(cmd, nil, err)
	}
	return d, nil
}

// receiptPath resolves a digest to the stored file's path after verifying
// its bytes (TOL-V0-136). It is a pure read: nothing is created or changed.
func receiptPath(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "path"}
	d, res := receiptDigest(cmd, args)
	if res != nil {
		return res
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return failure(cmd, nil, err)
	}
	raw, err := evidencestore.Get(repo.StateDir, d)
	if err != nil {
		return failure(cmd, nil, err)
	}
	o := wire.NewObject().Set("sha256", wire.String(string(d))).
		Set("bytes", wire.String(string(wire.SizeOf(uint64(len(raw)))))).
		Set("path", wire.String(evidencestore.Path(repo.StateDir, d))).
		Set("verified", wire.Bool(true))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// receiptGet writes the verified stored bytes of a digest to stdout and
// nothing else; a digest that cannot be resolved or whose bytes differ
// emits the usual refusal envelope instead (TOL-V0-136).
func receiptGet(env Env, args []string) int {
	cmd := []string{"receipt", "get"}
	d, res := receiptDigest(cmd, args)
	if res != nil {
		return emit(env.Stdout, res)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return emit(env.Stdout, failure(cmd, nil, err))
	}
	raw, err := evidencestore.Get(repo.StateDir, d)
	if err != nil {
		return emit(env.Stdout, failure(cmd, nil, err))
	}
	if _, err := env.Stdout.Write(raw); err != nil {
		return 1
	}
	return 0
}

// witnessReceiptScan is the TOL-V0-138 check of every ledger: each
// WITNESSED entry whose evidence is a Playwright report must resolve to a
// stored, digest-verified receipt. cited holds every report digest a
// current ledger entry names, in any state; it is what prune keeps.
type witnessReceiptScan struct {
	witnessed int64
	present   int64
	findings  []wire.Value
	total     int64
	cited     map[wire.Digest]bool
	// unfolded counts ledgers that could not be folded; their citations
	// are unknown, so prune removes nothing while any remains.
	unfolded int64
}

func scanWitnessReceipts(rc *readCtx) (*witnessReceiptScan, error) {
	s := &witnessReceiptScan{cited: map[wire.Digest]bool{}}
	read := store.ObligationEventReader(rc.repo)
	states := map[wire.Digest]string{}
	details := map[wire.Digest]string{}
	add := func(o *wire.Object) {
		s.total++
		if len(s.findings) < maxWitnessReceiptRows {
			s.findings = append(s.findings, wire.ObjectValue(o))
		}
	}
	for _, id := range rc.store.Inventory.IDs() {
		rec, ok := rc.store.Inventory.Get(id)
		if !ok || rec.ObligationsRef == nil {
			continue
		}
		l, err := ticket.FoldObligationChain(rec.TicketID, *rec.ObligationsRef, read)
		if err != nil {
			// A ledger that cannot be folded cannot show its witnesses are
			// resolvable: it is a finding, never skipped.
			s.unfolded++
			add(wire.NewObject().Set("ticketId", wire.String(id)).Set("code", wire.String("LEDGER_UNREADABLE")).
				Set("chainCode", wire.String(chainCode(err))).Set("detail", wire.String(prose(err.Error()))))
			continue
		}
		for _, e := range l.Entries {
			if e.Evidence == nil || e.Evidence.Source != ticket.ObligationSourceReport || e.Evidence.ReportSha256 == nil {
				continue
			}
			d := *e.Evidence.ReportSha256
			s.cited[d] = true
			if e.State != ticket.ObligationWitnessed {
				continue
			}
			s.witnessed++
			state, ok := states[d]
			if !ok {
				var cerr error
				state, cerr = evidencestore.Check(rc.repo.StateDir, d)
				states[d] = state
				if cerr != nil {
					details[d] = prose(cerr.Error())
				}
			}
			if state == evidencestore.StatePresent {
				s.present++
				continue
			}
			add(wire.NewObject().Set("ticketId", wire.String(id)).Set("obligationId", wire.String(e.ID)).
				Set("code", wire.String(state)).Set("reportSha256", wire.String(string(d))).
				Set("reviewable", wire.Bool(false)).Set("detail", wire.String(details[d])))
		}
	}
	return s, nil
}

// receiptAuditWitnesses adds the TOL-V0-138 section to a receipt audit
// result and refuses it MISSING_EVIDENCE when any witnessed report cannot
// be resolved, so an unreviewable witness is never reported OK.
func receiptAuditWitnesses(res *wire.Result, item *wire.Object, s *witnessReceiptScan, entries []evidencestore.Entry, listErr error) {
	if listErr != nil {
		// A store that cannot be listed keeps the per-obligation findings
		// already collected and is itself a finding.
		s.total++
		if len(s.findings) < maxWitnessReceiptRows {
			s.findings = append(s.findings, wire.ObjectValue(wire.NewObject().Set("code", wire.String("STORE_UNREADABLE")).
				Set("reviewable", wire.Bool(false)).Set("detail", wire.String(prose(listErr.Error())))))
		}
	}
	uncited := int64(0)
	for _, e := range entries {
		if !s.cited[e.Sha256] {
			uncited++
		}
	}
	item.Set("witnessReceipts", wire.ObjectValue(wire.NewObject().
		Set("witnessedReports", wire.String(string(wire.CountOf(s.witnessed)))).
		Set("resolved", wire.String(string(wire.CountOf(s.present)))).
		Set("findingsTotal", wire.String(string(wire.CountOf(s.total)))).
		Set("findingsTruncated", wire.Bool(s.total > int64(len(s.findings)))).
		Set("findings", wire.Array(s.findings...)).
		Set("storedFiles", wire.String(string(wire.CountOf(int64(len(entries)))))).
		Set("citedDigests", wire.String(string(wire.CountOf(int64(len(s.cited)))))).
		Set("uncitedStoredFiles", wire.String(string(wire.CountOf(uncited))))))
	if s.total > 0 {
		res.Outcome = wire.OutcomeRefused
		res.Codes = []string{wire.CodeMissingEvidence}
		res.Warnings = append(res.Warnings, prose(string(wire.CountOf(s.total))+" witness receipt finding(s): a witnessed obligation whose Playwright report digest the witness receipt store cannot resolve is unreviewable; store the exact report bytes with `receipt put FILE`"))
	}
}

// receiptPrune removes stored receipts no current ledger entry cites
// (TOL-V0-140). It holds the writer lock from the ledger scan through the
// last removal, so no witness can cite a digest it is removing. A file put
// within PruneGrace is kept, so a witness between its put and its ledger
// write is not raced; --dry-run lists and removes nothing. Every cited
// receipt is kept.
func receiptPrune(env Env, args []string) *wire.Result {
	cmd := []string{"receipt", "prune"}
	dryRun := false
	for _, a := range args {
		if a != "--dry-run" || dryRun {
			return usage(cmd, "receipt prune accepts only --dry-run")
		}
		dryRun = true
	}
	if !dryRun {
		repo, err := intent.Resolve(env.Cwd)
		if err != nil {
			return failure(cmd, nil, err)
		}
		lock, err := authority.AcquireLock(writerContext(), repo, authority.LockOptions{})
		if err != nil {
			return failure(cmd, nil, err)
		}
		defer lock.Close()
	}
	var scan *witnessReceiptScan
	rc, err := withStore(env, func(rc *readCtx) error {
		var err error
		scan, err = scanWitnessReceipts(rc)
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	if scan.unfolded > 0 {
		// A ledger that could not be folded leaves its citations unknown;
		// prune then removes nothing.
		return failure(cmd, rc, wire.Errorf(wire.CodeMissingEvidence, "receipt prune", "an obligation ledger could not be folded, so its citations are unknown; nothing was removed"))
	}
	entries, err := evidencestore.List(rc.repo.StateDir)
	if err != nil {
		return failure(cmd, rc, err)
	}
	cutoff := time.Now().Add(-evidencestore.PruneGrace)
	var removed []string
	var kept, recent int64
	for _, e := range entries {
		switch {
		case scan.cited[e.Sha256]:
			kept++
		case e.ModTime.After(cutoff):
			recent++
		case dryRun:
			removed = append(removed, string(e.Sha256))
		default:
			ok, err := evidencestore.Remove(rc.repo.StateDir, e.Sha256, cutoff)
			if err != nil {
				return failure(cmd, rc, err)
			}
			if !ok {
				recent++
				continue
			}
			removed = append(removed, string(e.Sha256))
		}
	}
	var temps int64
	if !dryRun {
		if temps, err = evidencestore.RemoveStaleTemps(rc.repo.StateDir, cutoff); err != nil {
			return failure(cmd, rc, err)
		}
	}
	sort.Strings(removed)
	total := len(removed)
	if len(removed) > maxWitnessReceiptRows {
		removed = removed[:maxWitnessReceiptRows]
	}
	o := wire.NewObject().Set("dryRun", wire.Bool(dryRun)).
		Set("storedFiles", wire.String(string(wire.CountOf(int64(len(entries)))))).
		Set("cited", wire.String(string(wire.CountOf(kept)))).
		Set("keptRecent", wire.String(string(wire.CountOf(recent)))).
		Set("removedTotal", wire.String(string(wire.CountOf(int64(total))))).
		Set("removedTruncated", wire.Bool(total > len(removed))).
		Set("removed", wire.Strings(removed)).
		Set("removedStaleTemps", wire.String(string(wire.CountOf(temps))))
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	if dryRun {
		res.Warnings = append(res.Warnings, "dry run: nothing was removed")
	}
	return res
}
