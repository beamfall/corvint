package wire

import "sort"

// ProfileCommandResult is the stdout envelope of every corvint-tasks command (§3.3).
const ProfileCommandResult = "taskman-command-result/0"

// Outcomes of taskman-command-result/0.
const (
	OutcomeOK      = "OK"
	OutcomeRefused = "REFUSED"
	OutcomeError   = "ERROR"
	OutcomeNotRun  = "NOT_RUN"
)

// UntrustedQueueData is the only legal member of the `untrusted` array.
const UntrustedQueueData = "UNTRUSTED_QUEUE_DATA"

// BarrierRef is the `snapshot.barrier` object.
type BarrierRef struct {
	Scope  string
	Reason string
}

// Snapshot is the `snapshot` object of the envelope.
type Snapshot struct {
	HeadSeq               *Size
	HeadReceiptSha256     *Digest
	IntentTreeSha256      *Digest
	PrimaryWorktreeSha256 *Digest
	PendingRedo           bool
	Barrier               *BarrierRef
}

// Page is the pagination object.
type Page struct {
	Offset    Count
	Limit     Count
	Total     *Count
	Truncated bool
}

// Result is a taskman-command-result/0 document.
type Result struct {
	Command   []string
	Outcome   string
	Codes     []string
	Snapshot  *Snapshot
	Mutation  *Value // taskman-outcome/0 or nil (null)
	Items     []Value
	Page      *Page
	Untrusted bool
	Warnings  []string
}

func sizeOrNull(p *Size) Value {
	if p == nil {
		return Null()
	}
	return String(string(*p))
}

func digestOrNull(p *Digest) Value {
	if p == nil {
		return Null()
	}
	return String(string(*p))
}

func countOrNull(p *Count) Value {
	if p == nil {
		return Null()
	}
	return String(string(*p))
}

func sortedUniqueStrings(ss []string) []string {
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

// Value renders the envelope. Codes and warnings are non-semantic arrays and
// are sorted; items keep their semantic order.
func (r *Result) Value() Value {
	o := NewObject()
	o.Set("profile", String(ProfileCommandResult))
	o.Set("command", Strings(r.Command))
	o.Set("outcome", String(r.Outcome))
	o.Set("codes", Strings(sortedUniqueStrings(r.Codes)))
	if r.Snapshot == nil {
		o.Set("snapshot", Null())
	} else {
		s := NewObject()
		s.Set("headSeq", sizeOrNull(r.Snapshot.HeadSeq))
		s.Set("headReceiptSha256", digestOrNull(r.Snapshot.HeadReceiptSha256))
		s.Set("intentTreeSha256", digestOrNull(r.Snapshot.IntentTreeSha256))
		s.Set("primaryWorktreeSha256", digestOrNull(r.Snapshot.PrimaryWorktreeSha256))
		s.Set("pendingRedo", Bool(r.Snapshot.PendingRedo))
		if r.Snapshot.Barrier == nil {
			s.Set("barrier", Null())
		} else {
			b := NewObject()
			b.Set("scope", String(r.Snapshot.Barrier.Scope))
			b.Set("reason", String(r.Snapshot.Barrier.Reason))
			s.Set("barrier", ObjectValue(b))
		}
		o.Set("snapshot", ObjectValue(s))
	}
	if r.Mutation == nil {
		o.Set("mutation", Null())
	} else {
		o.Set("mutation", *r.Mutation)
	}
	items := r.Items
	if items == nil {
		items = []Value{}
	}
	o.Set("items", Array(items...))
	if r.Page == nil {
		o.Set("page", Null())
	} else {
		p := NewObject()
		p.Set("offset", String(string(r.Page.Offset)))
		p.Set("limit", String(string(r.Page.Limit)))
		p.Set("total", countOrNull(r.Page.Total))
		p.Set("truncated", Bool(r.Page.Truncated))
		o.Set("page", ObjectValue(p))
	}
	if r.Untrusted {
		o.Set("untrusted", Strings([]string{UntrustedQueueData}))
	} else {
		o.Set("untrusted", Strings(nil))
	}
	o.Set("warnings", Strings(sortedUniqueStrings(r.Warnings)))
	return ObjectValue(o)
}

// Encode validates the envelope against its closed schema and the §1 size
// bounds and returns the on-disk/transport bytes (canonical body plus LF).
func (r *Result) Encode() ([]byte, error) {
	v := r.Value()
	if _, err := DecodeResult(EncodeFile(v)); err != nil {
		return nil, err
	}
	itemBytes := 0
	for _, it := range r.Items {
		itemBytes += len(Encode(it)) + 1
	}
	if itemBytes > MaxListResultBytes {
		return nil, Errorf(CodeLimitExceeded, "/items", "list result larger than %d bytes", MaxListResultBytes)
	}
	full := EncodeFile(v)
	if len(full)-itemBytes > MaxCommandResultBytes {
		return nil, Errorf(CodeLimitExceeded, "/", "command result envelope larger than %d bytes", MaxCommandResultBytes)
	}
	return full, nil
}

// DecodeResult parses and validates an envelope. Items are kept as opaque
// values (their kind is verb-specific).
func DecodeResult(data []byte) (*Result, error) {
	v, err := Parse(data)
	if err != nil {
		return nil, err
	}
	rd := NewReader(v, "/")
	rd.Closed("profile", "command", "outcome", "codes", "snapshot", "mutation", "items", "page", "untrusted", "warnings")
	if err := rd.Err(); err != nil {
		return nil, err
	}
	res := &Result{}
	rd.adopt(CheckProfile("/profile", rd.Field("profile").String(), ProfileCommandResult))
	res.Command = rd.Field("command").Strings(-1, true, (*Reader).Identifier)
	res.Outcome = rd.Field("outcome").Enum(OutcomeOK, OutcomeRefused, OutcomeError, OutcomeNotRun)
	res.Codes = rd.Field("codes").Strings(-1, false, func(c *Reader) string {
		s := c.String()
		if c.st.err == nil && !IsCode(s) {
			c.Fail(CodeMalformed, "unknown code %q", s)
		}
		return s
	})
	sn := rd.Field("snapshot")
	if !sn.IsNull() {
		sn.Closed("headSeq", "headReceiptSha256", "intentTreeSha256", "primaryWorktreeSha256", "pendingRedo", "barrier")
		s := &Snapshot{}
		s.HeadSeq = sn.Field("headSeq").SizeOrNull()
		s.HeadReceiptSha256 = sn.Field("headReceiptSha256").DigestOrNull()
		s.IntentTreeSha256 = sn.Field("intentTreeSha256").DigestOrNull()
		s.PrimaryWorktreeSha256 = sn.Field("primaryWorktreeSha256").DigestOrNull()
		s.PendingRedo = sn.Field("pendingRedo").Bool()
		b := sn.Field("barrier")
		if !b.IsNull() {
			b.Closed("scope", "reason")
			s.Barrier = &BarrierRef{
				Scope:  b.Field("scope").Enum("ADMISSION", "ALL"),
				Reason: b.Field("reason").Enum("CUTOVER", "EMERGENCY", "DRAIN", "OPERATOR"),
			}
		}
		res.Snapshot = s
	}
	m := rd.Field("mutation")
	if !m.IsNull() {
		mv := m.Value()
		if mv.Kind != KindObject {
			m.Fail(CodeMalformed, "mutation must be an object or null")
		}
		res.Mutation = &mv
	}
	for _, it := range rd.Field("items").Array(-1, true) {
		if it.Value().Kind != KindObject {
			it.Fail(CodeMalformed, "items must be objects")
		}
		res.Items = append(res.Items, it.Value())
	}
	pg := rd.Field("page")
	if !pg.IsNull() {
		pg.Closed("offset", "limit", "total", "truncated")
		p := &Page{}
		p.Offset = pg.Field("offset").Count()
		p.Limit = pg.Field("limit").Count()
		p.Total = pg.Field("total").CountOrNull()
		p.Truncated = pg.Field("truncated").Bool()
		res.Page = p
	}
	ut := rd.Field("untrusted").Strings(1, false, func(c *Reader) string { return c.Exact(UntrustedQueueData) })
	res.Untrusted = len(ut) == 1
	res.Warnings = rd.Field("warnings").Strings(-1, false, func(c *Reader) string { return c.Prose(1, MaxProseBytes) })
	if err := rd.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
