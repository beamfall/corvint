package store

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/importer"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ImportReport is one CTS-V0-003 import run: the export's item count, the
// records it planned to write, and one report per committed batch. A refused
// run carries the refusal as its only batch.
type ImportReport struct {
	Items, Planned int
	Batches        []*Report
}

// Import writes a foreign export's new and changed items as shadow IMPORT
// records (CTS-V0-003). The whole export is decoded and planned against one
// audit before the first write; the planned records then commit in
// IMPORT_APPLY batches under one lock and session. The audit is carried
// across the batches (CAL-V0-018): each batch checks only the head it
// expects, and apply checks each file it posts against its receipt.
func Import(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, export []byte, now wire.Timestamp) (*ImportReport, error) {
	return importWith(ctx, repo, actor, queueID, export, now, nil)
}

// importWith is Import with a hook run before each batch commits.
func importWith(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, export []byte, now wire.Timestamp, beforeBatch func([][]byte) error) (*ImportReport, error) {
	out := &ImportReport{}
	probe := transaction.Request{Operation: transaction.ImportApply, QueueID: queueID, Actor: actor}
	if actor.Role != "OWNER" && actor.Role != "OPERATOR" {
		result := transaction.Model(probe, transaction.Input{})
		out.Batches = append(out.Batches, &Report{Outcome: result.Outcome, Coverage: result.Coverage, Detail: result.Detail, Kind: result.Kind})
		return out, nil
	}
	exp, err := importer.Decode(export)
	if err != nil {
		return out, err
	}
	out.Items = len(exp.Items)
	if repo == nil {
		return out, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	if _, err = os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, wire.Errorf(wire.CodeUninitialized, "state", "no initialized store")
		}
		return out, err
	}
	if _, err = authority.Qualify(repo.CommonDir); err != nil {
		return out, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return out, err
	}
	defer lock.Close()
	now = recordedAt(ctx, now)
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return out, err
	}
	defer session.Close()
	head, err := writerGuards(repo, transaction.ImportApply)
	if err != nil {
		return importRefused(out, "", err)
	}
	if head.QueueID.Raw != queueID {
		return out, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	redone, err := redoPending(repo, session)
	if err != nil {
		return importRefused(out, "", err)
	}
	st, err := auditImport(repo, head)
	if err != nil {
		return out, err
	}
	records, err := importer.Plan(exp, st.store, actor.ID, now)
	if err != nil {
		return out, err
	}
	out.Planned = len(records)
	for _, batch := range packImport(queueID, records) {
		if beforeBatch != nil {
			if err = beforeBatch(batch); err != nil {
				return out, err
			}
		}
		report, err := importBatch(repo, session, actor, queueID, st, batch, now)
		if err != nil {
			return out, err
		}
		report.Redone = redone
		redone = false
		out.Batches = append(out.Batches, report)
		if report.Receipt == "" {
			return out, nil
		}
	}
	return out, nil
}

func importRefused(out *ImportReport, requestID string, err error) (*ImportReport, error) {
	report, err := guardFailure(&Report{}, requestID, err)
	if err == nil {
		out.Batches = append(out.Batches, report)
	}
	return out, err
}

// importAudit is one audited view of the intent store. bound records that
// the whole observation has been bound once; slots maps each ticket path to
// its index in tickets.
type importAudit struct {
	inv      *transaction.Inventory
	proof    *journal.Result
	tickets  [][]byte
	releases [][]byte
	store    importer.Store
	bound    bool
	slots    map[string]int
}

func auditImport(repo *intent.Repository, head *snapshot.Head) (*importAudit, error) {
	inv, err := inventory(repo)
	if err != nil {
		return nil, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	proof, err := lockedJournalReader(repo, head).Audit(paths...)
	if err != nil {
		return nil, err
	}
	if proof.StagingPresent {
		return nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	a := &importAudit{inv: inv, proof: proof, slots: map[string]int{}}
	recs := []*ticket.Record{}
	for _, path := range paths[2:] {
		raw := proof.Records[path].Raw
		if strings.HasPrefix(path, "intent/releases/") {
			a.releases = append(a.releases, raw)
			continue
		}
		a.slots[path] = len(a.tickets)
		a.tickets = append(a.tickets, raw)
		rec, err := ticket.Decode(raw)
		if err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	if a.store.Queue, err = intent.DecodeQueue(proof.Records["intent/queue.json"].Raw); err != nil {
		return nil, err
	}
	if a.store.Policy, err = intent.DecodePolicy(proof.Records["intent/policy.json"].Raw); err != nil {
		return nil, err
	}
	a.store.Tickets, err = ticket.NewInventory(a.store.Queue.QueueID, recs)
	return a, err
}

// importBatch commits one IMPORT_APPLY batch through the §5.2 writer against
// the carried audit a, and advances a past the batch.
func importBatch(repo *intent.Repository, session *authority.Session, actor mutation.Binding, queueID string, a *importAudit, records [][]byte, now wire.Timestamp) (*Report, error) {
	report := &Report{}
	request := transaction.Request{Operation: transaction.ImportApply, QueueID: queueID, RequestID: importRequestID(records), Actor: actor, Records: records}
	head, err := writerGuards(repo, request.Operation)
	if err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	entry, found, err := a.lookup(repo, head, request.RequestID)
	if err != nil {
		return report, err
	}
	if found {
		result := replayResult(request, entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	if wire.Sum(headRaw) != a.proof.Identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	headRc, err := headReceipt(repo, headRaw)
	if err != nil {
		return report, err
	}
	result := transaction.Model(request, transaction.Input{Inventory: a.inv, Head: headRaw, HeadReceipt: headRc, Queue: a.proof.Records["intent/queue.json"].Raw, Policy: a.proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, CanonicalTickets: a.tickets, CanonicalReleases: a.releases, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	if err = requireBranch(repo, a.store.Queue.IntentBranch); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	if err = a.bind(repo, request.Operation); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	if report.Receipt, err = apply(repo, session, result.Plan); err != nil {
		return report, err
	}
	return report, a.advance(result.Plan, result.Final, records)
}

// lookup walks the journal for a request only when the carried inventory
// holds its request file; a request absent from it was never recorded.
func (a *importAudit) lookup(repo *intent.Repository, head *snapshot.Head, id string) (mutation.IndexEntry, bool, error) {
	path, err := snapshot.RequestPath(id)
	if err != nil {
		return mutation.IndexEntry{}, false, err
	}
	if !a.inv.Has(path) {
		return mutation.IndexEntry{}, false, nil
	}
	index := journal.RequestIndex{Reader: lockedJournalReader(repo, head)}
	return index.Lookup(id)
}

// bind checks the whole audited observation once, before the first write.
// Later batches rely on the held lock and session: their head is checked
// against the one the previous batch wrote, and apply checks each post.
func (a *importAudit) bind(repo *intent.Repository, operation string) error {
	if a.bound {
		return nil
	}
	if err := bindObservation(repo, a.proof.Identity, operation); err != nil {
		return err
	}
	a.bound = true
	return nil
}

// advance moves the audit past a committed batch: the plan's final
// inventory, its head, and the ticket records it posted.
func (a *importAudit) advance(plan *transaction.Plan, final *transaction.Inventory, records [][]byte) error {
	a.inv = final
	a.proof.Identity.HeadSha256 = wire.Sum(plan.Head())
	for _, raw := range records {
		rec, err := ticket.Decode(raw)
		if err != nil {
			return err
		}
		path := "intent/tickets/" + rec.TicketID.Local + ".json"
		if i, ok := a.slots[path]; ok {
			a.tickets[i] = raw
			continue
		}
		a.slots[path] = len(a.tickets)
		a.tickets = append(a.tickets, raw)
	}
	return nil
}

// importRequestID names a batch by its records: "import-" and the first 128
// bits of the SHA-256 over their digests, so a retried batch replays.
func importRequestID(records [][]byte) string {
	joined := make([]byte, 0, len(records)*71)
	for _, raw := range records {
		joined = append(joined, wire.Sum(raw)...)
	}
	sum := wire.Sum(joined)
	digest, _ := hex.DecodeString(strings.TrimPrefix(string(sum), "sha256:"))
	return "import-" + hex.EncodeToString(digest[:16])
}

// packImport splits records, in order, into batches that fit one
// IMPORT_APPLY stage: at most 11 slots (receipt, head, request post, each
// ticket post and each blob of a ticket over the inline bound), at most 8
// inline receipt post entries (the request post and each inline ticket), and a
// stage descriptor within MaxStageDescriptorBytes, measured on a synthetic
// descriptor whose journal entries take their widest values.
func packImport(queueID string, records [][]byte) [][][]byte {
	slots, _ := snapshot.StageLimits(snapshot.StageImportApply)
	batches := [][][]byte{}
	batch := [][]byte{}
	for _, raw := range records {
		next := append(append([][]byte(nil), batch...), raw)
		if len(batch) > 0 && !importFits(queueID, next, slots) {
			batches = append(batches, batch)
			next = [][]byte{raw}
		}
		batch = next
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

func importFits(queueID string, batch [][]byte, slots int) bool {
	widest := wire.Digest("sha256:" + strings.Repeat("f", 64))
	max := wire.SizeOf(uint64(wire.MaxReceiptFileBytes))
	desc := snapshot.StageDescriptor{QueueID: queueID, Operation: snapshot.StageImportApply, RequestID: importRequestID(batch), RequestSha256: widest, RecordedAt: wire.Timestamp("0000-00-00T00:00:00.000000000Z"), Base: &snapshot.StageBase{LastSeq: wire.SizeOf(999999999999), LastReceiptSha256: widest}}
	arts := []snapshot.StageDescription{{Role: "HEAD", Target: "head.json", Sha256: widest, Bytes: max}, {Role: "RECEIPT", Target: "receipts/999999999999.json", Sha256: widest, Bytes: max}, {Role: "POST", Target: "requests/ff/" + strings.Repeat("f", 64) + ".json", Sha256: widest, Bytes: max}}
	inline := 1
	for _, raw := range batch {
		rec, _ := ticket.Decode(raw)
		arts = append(arts, snapshot.StageDescription{Role: "POST", Target: "intent/tickets/" + rec.TicketID.Local + ".json", Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
		if len(raw) <= wire.MaxInlinePostEntryBytes {
			inline++
			continue
		}
		arts = append(arts, snapshot.StageDescription{Role: "EVIDENCE", Target: "evidence/" + string(wire.Sum(raw)), Sha256: wire.Sum(raw), Bytes: wire.SizeOf(uint64(len(raw)))})
	}
	for i := range arts {
		arts[i].Slot = slotName(i)
	}
	desc.Artifacts = arts
	return len(arts) <= slots && inline <= wire.MaxInlinePostEntries && len(wire.EncodeFile(desc.Value())) <= snapshot.MaxStageDescriptorBytes
}
