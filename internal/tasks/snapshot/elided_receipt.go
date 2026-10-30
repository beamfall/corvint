package snapshot

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfileElidedReceipt is a retained receipt whose superseded liveness
// attempt record is elided (CAL-V0-300, CAL-V0-303, decision 0538). It is the
// taskman-receipt/0 it replaces under the same name with three changes: the
// profile, the elided attempt post's record set to null, and two added
// members: receiptSha256, the digest of the original bytes that the next
// receipt's prev names, and elided, the attempt path with the two liveness
// members (lastHeartbeatAt and lease) the original record posted. Every other
// member is kept, the request afterimage included. A build that reads only
// taskman-receipt/0 refuses it as UNSUPPORTED_VERSION (CAL-V0-131) instead of
// misreading it.
const ProfileElidedReceipt = "taskman-receipt/1"

// livenessKeys are the attempt record members a liveness write changes.
var livenessKeys = []string{"lastHeartbeatAt", "lease"}

// elidedForm is one parsed taskman-receipt/1.
type elidedForm struct {
	stripped  *wire.Object // the tombstone without receiptSha256 and elided
	digest    wire.Digest
	path      string
	heartbeat wire.Value
	lease     wire.Value
}

func parseReceiptFile(data []byte) (wire.Value, error) {
	if len(data) > wire.MaxReceiptFileBytes {
		return wire.Value{}, wire.Errorf(wire.CodeLimitExceeded, "/", "receipt larger than %d bytes", wire.MaxReceiptFileBytes)
	}
	return wire.Parse(data)
}

func profileOf(v wire.Value) string {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return ""
	}
	p, ok := v.Obj.Get("profile")
	if !ok || p.Kind != wire.KindString {
		return ""
	}
	return p.Str
}

// copyObject is a shallow copy: members are shared, the member set is not.
func copyObject(o *wire.Object) *wire.Object {
	out := wire.NewObject()
	for _, k := range o.Keys {
		out.Set(k, o.Vals[k])
	}
	return out
}

func withoutMembers(o *wire.Object, drop ...string) *wire.Object {
	out := wire.NewObject()
	for _, k := range o.Keys {
		keep := true
		for _, d := range drop {
			keep = keep && k != d
		}
		if keep {
			out.Set(k, o.Vals[k])
		}
	}
	return out
}

func parseElided(v wire.Value) (*elidedForm, *Receipt, error) {
	r := wire.NewReader(v, "/")
	sum := r.Field("receiptSha256").Digest()
	el := r.Field("elided")
	el.Closed("path", "lastHeartbeatAt", "lease")
	path := el.Field("path").Identifier()
	el.Field("lastHeartbeatAt").Timestamp()
	lease := readLease(el.Field("lease"))
	if err := r.Err(); err != nil {
		return nil, nil, err
	}
	if lease == nil {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/elided/lease", "an elided liveness record holds a lease")
	}
	f := &elidedForm{stripped: withoutMembers(v.Obj, "receiptSha256", "elided"), digest: sum, path: path}
	f.heartbeat, _ = el.Value().Obj.Get("lastHeartbeatAt")
	f.lease, _ = el.Value().Obj.Get("lease")
	rc, err := decodeReceiptValue(wire.ObjectValue(copyObject(f.stripped)), ProfileElidedReceipt, path)
	if err != nil {
		return nil, nil, err
	}
	if rc.Kind != "TRANSITION" || rc.Outcome != mutation.OutcomeCompleted || len(rc.Codes) != 0 || rc.Prev == nil || rc.RequestID == nil || rc.AttemptID == nil || rc.Generation == nil || path != "attempts/"+*rc.AttemptID+".json" {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/elided", "only a completed liveness transition of its attempt may be elided")
	}
	omitted := 0
	for _, p := range rc.Post {
		if p.Path == path && p.Sha256 != nil && p.Record == nil && p.BlobSha256 == nil {
			omitted++
		}
	}
	if omitted != 1 {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "/elided/path", "the elided path must be the one post whose record is omitted")
	}
	return f, rc, nil
}

// DecodeRetainedReceipt decodes one retained receipt file of either profile.
// For a taskman-receipt/0 it is DecodeReceipt and the file's digest. For a
// taskman-receipt/1 it returns the receipt as retained, with the elided
// attempt post's record and blob both nil, and the recorded digest of the
// original, which only the next receipt's prev and a ReceiptRestorer
// replay from receipt 1 verify. Readers that need only a receipt's header,
// chain link or non-attempt posts use it; readers that need content use a
// ReceiptRestorer.
func DecodeRetainedReceipt(data []byte) (rc *Receipt, digest wire.Digest, elided bool, err error) {
	v, err := parseReceiptFile(data)
	if err != nil {
		return nil, "", false, err
	}
	if profileOf(v) != ProfileElidedReceipt {
		rc, err := decodeReceiptValue(v, ProfileReceipt, "")
		if err != nil {
			return nil, "", false, err
		}
		return rc, wire.Sum(data), false, nil
	}
	_, rc, err = parseElided(v)
	if err != nil {
		return nil, "", false, err
	}
	return rc, mustDigest(v), true, nil
}

func mustDigest(v wire.Value) wire.Digest {
	d, _ := v.Obj.Get("receiptSha256")
	return wire.Digest(d.Str)
}

// restore rebuilds the original receipt bytes from base, the attempt record
// the elided post replaced, and proves them: base must hash to the receipt's
// pre digest, the rebuilt record to its post digest and the rebuilt receipt
// to the recorded original digest.
func (f *elidedForm) restore(rc *Receipt, base []byte) ([]byte, error) {
	var pre, post *wire.Digest
	index := -1
	for i, p := range rc.Post {
		if p.Path == f.path {
			pre, post, index = rc.Pre[i].Sha256, p.Sha256, i
		}
	}
	if base == nil || pre == nil || wire.Sum(base) != *pre {
		return nil, wire.Errorf(wire.CodeJournalForked, f.path, "elided receipt %s: the retained predecessor record differs from its pre digest", rc.Seq)
	}
	bv, err := wire.Parse(base)
	if err != nil || bv.Kind != wire.KindObject {
		return nil, wire.Errorf(wire.CodeJournalForked, f.path, "elided receipt %s: the retained predecessor record is not an object", rc.Seq)
	}
	record := copyObject(bv.Obj)
	record.Set("lastHeartbeatAt", f.heartbeat)
	record.Set("lease", f.lease)
	if wire.Sum(wire.EncodeFile(wire.ObjectValue(record))) != *post {
		return nil, wire.Errorf(wire.CodeJournalForked, f.path, "elided receipt %s: the restored record differs from its post digest", rc.Seq)
	}
	orig := copyObject(f.stripped)
	orig.Set("profile", wire.String(ProfileReceipt))
	posts := orig.Vals["post"].Arr
	restored := make([]wire.Value, len(posts))
	copy(restored, posts)
	entry := copyObject(posts[index].Obj)
	entry.Set("record", wire.ObjectValue(record))
	restored[index] = wire.ObjectValue(entry)
	orig.Set("post", wire.Array(restored...))
	raw := wire.EncodeFile(wire.ObjectValue(orig))
	if wire.Sum(raw) != f.digest {
		return nil, wire.Errorf(wire.CodeJournalForked, "/receiptSha256", "elided receipt %s: the restored receipt differs from its recorded digest", rc.Seq)
	}
	return raw, nil
}

// MaxElidedRun bounds how far back the latest whole record of an attempt may
// lie before one of its elided receipts: liveness compaction keeps a
// superseded receipt whole rather than exceed it (CAL-V0-302), so a reader
// that restores by reading backward, such as doctor, needs at most this many
// receipts.
const MaxElidedRun = 1024

// ReceiptRestorer replays retained receipts in sequence order from receipt
// 1 and returns each one's original taskman-receipt/0 bytes: a
// taskman-receipt/0 as read, a taskman-receipt/1 rebuilt from the latest
// retained record of its attempt and proven against its recorded digest
// (CAL-V0-301). It retains the latest inline record of each attempt that
// holds a lease, the only records an elided receipt can follow. The zero
// value is ready to use.
type ReceiptRestorer struct {
	bases map[string][]byte
	// open maps each attempt path whose latest post so far is an elided
	// receipt to that receipt's sequence number.
	open map[string]uint64
	// Elided counts the taskman-receipt/1 files restored so far.
	Elided uint64
}

// Next decodes the next receipt in sequence and returns it with its
// original bytes.
func (r *ReceiptRestorer) Next(raw []byte) (*Receipt, []byte, error) {
	v, err := parseReceiptFile(raw)
	if err != nil {
		return nil, nil, err
	}
	original, elidedPath := raw, ""
	if profileOf(v) == ProfileElidedReceipt {
		f, header, err := parseElided(v)
		if err != nil {
			return nil, nil, err
		}
		if original, err = f.restore(header, r.bases[f.path]); err != nil {
			return nil, nil, err
		}
		if v, err = wire.Parse(original); err != nil {
			return nil, nil, err
		}
		r.Elided++
		elidedPath = f.path
	}
	rc, err := decodeReceiptValue(v, ProfileReceipt, "")
	if err != nil {
		return nil, nil, err
	}
	r.observe(rc)
	if elidedPath != "" {
		if r.open == nil {
			r.open = map[string]uint64{}
		}
		r.open[elidedPath] = rc.Seq.Uint64()
	}
	return rc, original, nil
}

// WholeReceipt decodes raw exactly as Next does when raw is a whole
// taskman-receipt/0 file, without touching any restorer state, and returns
// nil with no error for an elided receipt, which only Next can restore. A
// walk may decode ahead with it and then Accept the receipt in sequence
// (V1-1701).
func WholeReceipt(raw []byte) (*Receipt, error) {
	v, err := parseReceiptFile(raw)
	if err != nil {
		return nil, err
	}
	if profileOf(v) == ProfileElidedReceipt {
		return nil, nil
	}
	return decodeReceiptValue(v, ProfileReceipt, "")
}

// Accept folds in a whole receipt that WholeReceipt decoded, in sequence:
// the state Next would have kept had it decoded the same bytes.
func (r *ReceiptRestorer) Accept(rc *Receipt) {
	r.observe(rc)
}

// Superseded refuses a walk that reached the head with an elided receipt
// still the latest post of its attempt. CAL-V0-300 elides only superseded
// receipts, so the head receipt and the latest record of every attempt are
// always whole; a store that breaks this is JOURNAL_FORKED.
func (r *ReceiptRestorer) Superseded() error {
	if r == nil {
		return nil
	}
	path, first := "", uint64(0)
	for p, seq := range r.open {
		if first == 0 || seq < first {
			path, first = p, seq
		}
	}
	if first == 0 {
		return nil
	}
	return wire.Errorf(wire.CodeJournalForked, path, "elided receipt %d is the latest post of its attempt; only a superseded liveness receipt may be elided", first)
}

// Base returns the latest retained record of an attempt path, nil when none
// is retained.
func (r *ReceiptRestorer) Base(path string) []byte { return r.bases[path] }

func (r *ReceiptRestorer) observe(rc *Receipt) {
	for _, p := range rc.Post {
		if !strings.HasPrefix(p.Path, "attempts/") {
			continue
		}
		delete(r.open, p.Path)
		if p.Record == nil || p.Record.Kind != wire.KindObject {
			delete(r.bases, p.Path)
			continue
		}
		if lease, ok := p.Record.Obj.Get("lease"); !ok || lease.Kind != wire.KindObject {
			delete(r.bases, p.Path)
			continue
		}
		if r.bases == nil {
			r.bases = map[string][]byte{}
		}
		r.bases[p.Path] = wire.EncodeFile(*p.Record)
	}
}

// ElideLiveness returns the taskman-receipt/1 form of original when it is an
// elidable liveness receipt (CAL-V0-300): a completed TRANSITION with no
// codes that posts exactly its attempt record inline and its request
// afterimage, where the record differs from base, the attempt's previous
// record, only in lastHeartbeatAt and in the lease's grantedSeq and
// expiresAt, with the holder unchanged. Anything else, an ownership,
// generation, phase or outcome change among them, reports ok false. Whether
// a later receipt supersedes it is the caller's decision. The tombstone is
// proven to restore to original before it is returned.
func ElideLiveness(original, base []byte) (tombstone []byte, ok bool, err error) {
	rc, err := DecodeReceipt(original)
	if err != nil {
		return nil, false, err
	}
	if rc.Kind != "TRANSITION" || rc.Outcome != mutation.OutcomeCompleted || len(rc.Codes) != 0 || rc.Prev == nil || rc.RequestID == nil || rc.AttemptID == nil || rc.Generation == nil || len(rc.Post) != 2 || base == nil {
		return nil, false, nil
	}
	path := "attempts/" + *rc.AttemptID + ".json"
	request, err := RequestPath(*rc.RequestID)
	if err != nil {
		return nil, false, nil
	}
	index := -1
	for i, p := range rc.Post {
		switch {
		case p.Path == path && p.Record != nil && p.Sha256 != nil && rc.Pre[i].Sha256 != nil && *rc.Pre[i].Sha256 == wire.Sum(base):
			index = i
		case p.Path == request && p.Record != nil:
		default:
			return nil, false, nil
		}
	}
	if index < 0 {
		return nil, false, nil
	}
	bv, err := wire.Parse(base)
	if err != nil || bv.Kind != wire.KindObject || !livenessOnly(bv.Obj, rc.Post[index].Record.Obj) {
		return nil, false, nil
	}
	v, err := wire.Parse(original)
	if err != nil {
		return nil, false, err
	}
	t := copyObject(v.Obj)
	t.Set("profile", wire.String(ProfileElidedReceipt))
	posts := make([]wire.Value, len(v.Obj.Vals["post"].Arr))
	copy(posts, v.Obj.Vals["post"].Arr)
	entry := copyObject(posts[index].Obj)
	entry.Set("record", wire.Null())
	posts[index] = wire.ObjectValue(entry)
	t.Set("post", wire.Array(posts...))
	t.Set("receiptSha256", wire.String(string(wire.Sum(original))))
	el := wire.NewObject()
	el.Set("path", wire.String(path))
	post := rc.Post[index].Record.Obj
	el.Set("lastHeartbeatAt", post.Vals["lastHeartbeatAt"])
	el.Set("lease", post.Vals["lease"])
	t.Set("elided", wire.ObjectValue(el))
	tombstone = wire.EncodeFile(wire.ObjectValue(t))
	restorer := &ReceiptRestorer{bases: map[string][]byte{path: base}}
	if _, back, err := restorer.Next(tombstone); err != nil || string(back) != string(original) {
		return nil, false, wire.Errorf(wire.CodeJournalForked, path, "elided receipt %s does not restore to its original bytes", rc.Seq)
	}
	return tombstone, true, nil
}

// livenessOnly reports whether post differs from pre only in the liveness
// members: lastHeartbeatAt set, and a lease held by the same holder before
// and after.
func livenessOnly(pre, post *wire.Object) bool {
	hb, ok := post.Get("lastHeartbeatAt")
	if !ok || hb.Kind != wire.KindString {
		return false
	}
	before, ok1 := pre.Get("lease")
	after, ok2 := post.Get("lease")
	if !ok1 || !ok2 || before.Kind != wire.KindObject || after.Kind != wire.KindObject {
		return false
	}
	h1, _ := before.Obj.Get("holder")
	h2, _ := after.Obj.Get("holder")
	if h1.Kind != wire.KindString || !wire.Equal(h1, h2) {
		return false
	}
	liveness := func(k string) bool { return k == livenessKeys[0] || k == livenessKeys[1] }
	for _, k := range post.Keys {
		if liveness(k) {
			continue
		}
		v, ok := pre.Get(k)
		if !ok || !wire.Equal(v, post.Vals[k]) {
			return false
		}
	}
	for _, k := range pre.Keys {
		if _, ok := post.Get(k); !ok {
			return false
		}
	}
	return true
}

// ElidedRecord restores the attempt record an elided liveness receipt omits
// from ancestor, the latest record of the same attempt a taskman-receipt/0
// retains before it. A run of elided receipts changes only the liveness
// members, so the omitted record is ancestor with the recorded liveness
// members; the result is proven against the receipt's post digest. It
// serves bounded backward readers that cannot replay from receipt 1.
func ElidedRecord(tombstone, ancestor []byte) ([]byte, error) {
	v, err := parseReceiptFile(tombstone)
	if err != nil {
		return nil, err
	}
	if profileOf(v) != ProfileElidedReceipt {
		return nil, wire.Errorf(wire.CodeMalformed, "/profile", "not an elided receipt")
	}
	f, rc, err := parseElided(v)
	if err != nil {
		return nil, err
	}
	av, err := wire.Parse(ancestor)
	if err != nil || av.Kind != wire.KindObject {
		return nil, wire.Errorf(wire.CodeJournalForked, f.path, "elided receipt %s: the ancestor record is not an object", rc.Seq)
	}
	record := copyObject(av.Obj)
	record.Set("lastHeartbeatAt", f.heartbeat)
	record.Set("lease", f.lease)
	raw := wire.EncodeFile(wire.ObjectValue(record))
	for _, p := range rc.Post {
		if p.Path == f.path && p.Sha256 != nil && wire.Sum(raw) == *p.Sha256 {
			return raw, nil
		}
	}
	return nil, wire.Errorf(wire.CodeJournalForked, f.path, "elided receipt %s: the restored record differs from its post digest", rc.Seq)
}
