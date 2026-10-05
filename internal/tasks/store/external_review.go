package store

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reviewAudit re-audits a REVIEW_* request with every attempt record, so the
// model judges leases and subject currency from journal-authoritative bytes
// (ERG-V0-009). Other operations keep their read boundary unchanged.
func reviewAudit(reader journal.Reader, inv *transaction.Inventory, env *mutation.Envelope, paths []string, canonical *journal.Result) ([]string, *journal.Result, error) {
	if !mutation.IsReviewOperation(env.Operation) {
		return paths, canonical, nil
	}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "attempts/") {
			paths = append(paths, file.Path)
		}
	}
	canonical, err := reader.Audit(paths...)
	if err != nil {
		return paths, nil, err
	}
	if canonical.StagingPresent {
		return paths, nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	return paths, canonical, nil
}

// reviewInputs reads, for a REVIEW_* request, the target gate's head event at
// evidence/<head> and the request's subject receipt plus its successor when
// it is not the head receipt. The model re-hashes every byte it is given;
// absent bytes make it refuse rather than infer.
func reviewInputs(repo *intent.Repository, env *mutation.Envelope, records map[string]journal.Record, head *snapshot.Head) (prior []byte, subject [][]byte, err error) {
	if !mutation.IsReviewOperation(env.Operation) || env.TargetID == nil || head == nil {
		return nil, nil, nil
	}
	p, ok := env.Payload.(*mutation.ReviewPayload)
	if !ok {
		return nil, nil, nil
	}
	q, err := snapshot.DecodeExternalReviewRequest(wire.EncodeFile(p.Request))
	if err != nil {
		return nil, nil, nil
	}
	if record, ok := records["intent/tickets/"+env.TargetID.Local+".json"]; ok {
		rec, err := ticket.Decode(record.Raw)
		if err != nil {
			return nil, nil, err
		}
		if ref, ok := rec.ExternalReviews[q.GateID]; ok {
			prior, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(ref.Head)), snapshot.MaxExternalReviewBytes)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, nil, err
			}
		}
	}
	seq := q.Subject.ReceiptSeq.Uint64()
	if seq == 0 || seq > head.LastSeq.Uint64() {
		return prior, nil, nil
	}
	for at := seq; at <= seq+1 && at <= head.LastSeq.Uint64(); at++ {
		raw, err := readReceiptBytes(repo, at)
		if err != nil {
			return nil, nil, err
		}
		subject = append(subject, raw)
	}
	return prior, subject, nil
}
