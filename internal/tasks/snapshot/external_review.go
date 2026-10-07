package snapshot

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
)

// External reviews are routing-only evidence. These experimental codecs do not
// implement a writer or satisfy executable gates or completion dispositions.
const (
	ProfileExternalReviewRequest = "taskman-external-review-request/0"
	ProfileExternalReviewEvent   = "taskman-external-review-event/0"
	MaxExternalReviewBytes       = 64 * wire.KiB
	MaxExternalReviewEvents      = 4096
)

type ExternalReviewCandidate struct {
	Kind, TreeOID string
	Sha256        wire.Digest
	Bytes         wire.Size
}
type ExternalReviewSubject struct {
	AttemptID                    string
	Generation, ReceiptSeq       wire.Size
	ReceiptSha256, AttemptSha256 wire.Digest
}
type ExternalReviewLease struct {
	AttemptID, Holder string
	Generation        wire.Size
}
type ExternalReviewReason struct{ Code, Text string }
type ExternalReviewRequest struct {
	RequestID, Action, TicketID, GateID                      string
	ExpectedGeneration, ExpectedRevision, AcceptanceRevision wire.Count
	DefinitionSha256, PolicySha256                           wire.Digest
	Subject                                                  ExternalReviewSubject
	Candidate                                                ExternalReviewCandidate
	Reasons                                                  []ExternalReviewReason
	Evidence                                                 []GateEvidence
	ReviewerLease, AuthorLease                               *ExternalReviewLease
	PriorReturn                                              *wire.Digest
	Verdict                                                  *string
}
type ExternalReviewRef struct {
	Generation, Revision wire.Count
	Head                 wire.Digest
}
type ExternalReviewEvent struct {
	Request                         ExternalReviewRequest
	ReviewGeneration, EventRevision wire.Count
	ActorID, ActorRole, TrustSource string
	Previous                        *wire.Digest
	RecordedAt                      wire.Timestamp
	ReceiptSeq                      wire.Size
}

// Value is the candidate's canonical closed object.
func (c ExternalReviewCandidate) Value() wire.Value { return externalCandidateValue(c) }

// Value is the subject's canonical closed object.
func (s ExternalReviewSubject) Value() wire.Value { return externalSubjectValue(s) }

func externalCandidateValue(c ExternalReviewCandidate) wire.Value {
	o := wire.NewObject().Set("kind", wire.String(c.Kind))
	if c.Kind == "TREE" {
		o.Set("treeOid", wire.String(c.TreeOID))
	} else {
		o.Set("sha256", wire.String(string(c.Sha256))).Set("bytes", wire.String(string(c.Bytes)))
	}
	return wire.ObjectValue(o)
}
func readExternalCandidate(r *wire.Reader) ExternalReviewCandidate {
	c := ExternalReviewCandidate{Kind: r.Field("kind").Enum("TREE", "EVIDENCE")}
	if c.Kind == "TREE" {
		r.Closed("kind", "treeOid")
		c.TreeOID = r.Field("treeOid").OID()
	} else {
		r.Closed("kind", "sha256", "bytes")
		c.Sha256 = r.Field("sha256").Digest()
		c.Bytes = r.Field("bytes").Size()
	}
	return c
}
func externalSubjectValue(s ExternalReviewSubject) wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("attemptId", wire.String(s.AttemptID)).Set("generation", wire.String(string(s.Generation))).Set("receiptSeq", wire.String(string(s.ReceiptSeq))).Set("receiptSha256", wire.String(string(s.ReceiptSha256))).Set("attemptSha256", wire.String(string(s.AttemptSha256))))
}
func readExternalSubject(r *wire.Reader) ExternalReviewSubject {
	r.Closed("attemptId", "generation", "receiptSeq", "receiptSha256", "attemptSha256")
	s := ExternalReviewSubject{r.Field("attemptId").Identifier(), r.Field("generation").Size(), r.Field("receiptSeq").Size(), r.Field("receiptSha256").Digest(), r.Field("attemptSha256").Digest()}
	if s.Generation == "0" || s.ReceiptSeq == "0" {
		r.Fail(wire.CodeMalformed, "subject generation and receipt sequence must be positive")
	}
	return s
}
func externalLeaseValue(l *ExternalReviewLease) wire.Value {
	if l == nil {
		return wire.Null()
	}
	return wire.ObjectValue(wire.NewObject().Set("attemptId", wire.String(l.AttemptID)).Set("generation", wire.String(string(l.Generation))).Set("holder", wire.String(l.Holder)))
}
func readExternalLease(r *wire.Reader) *ExternalReviewLease {
	if r.IsNull() {
		return nil
	}
	r.Closed("attemptId", "generation", "holder")
	l := &ExternalReviewLease{AttemptID: r.Field("attemptId").Identifier(), Generation: r.Field("generation").Size(), Holder: r.Field("holder").Label()}
	if l.Generation == "0" {
		r.Fail(wire.CodeMalformed, "lease generation must be positive")
	}
	return l
}
func externalReasonsValue(rs []ExternalReviewReason) wire.Value {
	a := []wire.Value{}
	for _, x := range rs {
		a = append(a, wire.ObjectValue(wire.NewObject().Set("code", wire.String(x.Code)).Set("text", wire.String(x.Text))))
	}
	return wire.Array(a...)
}
func readExternalReasons(r *wire.Reader) []ExternalReviewReason {
	a := []ExternalReviewReason{}
	total := 0
	for _, x := range r.Array(16, false) {
		x.Closed("code", "text")
		v := ExternalReviewReason{x.Field("code").Label(), x.Field("text").Prose(1, 512)}
		total += len(v.Text)
		a = append(a, v)
	}
	if total > 4096 {
		r.Fail(wire.CodeLimitExceeded, "reason text exceeds 4096 bytes")
	}
	return a
}
func externalEvidenceValue(es []GateEvidence) wire.Value {
	a := []wire.Value{}
	for _, x := range es {
		a = append(a, wire.ObjectValue(wire.NewObject().Set("label", wire.String(x.Label)).Set("sha256", wire.String(string(x.Sha256))).Set("bytes", wire.String(string(x.Bytes)))))
	}
	return wire.Array(a...)
}
func readExternalEvidence(r *wire.Reader) []GateEvidence {
	a := []GateEvidence{}
	for _, x := range r.Array(16, false) {
		x.Closed("label", "sha256", "bytes")
		a = append(a, GateEvidence{x.Field("label").Label(), x.Field("sha256").Digest(), x.Field("bytes").Size()})
	}
	return a
}
func (q ExternalReviewRequest) value() wire.Value {
	return wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ProfileExternalReviewRequest)).Set("requestId", wire.String(q.RequestID)).Set("action", wire.String(q.Action)).Set("ticketId", wire.String(q.TicketID)).Set("gateId", wire.String(q.GateID)).Set("expectedGeneration", wire.String(string(q.ExpectedGeneration))).Set("expectedRevision", wire.String(string(q.ExpectedRevision))).Set("acceptanceRevision", wire.String(string(q.AcceptanceRevision))).Set("definitionSha256", wire.String(string(q.DefinitionSha256))).Set("policySha256", wire.String(string(q.PolicySha256))).Set("subject", externalSubjectValue(q.Subject)).Set("candidate", externalCandidateValue(q.Candidate)).Set("reasons", externalReasonsValue(q.Reasons)).Set("evidence", externalEvidenceValue(q.Evidence)).Set("reviewerLease", externalLeaseValue(q.ReviewerLease)).Set("authorLease", externalLeaseValue(q.AuthorLease)).Set("priorReturn", digestOrNull(q.PriorReturn)).Set("verdict", wire.StringOrNull(q.Verdict)))
}
func externalParse(raw []byte) (*wire.Reader, error) {
	if len(raw) > MaxExternalReviewBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "external review exceeds 65536 bytes")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wire.NewReader(v, "/"), nil
}

// DecodeExternalReviewRequest validates a closed request; omitted nullable
// fields are refused rather than inferred from mutable state.
func DecodeExternalReviewRequest(raw []byte) (*ExternalReviewRequest, error) {
	r, err := externalParse(raw)
	if err != nil {
		return nil, err
	}
	if err = r.Profile(ProfileExternalReviewRequest); err != nil {
		return nil, err
	}
	r.Closed("profile", "requestId", "action", "ticketId", "gateId", "expectedGeneration", "expectedRevision", "acceptanceRevision", "definitionSha256", "policySha256", "subject", "candidate", "reasons", "evidence", "reviewerLease", "authorLease", "priorReturn", "verdict")
	if err = wire.CheckProfile("/profile", r.Field("profile").String(), ProfileExternalReviewRequest); err != nil {
		return nil, err
	}
	q := &ExternalReviewRequest{RequestID: r.Field("requestId").Identifier(), Action: r.Field("action").Enum("RECORD", "RESUBMIT"), TicketID: r.Field("ticketId").TicketID().Raw, GateID: r.Field("gateId").Label(), ExpectedGeneration: r.Field("expectedGeneration").Count(), ExpectedRevision: r.Field("expectedRevision").Count(), AcceptanceRevision: r.Field("acceptanceRevision").Count(), DefinitionSha256: r.Field("definitionSha256").Digest(), PolicySha256: r.Field("policySha256").Digest(), Subject: readExternalSubject(r.Field("subject")), Candidate: readExternalCandidate(r.Field("candidate")), Reasons: readExternalReasons(r.Field("reasons")), Evidence: readExternalEvidence(r.Field("evidence")), ReviewerLease: readExternalLease(r.Field("reviewerLease")), AuthorLease: readExternalLease(r.Field("authorLease")), PriorReturn: r.Field("priorReturn").DigestOrNull(), Verdict: r.Field("verdict").StringOrNull(func(x *wire.Reader) string { return x.Enum("PASS", "RETURN") })}
	if err = r.Err(); err != nil {
		return nil, err
	}
	if (q.ExpectedGeneration == "0") != (q.ExpectedRevision == "0") || q.ExpectedGeneration.Int() > q.ExpectedRevision.Int() {
		return nil, wire.Errorf(wire.CodeMalformed, "/expectedGeneration", "inconsistent review counters")
	}
	if q.Action == "RECORD" {
		if q.Verdict == nil || q.AuthorLease != nil || q.PriorReturn != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/action", "RECORD requires verdict, null author lease and priorReturn")
		}
		if *q.Verdict == "RETURN" && len(q.Reasons) == 0 {
			return nil, wire.Errorf(wire.CodeMalformed, "/reasons", "RETURN requires a reason")
		}
	} else {
		if q.Verdict != nil || q.ReviewerLease != nil || q.AuthorLease == nil || q.PriorReturn == nil || q.ExpectedRevision == "0" || len(q.Reasons) == 0 {
			return nil, wire.Errorf(wire.CodeMalformed, "/action", "RESUBMIT requires author lease, priorReturn, prior counters and reasons; null verdict/reviewer lease")
		}
	}
	return q, nil
}
func (q ExternalReviewRequest) Encode() ([]byte, error) {
	if (q.Candidate.Kind == "TREE" && (q.Candidate.Sha256 != "" || q.Candidate.Bytes != "")) || (q.Candidate.Kind == "EVIDENCE" && q.Candidate.TreeOID != "") {
		return nil, wire.Errorf(wire.CodeMalformed, "/candidate", "inactive union fields must be empty")
	}
	raw := wire.EncodeFile(q.value())
	if _, err := DecodeExternalReviewRequest(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func (e ExternalReviewEvent) Encode() ([]byte, error) {
	request, err := e.Request.Encode()
	if err != nil {
		return nil, err
	}
	q := e.Request
	o := wire.NewObject().Set("profile", wire.String(ProfileExternalReviewEvent)).Set("ticketId", wire.String(q.TicketID)).Set("acceptanceRevision", wire.String(string(q.AcceptanceRevision))).Set("gateId", wire.String(q.GateID)).Set("definitionSha256", wire.String(string(q.DefinitionSha256))).Set("policySha256", wire.String(string(q.PolicySha256))).Set("reviewGeneration", wire.String(string(e.ReviewGeneration))).Set("eventRevision", wire.String(string(e.EventRevision))).Set("action", wire.String(q.Action)).Set("subject", externalSubjectValue(q.Subject)).Set("candidate", externalCandidateValue(q.Candidate)).Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(e.ActorID)).Set("role", wire.String(e.ActorRole)))).Set("reviewerLease", externalLeaseValue(q.ReviewerLease)).Set("trust", wire.ObjectValue(wire.NewObject().Set("actorAuthentication", wire.String("NOT_OBSERVED")).Set("independence", wire.String("NOT_OBSERVED")).Set("source", wire.String(e.TrustSource)))).Set("verdict", wire.StringOrNull(q.Verdict)).Set("reasons", externalReasonsValue(q.Reasons)).Set("evidence", externalEvidenceValue(q.Evidence)).Set("previous", digestOrNull(e.Previous)).Set("recordedAt", wire.String(string(e.RecordedAt))).Set("receiptSeq", wire.String(string(e.ReceiptSeq))).Set("requestSha256", wire.String(string(wire.Sum(request)))).Set("request", q.value())
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeExternalReviewEvent(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
func DecodeExternalReviewEvent(raw []byte) (*ExternalReviewEvent, error) {
	r, err := externalParse(raw)
	if err != nil {
		return nil, err
	}
	if err = r.Profile(ProfileExternalReviewEvent); err != nil {
		return nil, err
	}
	r.Closed("profile", "ticketId", "acceptanceRevision", "gateId", "definitionSha256", "policySha256", "reviewGeneration", "eventRevision", "action", "subject", "candidate", "actor", "reviewerLease", "trust", "verdict", "reasons", "evidence", "previous", "recordedAt", "receiptSeq", "requestSha256", "request")
	if err = wire.CheckProfile("/profile", r.Field("profile").String(), ProfileExternalReviewEvent); err != nil {
		return nil, err
	}
	request := wire.EncodeFile(r.Field("request").Value())
	q, err := DecodeExternalReviewRequest(request)
	if err != nil {
		return nil, err
	}
	if r.Field("requestSha256").Digest() != wire.Sum(request) {
		return nil, wire.Errorf(wire.CodeMalformed, "/requestSha256", "original request digest differs")
	}
	if r.Field("ticketId").TicketID().Raw != q.TicketID || r.Field("gateId").Label() != q.GateID || r.Field("acceptanceRevision").Count() != q.AcceptanceRevision || r.Field("definitionSha256").Digest() != q.DefinitionSha256 || r.Field("policySha256").Digest() != q.PolicySha256 || r.Field("action").Enum("RECORD", "RESUBMIT") != q.Action || readExternalSubject(r.Field("subject")) != q.Subject || readExternalCandidate(r.Field("candidate")) != q.Candidate || !reflect.DeepEqual(readExternalLease(r.Field("reviewerLease")), q.ReviewerLease) || !reflect.DeepEqual(r.Field("verdict").StringOrNull(func(x *wire.Reader) string { return x.Enum("PASS", "RETURN") }), q.Verdict) || !reflect.DeepEqual(readExternalReasons(r.Field("reasons")), q.Reasons) || !reflect.DeepEqual(readExternalEvidence(r.Field("evidence")), q.Evidence) {
		return nil, wire.Errorf(wire.CodeMalformed, "/request", "event and retained request differ")
	}
	actor := r.Field("actor").Closed("id", "role")
	trust := r.Field("trust").Closed("actorAuthentication", "independence", "source")
	trust.Field("actorAuthentication").Enum("NOT_OBSERVED")
	trust.Field("independence").Enum("NOT_OBSERVED")
	e := &ExternalReviewEvent{Request: *q, ReviewGeneration: r.Field("reviewGeneration").Count(), EventRevision: r.Field("eventRevision").Count(), ActorID: actor.Field("id").Label(), ActorRole: actor.Field("role").Enum("OWNER", "OPERATOR", "REVIEWER", "WORKER"), TrustSource: trust.Field("source").Enum("LEASE_BOUND", "OPERATOR_ATTESTED"), Previous: r.Field("previous").DigestOrNull(), RecordedAt: r.Field("recordedAt").Timestamp(), ReceiptSeq: r.Field("receiptSeq").Size()}
	if err = r.Err(); err != nil {
		return nil, err
	}
	if e.ReviewGeneration == "0" || e.ReviewGeneration.Int() > e.EventRevision.Int() || e.EventRevision.Int() > MaxExternalReviewEvents || e.ReceiptSeq == "0" || (e.EventRevision == "1") != (e.Previous == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/eventRevision", "invalid event counters or previous link")
	}
	if e.EventRevision.Int() != q.ExpectedRevision.Int()+1 || e.ReviewGeneration.Int() < q.ExpectedGeneration.Int() || e.ReviewGeneration.Int() > q.ExpectedGeneration.Int()+1 || (q.ExpectedGeneration == "0" && e.ReviewGeneration != "1") || (q.Action == "RESUBMIT" && e.ReviewGeneration.Int() != q.ExpectedGeneration.Int()+1) {
		return nil, wire.Errorf(wire.CodeMalformed, "/eventRevision", "event counters do not follow retained request")
	}
	if e.TrustSource == "OPERATOR_ATTESTED" && (q.Action != "RECORD" || q.ReviewerLease != nil || (e.ActorRole != "OWNER" && e.ActorRole != "OPERATOR")) {
		return nil, wire.Errorf(wire.CodeMalformed, "/trust", "operator attestation requires OWNER/OPERATOR record without reviewer lease")
	}
	if e.TrustSource == "LEASE_BOUND" && q.ReviewerLease == nil && q.AuthorLease == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/trust", "lease bound event requires explicit lease")
	}
	return e, nil
}

// DecodeExternalReviewRef validates a single bounded pointer, not inline history.
func DecodeExternalReviewRef(raw []byte) (*ExternalReviewRef, error) {
	r, err := externalParse(raw)
	if err != nil {
		return nil, err
	}
	r.Closed("generation", "revision", "head")
	v := &ExternalReviewRef{r.Field("generation").Count(), r.Field("revision").Count(), r.Field("head").Digest()}
	if err = r.Err(); err != nil {
		return nil, err
	}
	if v.Generation == "0" || v.Generation.Int() > v.Revision.Int() || v.Revision.Int() > MaxExternalReviewEvents {
		return nil, wire.Errorf(wire.CodeMalformed, "/revision", "invalid reference counters")
	}
	return v, nil
}
func (v ExternalReviewRef) Encode() ([]byte, error) {
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("generation", wire.String(string(v.Generation))).Set("revision", wire.String(string(v.Revision))).Set("head", wire.String(string(v.Head)))))
	_, err := DecodeExternalReviewRef(raw)
	return raw, err
}

// CanonicalExternalReviewEvent rejects alternate encodings at a content-addressed boundary.
func CanonicalExternalReviewEvent(raw []byte) (*ExternalReviewEvent, error) {
	e, err := DecodeExternalReviewEvent(raw)
	if err != nil {
		return nil, err
	}
	again, err := e.Encode()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, again) {
		return nil, wire.Errorf(wire.CodeMalformed, "/", "event is not canonical JSON+LF")
	}
	return e, nil
}
