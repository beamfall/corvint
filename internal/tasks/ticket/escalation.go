package ticket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// These proposed codecs are deliberately separate from Record. They confer no
// native authority and are not yet accepted by a writer, import, or claim path.
const EscalationEventProfile = "taskman-escalation-event/0"
const EscalationRequestProfile = "taskman-escalation-request/0"
const EscalationMaxEventBytes = 65536

// EscalationSource identifies an immutable successful admission, not the latest
// projection of an attempt (whose identifier may be reused across generations).
type EscalationSource struct {
	QueueID            string      `json:"queueId"`
	TicketID           string      `json:"ticketId"`
	AttemptID          string      `json:"attemptId"`
	Generation         wire.Size   `json:"generation"`
	Holder             string      `json:"holder"`
	AcceptanceRevision wire.Count  `json:"acceptanceRevision"`
	ReceiptSequence    wire.Size   `json:"receiptSequence"`
	ReceiptSha256      wire.Digest `json:"receiptSha256"`
	PostAttemptSha256  wire.Digest `json:"postAttemptSha256"`
	TicketRecordSha256 wire.Digest `json:"ticketRecordSha256"`
}

type EscalationBlockedBy struct {
	TicketID string `json:"ticketId"`
	Gate     string `json:"gate,omitempty"`
}

type EscalationOpen struct {
	Source           EscalationSource     `json:"source"`
	Kind             string               `json:"kind"`
	Question         string               `json:"question"`
	Options          []string             `json:"options"`
	Supersedes       string               `json:"supersedes,omitempty"`
	ExpectedRevision wire.Count           `json:"expectedRevision,omitempty"`
	BlockedBy        *EscalationBlockedBy `json:"blockedBy,omitempty"`
}

// Empty RequestID/ExpectedRevision means sole-current-open-at-commit shorthand.
// A resolved question must never be inserted into this original request.
type EscalationAnswer struct {
	Text             string     `json:"text"`
	RequestID        string     `json:"requestId,omitempty"`
	ExpectedRevision wire.Count `json:"expectedRevision,omitempty"`
}

type EscalationRequest struct {
	Profile                string            `json:"profile"`
	QueueID                string            `json:"queueId"`
	TicketID               string            `json:"ticketId"`
	RequestID              string            `json:"requestId"`
	Actor                  string            `json:"actor"`
	ActorRole              string            `json:"actorRole"`
	Operation              string            `json:"operation"`
	ExpectedTicketRevision *wire.Count       `json:"expectedTicketRevision,omitempty"`
	Open                   *EscalationOpen   `json:"open,omitempty"`
	Answer                 *EscalationAnswer `json:"answer,omitempty"`
}

// OPEN has no self-digest. Its reference receives the encoded OPEN digest;
// later events bind that immutable origin separately from their request hash.
type EscalationEvent struct {
	Profile                  string            `json:"profile"`
	QueueID                  string            `json:"queueId"`
	TicketID                 string            `json:"ticketId"`
	EscalationID             string            `json:"escalationId"`
	Revision                 wire.Count        `json:"revision"`
	Operation                string            `json:"operation"`
	QuestionOriginSha256     *wire.Digest      `json:"questionOriginSha256,omitempty"`
	PreviousSha256           *wire.Digest      `json:"previousSha256,omitempty"`
	ReplacementID            string            `json:"replacementId,omitempty"`
	Source                   EscalationSource  `json:"source"`
	Actor                    string            `json:"actor"`
	ActorRole                string            `json:"actorRole"`
	RecordedAt               wire.Timestamp    `json:"recordedAt"`
	OriginalRequest          EscalationRequest `json:"originalRequest"`
	RequestSha256            wire.Digest       `json:"requestSha256"`
	ResolvedRequestID        string            `json:"resolvedRequestId"`
	ResolvedPreviousRevision wire.Count        `json:"resolvedPreviousRevision"`
}

type EscalationRef struct {
	RequestID          string      `json:"requestId"`
	OriginSha256       wire.Digest `json:"originSha256"`
	HeadSha256         wire.Digest `json:"headSha256"`
	Revision           wire.Count  `json:"revision"`
	AcceptanceRevision wire.Count  `json:"acceptanceRevision"`
	Kind               string      `json:"kind"`
	State              string      `json:"state"`
}

// Revision counts typed transactions. Sum(Entries.Revision) counts events:
// supersession emits two events but increments ticket content only once.
type EscalationRefs struct {
	Revision                  wire.Count      `json:"revision"`
	LastControlTicketRevision wire.Count      `json:"lastControlTicketRevision"`
	WorkRevision              wire.Count      `json:"workRevision"`
	Entries                   []EscalationRef `json:"entries"`
}

func escalationError(message string) error { return fmt.Errorf("escalation: %s", message) }
func escalationID(s string) error          { _, e := wire.ParseIdentifier("/escalation", s); return e }
func escalationPositive(c wire.Count) error {
	if _, e := wire.ParseCount("/escalation", string(c)); e != nil {
		return e
	}
	if c.Int() < 1 {
		return escalationError("positive revision required")
	}
	return nil
}
func escalationDigest(d wire.Digest) error {
	_, e := wire.ParseDigest("/escalation", string(d))
	return e
}
func escalationText(s string, max int) error {
	if _, e := wire.ParseProse("/escalation", s, 1, max); e != nil {
		return e
	}
	if strings.TrimSpace(s) == "" {
		return escalationError("blank text")
	}
	return nil
}
func escalationKind(k string) bool {
	return k == "decision" || k == "infrastructure" || k == "scope" || k == "blocked"
}
func escalationIdentity(q, t string) error {
	if _, e := wire.ParseQueueID("/queueId", q); e != nil {
		return e
	}
	id, e := wire.ParseTicketID("/ticketId", t)
	if e != nil {
		return e
	}
	if id.QueueID() != q {
		return escalationError("ticket/queue mismatch")
	}
	return nil
}

func (s EscalationSource) Validate() error {
	if e := escalationIdentity(s.QueueID, s.TicketID); e != nil {
		return e
	}
	if e := escalationID(s.AttemptID); e != nil {
		return e
	}
	// The ticket codec stays below snapshot; verify the attempt's queue prefix here.
	parts := strings.Split(s.AttemptID, ":")
	if len(parts) != 4 || !strings.HasPrefix(s.AttemptID, "attempt:"+strings.TrimPrefix(s.QueueID, "queue:")+":") || len(parts[3]) != 32 {
		return escalationError("attempt/queue mismatch")
	}
	if _, e := wire.ParseDigest("/attemptId", parts[3]+parts[3]); e != nil {
		return e
	}
	if e := escalationID(s.Holder); e != nil {
		return e
	}
	for _, v := range []wire.Size{s.Generation, s.ReceiptSequence} {
		if _, e := wire.ParseSize("/source", string(v)); e != nil {
			return e
		}
		if v.Uint64() == 0 {
			return escalationError("zero admission identity")
		}
	}
	if e := escalationPositive(s.AcceptanceRevision); e != nil {
		return e
	}
	for _, d := range []wire.Digest{s.ReceiptSha256, s.PostAttemptSha256, s.TicketRecordSha256} {
		if e := escalationDigest(d); e != nil {
			return e
		}
	}
	return nil
}

func (r EscalationRequest) Validate() error {
	if r.Profile != EscalationRequestProfile {
		return escalationError("request profile")
	}
	if e := escalationIdentity(r.QueueID, r.TicketID); e != nil {
		return e
	}
	for _, id := range []string{r.RequestID, r.Actor} {
		if e := escalationID(id); e != nil {
			return e
		}
	}
	if r.ActorRole != "OWNER" && r.ActorRole != "OPERATOR" {
		return escalationError("actor role")
	}
	if r.ExpectedTicketRevision != nil {
		if e := escalationPositive(*r.ExpectedTicketRevision); e != nil {
			return e
		}
	}
	switch r.Operation {
	case "OPEN":
		if r.Open == nil || r.Answer != nil {
			return escalationError("OPEN shape")
		}
		o := r.Open
		if e := o.Source.Validate(); e != nil {
			return e
		}
		if o.Source.QueueID != r.QueueID || o.Source.TicketID != r.TicketID || r.Actor != o.Source.Holder {
			return escalationError("source/request identity")
		}
		if !escalationKind(o.Kind) {
			return escalationError("kind")
		}
		if e := escalationText(o.Question, 4096); e != nil {
			return e
		}
		if o.Options == nil || len(o.Options) > 8 {
			return escalationError("options bound/array")
		}
		seen := map[string]bool{}
		for _, v := range o.Options {
			if e := escalationText(v, 256); e != nil {
				return e
			}
			if seen[v] {
				return escalationError("duplicate option")
			}
			seen[v] = true
		}
		if (o.Supersedes == "") != (o.ExpectedRevision == "") {
			return escalationError("supersedes/CAS pair")
		}
		if o.Supersedes != "" {
			if e := escalationID(o.Supersedes); e != nil {
				return e
			}
			if o.Supersedes == r.RequestID {
				return escalationError("self supersession")
			}
			if e := escalationPositive(o.ExpectedRevision); e != nil {
				return e
			}
		}
		if o.BlockedBy != nil {
			if o.Kind != "blocked" {
				return escalationError("blocked relation on other kind")
			}
			if e := escalationIdentity(r.QueueID, o.BlockedBy.TicketID); e != nil {
				return e
			}
			if o.BlockedBy.Gate != "" {
				if e := escalationID(o.BlockedBy.Gate); e != nil {
					return e
				}
			}
		}
	case "ANSWER":
		if r.Answer == nil || r.Open != nil {
			return escalationError("ANSWER shape")
		}
		a := r.Answer
		if e := escalationText(a.Text, 8192); e != nil {
			return e
		}
		if (a.RequestID == "") != (a.ExpectedRevision == "") {
			return escalationError("answer selector/CAS pair")
		}
		if a.RequestID != "" {
			if e := escalationID(a.RequestID); e != nil {
				return e
			}
			if e := escalationPositive(a.ExpectedRevision); e != nil {
				return e
			}
		}
	default:
		return escalationError("request operation")
	}
	return nil
}

func (e EscalationEvent) Validate() error {
	if e.Profile != EscalationEventProfile {
		return escalationError("event profile")
	}
	if err := e.OriginalRequest.Validate(); err != nil {
		return err
	}
	r := e.OriginalRequest
	if e.QueueID != r.QueueID || e.TicketID != r.TicketID || e.Actor != r.Actor || e.ActorRole != r.ActorRole {
		return escalationError("event/request binding")
	}
	if err := escalationID(e.EscalationID); err != nil {
		return err
	}
	if e.ResolvedRequestID != e.EscalationID {
		return escalationError("resolved identity")
	}
	if err := escalationPositive(e.Revision); err != nil {
		return err
	}
	if e.Revision.Int() > 64 {
		return escalationError("event revision cap")
	}
	if _, err := wire.ParseCount("/resolvedPreviousRevision", string(e.ResolvedPreviousRevision)); err != nil {
		return err
	}
	if err := e.Source.Validate(); err != nil {
		return err
	}
	if e.Source.QueueID != e.QueueID || e.Source.TicketID != e.TicketID {
		return escalationError("event source identity")
	}
	if _, err := wire.ParseTimestamp("/recordedAt", string(e.RecordedAt)); err != nil {
		return err
	}
	raw, err := EncodeEscalationRequest(r)
	if err != nil {
		return err
	}
	if wire.Sum(raw) != e.RequestSha256 {
		return escalationError("original request digest")
	}
	switch e.Operation {
	case "OPEN":
		if r.Operation != "OPEN" || e.EscalationID != r.RequestID || e.Revision != "1" || e.ResolvedPreviousRevision != "0" || e.QuestionOriginSha256 != nil || e.PreviousSha256 != nil || e.ReplacementID != "" || e.Source != r.Open.Source {
			return escalationError("OPEN material")
		}
	case "ANSWER", "SUPERSEDE":
		if e.Revision.Int() != e.ResolvedPreviousRevision.Int()+1 || e.ResolvedPreviousRevision.Int() < 1 || e.QuestionOriginSha256 == nil || e.PreviousSha256 == nil {
			return escalationError("event chain")
		}
		if err := escalationDigest(*e.QuestionOriginSha256); err != nil {
			return err
		}
		if err := escalationDigest(*e.PreviousSha256); err != nil {
			return err
		}
		if e.Operation == "ANSWER" {
			if r.Operation != "ANSWER" || e.ReplacementID != "" {
				return escalationError("ANSWER material")
			}
			if r.Answer.RequestID != "" && (r.Answer.RequestID != e.EscalationID || r.Answer.ExpectedRevision != e.ResolvedPreviousRevision) {
				return escalationError("answer selector binding")
			}
		} else {
			if r.Operation != "OPEN" || r.Open.Supersedes != e.EscalationID || r.Open.ExpectedRevision != e.ResolvedPreviousRevision || e.ReplacementID != r.RequestID || e.Source != r.Open.Source {
				return escalationError("SUPERSEDE material")
			}
		}
	default:
		return escalationError("event operation")
	}
	return nil
}

func (r EscalationRefs) Validate() error {
	for _, v := range []wire.Count{r.Revision, r.LastControlTicketRevision, r.WorkRevision} {
		if e := escalationPositive(v); e != nil {
			return e
		}
	}
	if r.WorkRevision.Int() >= r.LastControlTicketRevision.Int() {
		return escalationError("work/control revision")
	}
	if len(r.Entries) < 1 || len(r.Entries) > 64 {
		return escalationError("request count")
	}
	var total, open int64
	for i, e := range r.Entries {
		if err := escalationID(e.RequestID); err != nil {
			return err
		}
		if i > 0 && r.Entries[i-1].RequestID >= e.RequestID {
			return escalationError("unsorted/duplicate requests")
		}
		for _, d := range []wire.Digest{e.OriginSha256, e.HeadSha256} {
			if err := escalationDigest(d); err != nil {
				return err
			}
		}
		if err := escalationPositive(e.Revision); err != nil {
			return err
		}
		if e.Revision.Int() > 64 {
			return escalationError("request history cap")
		}
		total += e.Revision.Int()
		if err := escalationPositive(e.AcceptanceRevision); err != nil {
			return err
		}
		if !escalationKind(e.Kind) {
			return escalationError("reference kind")
		}
		switch e.State {
		case "OPEN":
			open++
		case "ANSWERED", "SUPERSEDED":
		default:
			return escalationError("reference lifecycle")
		}
	}
	if total > 4096 || open > 16 || r.Revision.Int() > total || total > 2*r.Revision.Int() {
		return escalationError("event/transaction/open capacity")
	}
	return nil
}

// Escalation codecs use the repository's number-free, duplicate-rejecting wire
// parser and canonical LF encoding. Strict JSON decoding closes nested structs;
// canonical readback also rejects absent required keys and explicit null aliases.
func escalationEncode(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var decoded any
	if e := json.Unmarshal(b, &decoded); e != nil {
		return nil, e
	}
	w, e := escalationWireValue(decoded)
	if e != nil {
		return nil, e
	}
	out := wire.EncodeFile(w)
	if _, e := wire.Parse(out); e != nil {
		return nil, e
	}
	if len(out) > EscalationMaxEventBytes {
		return nil, escalationError("encoded record cap")
	}
	return out, nil
}

// Convert only the closed structs' number-free JSON values into canonical wire
// values. External bytes always go through wire.Parse before JSON decoding.
func escalationWireValue(v any) (wire.Value, error) {
	switch x := v.(type) {
	case nil:
		return wire.Null(), nil
	case string:
		return wire.String(x), nil
	case bool:
		return wire.Bool(x), nil
	case []any:
		a := make([]wire.Value, len(x))
		for i, item := range x {
			v, e := escalationWireValue(item)
			if e != nil {
				return wire.Value{}, e
			}
			a[i] = v
		}
		return wire.Array(a...), nil
	case map[string]any:
		o := wire.NewObject()
		for k, item := range x {
			v, e := escalationWireValue(item)
			if e != nil {
				return wire.Value{}, e
			}
			o.Set(k, v)
		}
		return wire.ObjectValue(o), nil
	default:
		return wire.Value{}, escalationError("non-wire JSON value")
	}
}

func escalationDecode(raw []byte, out any) error {
	if len(raw) > EscalationMaxEventBytes {
		return escalationError("encoded record cap")
	}
	if _, e := wire.Parse(raw); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	canonical, e := escalationEncode(out)
	if e != nil {
		return e
	}
	if !bytes.Equal(raw, canonical) {
		return escalationError("noncanonical or missing fields")
	}
	return nil
}
func EncodeEscalationRequest(r EscalationRequest) ([]byte, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	return escalationEncode(r)
}
func DecodeEscalationRequest(b []byte) (EscalationRequest, error) {
	var r EscalationRequest
	if e := escalationDecode(b, &r); e != nil {
		return r, e
	}
	return r, r.Validate()
}
func EncodeEscalationEvent(v EscalationEvent) ([]byte, error) {
	if e := v.Validate(); e != nil {
		return nil, e
	}
	return escalationEncode(v)
}
func DecodeEscalationEvent(b []byte) (EscalationEvent, error) {
	var e EscalationEvent
	if err := escalationDecode(b, &e); err != nil {
		return e, err
	}
	return e, e.Validate()
}
func EncodeEscalationRefs(r EscalationRefs) ([]byte, error) {
	if e := r.Validate(); e != nil {
		return nil, e
	}
	return escalationEncode(r)
}
func DecodeEscalationRefs(b []byte) (EscalationRefs, error) {
	var r EscalationRefs
	if e := escalationDecode(b, &r); e != nil {
		return r, e
	}
	return r, r.Validate()
}
