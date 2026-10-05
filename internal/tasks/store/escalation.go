package store

import (
	"context"
	"os"
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

// Escalate commits one typed OPEN (ESC-V0-001): raw is the canonical
// taskman-escalation-request/0 document, whose requestId is the question's
// ID. AnswerEscalation commits one typed ANSWER (ESC-V0-004), whose requestId
// is the answer's own ID. Each posts the ticket and its events in one LEASE
// transaction; Report.EscalationEvents carries the committed events, read
// back from the receipt, so a replay recovers the resolved target.
func Escalate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, raw []byte, now wire.Timestamp) (*Report, error) {
	return escalationWrite(ctx, repo, actor, queueID, transaction.LeaseEscalate, raw, now)
}

func AnswerEscalation(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, raw []byte, now wire.Timestamp) (*Report, error) {
	return escalationWrite(ctx, repo, actor, queueID, transaction.LeaseAnswer, raw, now)
}

func escalationWrite(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID, verb string, raw []byte, now wire.Timestamp) (*Report, error) {
	req, err := ticket.DecodeEscalationRequest(raw)
	if err != nil {
		return &Report{}, err
	}
	if refused := transaction.EscalationActorRefusal(req.RequestID, raw, actor); refused != nil {
		report := &Report{}
		setLeaseReport(report, *refused)
		return report, nil
	}
	lease := transaction.LeaseRequest{Verb: verb, Evidence: string(wire.Sum(raw))}
	request := transaction.Request{Operation: transaction.Lease, QueueID: queueID, RequestID: req.RequestID, Actor: actor, Lease: &lease}
	facts := func(_ *journal.Result, input *transaction.Input) (transaction.LeaseFacts, error) {
		f, err := escalationFacts(repo, req, raw, input.CanonicalTickets)
		return transaction.LeaseFacts{Escalation: f}, err
	}
	report, _, err := administrativeWriteWith(ctx, repo, request, now, nil, facts)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Outcome.ReceiptSeq == nil {
		return report, err
	}
	return report, escalationEvents(repo, report)
}

// escalationFacts reads what the planner audits: the claim receipt an OPEN
// names and, by digest, the claim's blob POST attempt and the ticket's
// question origins and heads. Each read is bounded; a missing file is left
// out, so the planner abstains rather than trusting it.
func escalationFacts(repo *intent.Repository, req ticket.EscalationRequest, raw []byte, tickets [][]byte) (*transaction.EscalationFacts, error) {
	f := &transaction.EscalationFacts{Request: raw, Blobs: map[wire.Digest][]byte{}}
	if req.Operation == "OPEN" {
		if rc, err := readReceiptBytes(repo, req.Open.Source.ReceiptSequence.Uint64()); err == nil {
			f.ClaimReceipt = rc
			if decoded, err := snapshot.DecodeReceipt(rc); err == nil {
				for _, p := range decoded.Post {
					if p.BlobSha256 != nil && strings.HasPrefix(p.Path, "attempts/") {
						if err := readEvidence(repo, *p.BlobSha256, wire.MaxAttemptRecordBytes, f.Blobs); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	for _, b := range tickets {
		rec, err := ticket.Decode(b)
		if err != nil || rec.TicketID.Raw != req.TicketID || rec.Escalations == nil {
			continue
		}
		for _, en := range rec.Escalations.Entries {
			for _, d := range []wire.Digest{en.OriginSha256, en.HeadSha256} {
				if err := readEvidence(repo, d, ticket.EscalationMaxEventBytes, f.Blobs); err != nil {
					return nil, err
				}
			}
		}
	}
	return f, nil
}

func readEvidence(repo *intent.Repository, d wire.Digest, limit int, into map[wire.Digest][]byte) error {
	if _, ok := into[d]; ok {
		return nil
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)), limit)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	into[d] = raw
	return nil
}

// escalationEvents reads the events a committed or replayed escalation
// posted from its receipt and evidence, verifying each digest.
func escalationEvents(repo *intent.Repository, report *Report) error {
	rc, err := readReceipt(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return err
	}
	if rc.TicketID != nil {
		report.Ticket = rc.TicketID.Raw
	}
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "evidence/") || p.Sha256 == nil {
			continue
		}
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(*p.Sha256)), ticket.EscalationMaxEventBytes)
		if err != nil {
			return err
		}
		if wire.Sum(raw) != *p.Sha256 {
			return wire.Errorf(wire.CodeJournalForked, "escalation", "event digest differs")
		}
		ev, err := ticket.DecodeEscalationEvent(raw)
		if err != nil {
			return err
		}
		report.EscalationEvents = append(report.EscalationEvents, ev)
	}
	return nil
}
