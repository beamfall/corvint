package ticket

// Experimental operator-note codecs. These standalone types are not installed
// in Record: writer, recovery, import and claim integration remain separate work.
import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const OperatorNoteProfile = "taskman-operator-note/0"
const MaxOperatorNoteEvents = 4096
const MaxOperatorNoteBytes = 65536
const MaxOperatorNoteTextBytes = 8192

// OperatorNoteReference is absent until the first note event. A nil Current in
// a present reference means CLEARED, not never-noted. History is immutable.
type OperatorNoteReference struct {
	Revision wire.Count
	Current  *wire.Digest
	Head     wire.Digest
}

func (n OperatorNoteReference) Value() wire.Value {
	current := wire.Null()
	if n.Current != nil {
		current = wire.String(string(*n.Current))
	}
	return noteObject("revision", wire.String(string(n.Revision)), "current", current, "head", wire.String(string(n.Head)))
}

func OperatorNoteReferenceFromValue(v wire.Value) (*OperatorNoteReference, error) {
	r := wire.NewReader(v, "/operatorNote")
	r.Closed("revision", "current", "head")
	n := &OperatorNoteReference{Revision: r.Field("revision").Count(), Current: r.Field("current").DigestOrNull(), Head: r.Field("head").Digest()}
	if err := r.Err(); err != nil {
		return nil, err
	}
	if n.Revision.Int() < 1 || n.Revision.Int() > MaxOperatorNoteEvents {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/operatorNote/revision", "note revision must be 1..4096")
	}
	if n.Current != nil && *n.Current != n.Head {
		return nil, wire.Errorf(wire.CodeMalformed, "/operatorNote/current", "current must equal head or null")
	}
	return n, nil
}

// OperatorNoteRequest is a wire-only view of the retained original envelope.
// Actor fields are claims, never authentication. Mutation owns authority/CAS.
type OperatorNoteRequest struct {
	RequestID          string
	QueueID            wire.QueueID
	TicketID           wire.TicketID
	ExpectedRevision   *wire.Count
	ActorID, ActorRole string
	Operation          string
	Text               string
	Supersedes         *wire.Count
	IssuedAt           wire.Timestamp
	Raw                []byte
}

// DecodeOperatorNoteRequest does not extend the ordinary mutation vocabulary.
func DecodeOperatorNoteRequest(raw []byte) (*OperatorNoteRequest, error) {
	if len(raw) > MaxOperatorNoteBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/request", "note request too large")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/request")
	r.Closed("profile", "requestId", "actor", "queueId", "targetId", "expectedRevision", "operation", "payload", "issuedAt")
	if err = r.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("/request/profile", r.Field("profile").String(), "taskman-mutation/0"); err != nil {
		return nil, err
	}
	q := &OperatorNoteRequest{}
	q.RequestID = r.Field("requestId").Identifier()
	q.QueueID = r.Field("queueId").QueueID()
	q.TicketID = r.Field("targetId").TicketID()
	q.ExpectedRevision = r.Field("expectedRevision").CountOrNull()
	a := r.Field("actor")
	a.Closed("id", "role")
	q.ActorID = a.Field("id").Label()
	q.ActorRole = a.Field("role").Enum("OWNER", "OPERATOR")
	q.Operation = r.Field("operation").Enum("NOTE_SET", "NOTE_CLEAR")
	q.IssuedAt = r.Field("issuedAt").Timestamp()
	p := r.Field("payload")
	if q.Operation == "NOTE_SET" {
		p.Closed("text", "supersedes")
		q.Text = p.Field("text").Prose(1, MaxOperatorNoteTextBytes)
	} else {
		p.Closed("supersedes")
	}
	q.Supersedes = p.Field("supersedes").CountOrNull()
	if err = r.Err(); err != nil {
		return nil, err
	}
	if len(q.RequestID) > wire.MaxRequestIDBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/request/requestId", "request ID too large")
	}
	if q.TicketID.QueueID() != q.QueueID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/targetId", "ticket outside request queue")
	}
	if q.Operation == "NOTE_SET" && strings.TrimSpace(q.Text) == "" {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/payload/text", "note must contain non-whitespace prose")
	}
	q.Raw = append([]byte(nil), raw...)
	return q, nil
}

// OperatorNoteEvent retains exactly one original request. Text is not duplicated
// outside that request. RecordedAt is the writer's clock, independent of IssuedAt.
type OperatorNoteEvent struct {
	TicketID                                         wire.TicketID
	NoteRevision, TicketRevision, AcceptanceRevision wire.Count
	Operation                                        string
	Previous                                         *wire.Digest
	ActorID, ActorRole                               string
	RecordedAt                                       wire.Timestamp
	RequestSha256                                    wire.Digest
	Request                                          []byte
}

// Encode validates the proposed event through the same closed decoder as reads.
func (n OperatorNoteEvent) Encode() ([]byte, error) {
	q, err := wire.Parse(n.Request)
	if err != nil {
		return nil, err
	}
	previous := wire.Null()
	if n.Previous != nil {
		previous = wire.String(string(*n.Previous))
	}
	v := noteObject("profile", wire.String(OperatorNoteProfile), "ticketId", wire.String(n.TicketID.Raw),
		"noteRevision", wire.String(string(n.NoteRevision)), "ticketRevision", wire.String(string(n.TicketRevision)), "acceptanceRevision", wire.String(string(n.AcceptanceRevision)),
		"operation", wire.String(n.Operation), "previous", previous, "actor", noteObject("id", wire.String(n.ActorID), "role", wire.String(n.ActorRole)),
		"recordedAt", wire.String(string(n.RecordedAt)), "requestSha256", wire.String(string(n.RequestSha256)), "request", q)
	raw := wire.EncodeFile(v)
	if _, err = DecodeOperatorNoteEvent(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func DecodeOperatorNoteEvent(raw []byte) (*OperatorNoteEvent, error) {
	if len(raw) > MaxOperatorNoteBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "note event exceeds 65536 bytes")
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "ticketId", "noteRevision", "ticketRevision", "acceptanceRevision", "operation", "previous", "actor", "recordedAt", "requestSha256", "request")
	if err = r.Err(); err != nil {
		return nil, err
	}
	if err = wire.CheckProfile("/profile", r.Field("profile").String(), OperatorNoteProfile); err != nil {
		return nil, err
	}
	n := &OperatorNoteEvent{TicketID: r.Field("ticketId").TicketID(), NoteRevision: r.Field("noteRevision").Count(), TicketRevision: r.Field("ticketRevision").Count(), AcceptanceRevision: r.Field("acceptanceRevision").Count(), Operation: r.Field("operation").Enum("SET", "CLEAR"), Previous: r.Field("previous").DigestOrNull(), RecordedAt: r.Field("recordedAt").Timestamp(), RequestSha256: r.Field("requestSha256").Digest()}
	a := r.Field("actor")
	a.Closed("id", "role")
	n.ActorID = a.Field("id").Label()
	n.ActorRole = a.Field("role").Enum("OWNER", "OPERATOR")
	if err = r.Err(); err != nil {
		return nil, err
	}
	request, _ := v.Obj.Get("request")
	n.Request = wire.EncodeFile(request)
	q, err := DecodeOperatorNoteRequest(n.Request)
	if err != nil {
		return nil, err
	}
	if n.NoteRevision.Int() < 1 || n.NoteRevision.Int() > MaxOperatorNoteEvents {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/noteRevision", "note revision must be 1..4096")
	}
	if n.TicketRevision.Int() < 1 || n.AcceptanceRevision.Int() < 1 || n.AcceptanceRevision.Int() > n.TicketRevision.Int() {
		return nil, wire.Errorf(wire.CodeMalformed, "/ticketRevision", "invalid note-write revisions")
	}
	if (n.NoteRevision == "1") != (n.Previous == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/previous", "only first event has null previous")
	}
	if wire.Sum(n.Request) != n.RequestSha256 {
		return nil, wire.Errorf(wire.CodeMalformed, "/requestSha256", "retained request digest mismatch")
	}
	if q.TicketID.Raw != n.TicketID.Raw || q.ActorID != n.ActorID || q.ActorRole != n.ActorRole || q.Operation != "NOTE_"+n.Operation {
		return nil, wire.Errorf(wire.CodeMalformed, "/request", "event/request identity mismatch")
	}
	return n, nil
}

// ResolveOperatorNote verifies the referenced historical note-write event. It
// deliberately does not compare its revisions to a later unrelated ticket edit.
func ResolveOperatorNote(id wire.TicketID, ref OperatorNoteReference, raw []byte) (*OperatorNoteEvent, error) {
	if _, err := OperatorNoteReferenceFromValue(ref.Value()); err != nil {
		return nil, err
	}
	n, err := DecodeOperatorNoteEvent(raw)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != ref.Head || n.TicketID.Raw != id.Raw || n.NoteRevision != ref.Revision || (n.Operation == "SET") != (ref.Current != nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/operatorNote", "event/reference binding mismatch")
	}
	return n, nil
}

func noteObject(kv ...any) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
