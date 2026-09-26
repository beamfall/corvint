// Package snapshot implements the TM-V0-008 read snapshot protocol over the
// private state directory of §3.4: read head.json, prove that the
// `<lastSeq+1>` and `<lastSeq+2>` receipt slots are absent, read, re-check,
// retry at most three times, else NOT_RUN/SNAPSHOT_MOVED. It also carries the
// read-only decoders for the head, barrier and receipt records that reads,
// `archive export` and `archive verify` need. It never writes, locks,
// creates, redoes or migrates anything. The journal's own writer-side
// implementation of these records belongs to TCP-02 (internal/journal);
// this package is a strict read view of the same frozen schemas.
package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profiles and fixed file contents (§3.4).
const (
	ProfileHead    = "taskman-journal-head/0"
	ProfileBarrier = "taskman-barrier/0"
	ProfileReceipt = "taskman-receipt/0"
	VersionBytes   = "taskman-state/0\n"
)

// Head is a validated taskman-journal-head/0.
type Head struct {
	QueueID           wire.QueueID
	LastSeq           wire.Size
	LastReceiptSha256 *wire.Digest
	Generation        wire.Size
	InitSha256        wire.Digest
	PrimaryWorktree   string
	VersionSha256     wire.Digest
}

// DecodeHead parses and validates head.json (≤4 KiB).
func DecodeHead(data []byte) (*Head, error) {
	if len(data) > wire.MaxJournalHeadBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "head.json larger than %d bytes", wire.MaxJournalHeadBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "lastSeq", "lastReceiptSha256", "generation", "initSha256", "primaryWorktree", "versionSha256")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileHead); err != nil {
		return nil, err
	}
	h := &Head{}
	h.QueueID = r.Field("queueId").QueueID()
	h.LastSeq = r.Field("lastSeq").Size()
	h.LastReceiptSha256 = r.Field("lastReceiptSha256").DigestOrNull()
	h.Generation = r.Field("generation").Size()
	h.InitSha256 = r.Field("initSha256").Digest()
	h.PrimaryWorktree = r.Field("primaryWorktree").PathText()
	h.VersionSha256 = r.Field("versionSha256").Digest()
	if err := r.Err(); err != nil {
		return nil, err
	}
	// §2: seq starts at 1 and the INIT receipt is seq 1, so a head always
	// names its last receipt.
	if h.LastSeq.Uint64() < 1 || h.LastReceiptSha256 == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/lastSeq", "head must name at least the INIT receipt (lastSeq ≥ 1 with lastReceiptSha256)")
	}
	return h, nil
}

// Value renders the head.
func (h *Head) Value() wire.Value {
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileHead))
	o.Set("queueId", wire.String(h.QueueID.Raw))
	o.Set("lastSeq", wire.String(string(h.LastSeq)))
	if h.LastReceiptSha256 == nil {
		o.Set("lastReceiptSha256", wire.Null())
	} else {
		o.Set("lastReceiptSha256", wire.String(string(*h.LastReceiptSha256)))
	}
	o.Set("generation", wire.String(string(h.Generation)))
	o.Set("initSha256", wire.String(string(h.InitSha256)))
	o.Set("primaryWorktree", wire.String(h.PrimaryWorktree))
	o.Set("versionSha256", wire.String(string(h.VersionSha256)))
	return wire.ObjectValue(o)
}

// Barrier is a validated taskman-barrier/0.
type Barrier struct {
	QueueID  wire.QueueID
	Scope    string
	Reason   string
	Actor    string
	SinceSeq wire.Size
	Since    wire.Timestamp
}

// DecodeBarrier parses and validates barrier.json (≤4 KiB).
func DecodeBarrier(data []byte) (*Barrier, error) {
	if len(data) > wire.MaxBarrierBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "barrier.json larger than %d bytes", wire.MaxBarrierBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "scope", "reason", "actor", "sinceSeq", "since")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileBarrier); err != nil {
		return nil, err
	}
	b := &Barrier{}
	b.QueueID = r.Field("queueId").QueueID()
	b.Scope = r.Field("scope").Enum("ADMISSION", "ALL")
	b.Reason = r.Field("reason").Enum("CUTOVER", "EMERGENCY", "DRAIN", "OPERATOR")
	b.Actor = r.Field("actor").Label()
	b.SinceSeq = r.Field("sinceSeq").Size()
	b.Since = r.Field("since").Timestamp()
	if err := r.Err(); err != nil {
		return nil, err
	}
	return b, nil
}

// ReceiptKinds is the closed receipt kind set (§3.4).
var ReceiptKinds = []string{
	"INIT", "MUTATION", "ADMIT", "TRANSITION", "EFFECT_INTENT", "EFFECT_OUTCOME", "GATE_RESULT", "REVIEW",
	"MANIFEST", "RELEASE", "PAUSE", "UNPAUSE", "DRAIN", "IMPORT_PLAN", "IMPORT_APPLY", "AUTHORITY_SWITCH",
	"ARCHIVE", "RESTORE", "PRUNE", "RECONCILE", "ADJUDICATE", "CONFIG_PIN", "QUALIFICATION", "POLICY_UPDATE",
}

// PostEntry is one receipt post entry.
type PostEntry struct {
	Path       string
	Sha256     *wire.Digest
	Record     *wire.Value
	BlobSha256 *wire.Digest
}

// PreEntry is one receipt pre entry.
type PreEntry struct {
	Path   string
	Sha256 *wire.Digest
}

// Receipt is the read view of a taskman-receipt/0: the chain fields plus the
// closed-schema check. Nothing here interprets a receipt's effect.
type Receipt struct {
	Seq              wire.Size
	Prev             *wire.Digest
	Kind             string
	RequestID        *string
	ActorID          string
	ActorRole        string
	TicketID         *wire.TicketID
	ReleaseID        *string
	AttemptID        *string
	Generation       *wire.Size
	ExpectedRevision *wire.Count
	HeadGeneration   wire.Size
	Pre              []PreEntry
	Post             []PostEntry
	Outcome          string
	Codes            []string
	RecordedAt       wire.Timestamp
}

// DecodeReceipt parses and validates one receipt file (≤1 MiB, at most 8
// inline post entries of ≤64 KiB each, exactly one of record/blobSha256,
// except the explicit all-null UNPAUSE barrier deletion).
func DecodeReceipt(data []byte) (*Receipt, error) {
	if len(data) > wire.MaxReceiptFileBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "receipt larger than %d bytes", wire.MaxReceiptFileBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	if _, ok := v.Obj.Get("releaseId"); !ok {
		v.Obj.Set("releaseId", wire.Null())
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "seq", "prev", "kind", "requestId", "actor", "ticketId", "releaseId", "attemptId", "generation",
		"expectedRevision", "headGeneration", "pre", "post", "outcome", "codes", "recordedAt")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileReceipt); err != nil {
		return nil, err
	}
	rc := &Receipt{}
	rc.Seq = r.Field("seq").Size()
	rc.Prev = r.Field("prev").DigestOrNull()
	rc.Kind = r.Field("kind").Enum(ReceiptKinds...)
	rc.RequestID = r.Field("requestId").StringOrNull(readRequestID)
	actor := r.Field("actor")
	actor.Closed("id", "role")
	rc.ActorID = actor.Field("id").Label()
	rc.ActorRole = actor.Field("role").Enum("OWNER", "OPERATOR", "WORKER", "REVIEWER", "IMPORTER", "SYSTEM")
	tk := r.Field("ticketId")
	if !tk.IsNull() {
		id := tk.TicketID()
		rc.TicketID = &id
	}
	if value, ok := v.Obj.Get("releaseId"); ok {
		rc.ReleaseID = wire.NewReader(value, "/releaseId").LabelOrNull()
	}
	if rc.TicketID != nil && rc.ReleaseID != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/releaseId", "ticketId and releaseId are mutually exclusive")
	}
	rc.AttemptID = r.Field("attemptId").StringOrNull((*wire.Reader).Identifier)
	rc.Generation = r.Field("generation").SizeOrNull()
	rc.ExpectedRevision = r.Field("expectedRevision").CountOrNull()
	rc.HeadGeneration = r.Field("headGeneration").Size()
	for _, p := range r.Field("pre").Array(-1, true) {
		p.Closed("path", "sha256")
		rc.Pre = append(rc.Pre, PreEntry{Path: p.Field("path").Identifier(), Sha256: p.Field("sha256").DigestOrNull()})
	}
	inline := 0
	for _, p := range r.Field("post").Array(-1, true) {
		p.Closed("path", "sha256", "record", "blobSha256")
		pe := PostEntry{}
		pe.Path = p.Field("path").Identifier()
		pe.Sha256 = p.Field("sha256").DigestOrNull()
		rec := p.Field("record")
		if !rec.IsNull() {
			rv := rec.Value()
			if rv.Kind != wire.KindObject {
				rec.Fail(wire.CodeMalformed, "record must be an object or null")
			}
			if len(wire.EncodeFile(rv)) > wire.MaxInlinePostEntryBytes {
				rec.Fail(wire.CodeLimitExceeded, "inline post entry larger than %d bytes", wire.MaxInlinePostEntryBytes)
			}
			inline++
			pe.Record = &rv
		}
		pe.BlobSha256 = p.Field("blobSha256").DigestOrNull()
		if p.Err() == nil && pe.Sha256 != nil && (pe.Record == nil) == (pe.BlobSha256 == nil) {
			p.Fail(wire.CodeMalformed, "exactly one of record and blobSha256 must be non-null")
		}
		rc.Post = append(rc.Post, pe)
	}
	if r.Err() == nil && inline > wire.MaxInlinePostEntries {
		r.Field("post").Fail(wire.CodeLimitExceeded, "more than %d inline post entries", wire.MaxInlinePostEntries)
	}
	rc.Outcome = r.Field("outcome").Enum(mutation.Outcomes...)
	rc.Codes = r.Field("codes").Strings(-1, false, func(c *wire.Reader) string {
		s := c.String()
		if c.Err() == nil && !wire.IsCode(s) {
			c.Fail(wire.CodeMalformed, "unknown code %q", s)
		}
		return s
	})
	rc.RecordedAt = r.Field("recordedAt").Timestamp()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if rc.Seq.Uint64() < 1 {
		return nil, wire.Errorf(wire.CodeMalformed, "/seq", "seq starts at 1")
	}
	if (rc.Seq == "1") != (rc.Prev == nil) {
		return nil, wire.Errorf(wire.CodeMalformed, "/prev", "prev is null iff seq is 1")
	}
	if err := validateEntries(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

// ReceiptName renders `<seq as 12 decimal digits>.json` (§2). Sequence
// numbers above twelve digits cannot exist below journal saturation.
func ReceiptName(seq uint64) (string, error) {
	if seq > 999999999999 {
		return "", wire.Errorf(wire.CodeMalformed, "receipts", "seq %d exceeds twelve digits", seq)
	}
	s := wire.SizeOf(seq)
	pad := ""
	for i := len(s); i < 12; i++ {
		pad += "0"
	}
	return pad + string(s) + ".json", nil
}
