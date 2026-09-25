package mutation

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// OutcomeProfile is the taskman-outcome/0 profile (§3.3).
const OutcomeProfile = "taskman-outcome/0"

// Outcome values of taskman-outcome/0 (§3.3), closed.
const (
	OutcomeCompleted         = "COMPLETED"
	OutcomeRevisionConflict  = "REVISION_CONFLICT"
	OutcomeValidationFailed  = "VALIDATION_FAILED"
	OutcomeUnauthorized      = "UNAUTHORIZED"
	OutcomeBlocked           = "BLOCKED"
	OutcomeUnsupported       = "UNSUPPORTED"
	OutcomeCapacityExhausted = "CAPACITY_EXHAUSTED"
	OutcomeUncertainEffect   = "UNCERTAIN_EFFECT"
	OutcomeRequestIDConflict = "REQUEST_ID_CONFLICT"
	OutcomeStorageFailed     = "STORAGE_FAILED"
)

// Outcomes is the closed outcome vocabulary.
var Outcomes = []string{
	OutcomeCompleted, OutcomeRevisionConflict, OutcomeValidationFailed, OutcomeUnauthorized,
	OutcomeBlocked, OutcomeUnsupported, OutcomeCapacityExhausted, OutcomeUncertainEffect,
	OutcomeRequestIDConflict, OutcomeStorageFailed,
}

// Outcome is a taskman-outcome/0 document. ReceiptSeq is nil for every
// outcome this package produces: a planned result has no receipt. Only the
// journal commit of TCP-02b fills it, and a Replayed outcome carries whatever
// the recorded original carried.
type Outcome struct {
	RequestID                   string
	Outcome                     string
	Replayed                    bool
	ResultingRevision           *wire.Count
	ResultingAcceptanceRevision *wire.Count
	ReleaseID                   *string
	ResultingReleaseRevision    *wire.Count
	ReceiptSeq                  *wire.Size
	Codes                       []string
}

// Value renders the outcome. Codes are a non-semantic array and are sorted
// without duplicates.
func (o *Outcome) Value() wire.Value {
	v := wire.NewObject()
	v.Set("profile", wire.String(OutcomeProfile))
	v.Set("requestId", wire.String(o.RequestID))
	v.Set("outcome", wire.String(o.Outcome))
	v.Set("replayed", wire.Bool(o.Replayed))
	v.Set("resultingRevision", countOrNull(o.ResultingRevision))
	v.Set("resultingAcceptanceRevision", countOrNull(o.ResultingAcceptanceRevision))
	if o.ReleaseID != nil {
		v.Set("releaseId", wire.String(*o.ReleaseID))
		v.Set("resultingReleaseRevision", countOrNull(o.ResultingReleaseRevision))
	}
	if o.ReceiptSeq == nil {
		v.Set("receiptSeq", wire.Null())
	} else {
		v.Set("receiptSeq", wire.String(string(*o.ReceiptSeq)))
	}
	v.Set("codes", wire.Strings(sortedUnique(o.Codes)))
	return wire.ObjectValue(v)
}

// Encode validates the outcome against its closed schema and the §1 bound
// and returns the transport bytes.
func (o *Outcome) Encode() ([]byte, error) {
	raw := wire.EncodeFile(o.Value())
	if _, err := DecodeOutcome(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// DecodeOutcome parses and validates a taskman-outcome/0 (≤64 KiB).
func DecodeOutcome(data []byte) (*Outcome, error) {
	if len(data) > wire.MaxOutcomeBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "outcome larger than %d bytes", wire.MaxOutcomeBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	if _, ok := v.Obj.Get("releaseId"); !ok {
		v.Obj.Set("releaseId", wire.Null())
	}
	if _, ok := v.Obj.Get("resultingReleaseRevision"); !ok {
		v.Obj.Set("resultingReleaseRevision", wire.Null())
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "requestId", "outcome", "replayed", "resultingRevision", "resultingAcceptanceRevision", "releaseId", "resultingReleaseRevision", "receiptSeq", "codes")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), OutcomeProfile); err != nil {
		return nil, err
	}
	o := &Outcome{}
	o.RequestID = readRequestID(r.Field("requestId"))
	o.Outcome = r.Field("outcome").Enum(Outcomes...)
	o.Replayed = r.Field("replayed").Bool()
	o.ResultingRevision = r.Field("resultingRevision").CountOrNull()
	o.ResultingAcceptanceRevision = r.Field("resultingAcceptanceRevision").CountOrNull()
	if value, ok := v.Obj.Get("releaseId"); ok {
		o.ReleaseID = wire.NewReader(value, "/releaseId").LabelOrNull()
	}
	if value, ok := v.Obj.Get("resultingReleaseRevision"); ok {
		o.ResultingReleaseRevision = wire.NewReader(value, "/resultingReleaseRevision").CountOrNull()
	}
	o.ReceiptSeq = r.Field("receiptSeq").SizeOrNull()
	o.Codes = nonNil(r.Field("codes").Strings(-1, false, func(c *wire.Reader) string {
		s := c.String()
		if c.Err() == nil && !wire.IsCode(s) {
			c.Fail(wire.CodeMalformed, "unknown code %q", s)
		}
		return s
	}))
	if err := r.Err(); err != nil {
		return nil, err
	}
	if o.Outcome == OutcomeCompleted && ((o.ResultingRevision == nil) != (o.ResultingAcceptanceRevision == nil)) {
		return nil, wire.Errorf(wire.CodeMalformed, "/resultingRevision", "COMPLETED carries both resulting revisions or neither")
	}
	if o.Outcome != OutcomeCompleted && (o.ResultingRevision != nil || o.ResultingAcceptanceRevision != nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/resultingRevision", "a refused outcome carries no resulting revision")
	}
	ticketShape := o.ResultingRevision != nil || o.ResultingAcceptanceRevision != nil
	releaseShape := o.ReleaseID != nil || o.ResultingReleaseRevision != nil
	if (o.ReleaseID == nil) != (o.ResultingReleaseRevision == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/releaseId", "release outcome carries both release identity fields or neither")
	}
	if ticketShape && releaseShape {
		return nil, wire.Errorf(wire.CodeMalformed, "/releaseId", "ticket and release result fields are mutually exclusive")
	}
	if o.Outcome != OutcomeCompleted && releaseShape {
		return nil, wire.Errorf(wire.CodeMalformed, "/releaseId", "a refused outcome carries no release result")
	}
	return o, nil
}

// HasCode reports whether the outcome carries the §11 code.
func (o *Outcome) HasCode(code string) bool {
	for _, c := range o.Codes {
		if c == code {
			return true
		}
	}
	return false
}

func sortedUnique(ss []string) []string {
	out := make([]string, 0, len(ss))
	seen := map[string]bool{}
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
