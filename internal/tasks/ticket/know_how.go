package ticket

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Know-how ledger operations (KHN-V0-002, KHN-V0-018). An ADD records one
// note, optionally superseding an earlier active note on the same home
// ticket; a RETRACT withdraws one; a RECONFIRM re-pins an active note's
// anchors to a later commit without changing its text or the active set.
// Entries are never rewritten or removed, so earlier pins stay in history.
const (
	KnowHowAdd       = "ADD"
	KnowHowRetract   = "RETRACT"
	KnowHowReconfirm = "RECONFIRM"
)

var knowHowAddKeys = []string{"actor", "anchors", "attempt", "commit", "evidencePath", "generation", "operation", "reason", "recordedAt", "routes", "seq", "supersedes", "text"}
var knowHowRetractKeys = []string{"actor", "note", "operation", "reason", "recordedAt", "seq"}
var knowHowReconfirmKeys = []string{"actor", "anchors", "attempt", "commit", "generation", "note", "operation", "recordedAt", "seq"}

// KnowHowAnchor pins one repository-relative file to the Git blob it had at
// the writer's commit. A symbol anchor (KHN-V0-016) also names one
// declaration in that file and pins the SHA-256 of its extent's text, so an
// edit elsewhere in the file leaves it fresh. Symbol and SymbolSha256 are
// both empty for a file anchor, whose encoding is unchanged.
type KnowHowAnchor struct {
	Path         string
	Blob         string
	Symbol       string
	SymbolSha256 string
}

// KnowHowAnchorLess orders anchors by path, a file anchor before the symbol
// anchors of the same path, then by symbol.
func KnowHowAnchorLess(a, b KnowHowAnchor) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Symbol < b.Symbol
}

// KnowHowEntry is one append-only ledger entry of the optional knowHow record
// member. Seq is its 1-based position. ADD carries the note: Text, Anchors,
// Routes, Commit, Supersedes and the writer-asserted provenance (Attempt,
// Generation, EvidencePath). RETRACT names the withdrawn note in Note.
// RECONFIRM names the re-pinned note in Note and carries its new Anchors,
// Commit, Attempt and Generation. Reason is required by RETRACT and by a
// superseding ADD and is nil otherwise. Actor and RecordedAt come from the
// writer's trusted binding and clock.
type KnowHowEntry struct {
	Seq          wire.Count
	Operation    string
	Text         string
	Anchors      []KnowHowAnchor
	Routes       []string
	Commit       string
	Supersedes   *wire.Count
	Attempt      *string
	Generation   *wire.Size
	EvidencePath *string
	Note         wire.Count
	Reason       *string
	ActorID      string
	ActorRole    string
	RecordedAt   wire.Timestamp
	// Reconfirmed is the latest RECONFIRM of an active note as projected by
	// EffectiveKnowHow; it is never encoded in the ledger.
	Reconfirmed *KnowHowEntry
}

// ReadKnowHow decodes the optional knowHow member: a non-empty array in
// append order (absence is the empty ledger) of closed ADD, RETRACT or
// RECONFIRM entries.
func ReadKnowHow(r *wire.Reader) []KnowHowEntry {
	entries := r.Array(wire.KnowHowMaxEntries, true)
	if r.Err() == nil && len(entries) == 0 {
		r.Fail(wire.CodeMalformed, "knowHow is omitted when empty")
		return nil
	}
	out := make([]KnowHowEntry, 0, len(entries))
	for _, e := range entries {
		k := KnowHowEntry{Operation: e.Field("operation").Enum(KnowHowAdd, KnowHowRetract, KnowHowReconfirm)}
		if r.Err() != nil {
			return nil
		}
		switch k.Operation {
		case KnowHowAdd:
			e.Closed(knowHowAddKeys...)
			k.Text = ReadKnowHowText(e.Field("text"))
			k.Anchors = ReadKnowHowAnchors(e.Field("anchors"))
			k.Routes = ReadKnowHowRoutes(e.Field("routes"))
			k.Commit = e.Field("commit").OID()
			k.Supersedes = e.Field("supersedes").CountOrNull()
			k.Attempt = e.Field("attempt").StringOrNull((*wire.Reader).Identifier)
			k.Generation = e.Field("generation").SizeOrNull()
			k.EvidencePath = e.Field("evidencePath").StringOrNull(ReadKnowHowFile)
			if e.Field("reason").IsNull() {
				k.Reason = nil
			} else {
				s := ReadKnowHowReason(e.Field("reason"))
				k.Reason = &s
			}
		case KnowHowRetract:
			e.Closed(knowHowRetractKeys...)
			k.Note = e.Field("note").Count()
			s := ReadKnowHowReason(e.Field("reason"))
			k.Reason = &s
		default:
			e.Closed(knowHowReconfirmKeys...)
			k.Note = e.Field("note").Count()
			k.Anchors = ReadKnowHowAnchors(e.Field("anchors"))
			k.Commit = e.Field("commit").OID()
			k.Attempt = e.Field("attempt").StringOrNull((*wire.Reader).Identifier)
			k.Generation = e.Field("generation").SizeOrNull()
		}
		k.Seq = e.Field("seq").Count()
		a := e.Field("actor")
		a.Closed("id", "role")
		k.ActorID = a.Field("id").Label()
		k.ActorRole = a.Field("role").Enum("OWNER", "OPERATOR", "WORKER")
		k.RecordedAt = e.Field("recordedAt").Timestamp()
		out = append(out, k)
	}
	return out
}

// ReadKnowHowText reads nonblank prose of at most KnowHowMaxTextBytes.
func ReadKnowHowText(r *wire.Reader) string {
	s := r.Prose(1, wire.KnowHowMaxTextBytes)
	if r.Err() == nil && strings.TrimSpace(s) == "" {
		r.Fail(wire.CodeMalformed, "text must contain non-whitespace prose")
		return ""
	}
	return s
}

// ReadKnowHowReason reads nonblank prose of at most KnowHowMaxReasonBytes.
func ReadKnowHowReason(r *wire.Reader) string {
	s := r.Prose(1, wire.KnowHowMaxReasonBytes)
	if r.Err() == nil && strings.TrimSpace(s) == "" {
		r.Fail(wire.CodeMalformed, "reason must contain non-whitespace prose")
		return ""
	}
	return s
}

// ReadKnowHowAnchors reads 1..KnowHowMaxAnchors closed {blob, path} file
// anchors or {blob, path, symbol, symbolSha256} symbol anchors (KHN-V0-016)
// in strictly ascending (path, symbol) order. A path is a repository-relative
// file: the Path grammar refuses absolute and '..' paths, and a trailing '/'
// directory prefix is refused here. A symbol anchor carries both symbol keys
// or neither, so a file anchor keeps its earlier bytes, and anchors of one
// path pin one blob.
func ReadKnowHowAnchors(r *wire.Reader) []KnowHowAnchor {
	items := r.Array(wire.KnowHowMaxAnchors, true)
	if r.Err() == nil && len(items) == 0 {
		r.Fail(wire.CodeMalformed, "a note names at least one path anchor")
		return nil
	}
	out := make([]KnowHowAnchor, 0, len(items))
	for i, it := range items {
		symbol := wire.Has(it.Value(), "symbol") || wire.Has(it.Value(), "symbolSha256")
		if symbol {
			it.Closed("blob", "path", "symbol", "symbolSha256")
		} else {
			it.Closed("blob", "path")
		}
		a := KnowHowAnchor{Path: ReadKnowHowFile(it.Field("path")), Blob: it.Field("blob").OID()}
		if symbol {
			a.Symbol = ReadKnowHowSymbol(it.Field("symbol"))
			a.SymbolSha256 = string(it.Field("symbolSha256").Digest())
		}
		if r.Err() != nil {
			return nil
		}
		if i > 0 && !KnowHowAnchorLess(out[i-1], a) {
			r.Fail(wire.CodeMalformed, "anchors are sorted by path and symbol without duplicates")
			return nil
		}
		if i > 0 && out[i-1].Path == a.Path && out[i-1].Blob != a.Blob {
			r.Fail(wire.CodeMalformed, "anchors of path %q pin different blobs", a.Path)
			return nil
		}
		out = append(out, a)
	}
	return out
}

// ReadKnowHowSymbol reads a symbol anchor name (wire.ParseKnowHowSymbol).
func ReadKnowHowSymbol(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	if _, err := wire.ParseKnowHowSymbol(r.Where(), s); err != nil {
		r.Fail(wire.CodeOf(err), "symbol %q must be 1..%d printable ASCII bytes without space or '#'", s, wire.KnowHowMaxSymbolBytes)
		return ""
	}
	return s
}

// ReadKnowHowFile reads a Path naming a file, not a directory prefix.
func ReadKnowHowFile(r *wire.Reader) string {
	s := r.Path()
	if r.Err() == nil && strings.HasSuffix(s, "/") {
		r.Fail(wire.CodeMalformed, "path %q names a directory prefix, not a file", s)
		return ""
	}
	return s
}

// ReadKnowHowRoutes reads at most KnowHowMaxRoutes sorted unique opaque
// route or flow tokens of at most KnowHowMaxRouteBytes.
func ReadKnowHowRoutes(r *wire.Reader) []string {
	return nonNilStrings(r.Strings(wire.KnowHowMaxRoutes, false, func(x *wire.Reader) string {
		s := x.String()
		if x.Err() != nil {
			return ""
		}
		if _, err := wire.ParseToken(x.Where(), s, wire.KnowHowMaxRouteBytes); err != nil {
			x.Fail(wire.CodeOf(err), "route %q is not a token of at most %d bytes", s, wire.KnowHowMaxRouteBytes)
			return ""
		}
		return s
	}))
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// KnowHowAnchorsValue encodes anchors in (path, symbol) order; a file
// anchor omits the symbol keys.
func KnowHowAnchorsValue(anchors []KnowHowAnchor) wire.Value {
	vs := make([]wire.Value, 0, len(anchors))
	for _, a := range anchors {
		o := wire.NewObject().Set("blob", wire.String(a.Blob)).Set("path", wire.String(a.Path))
		if a.Symbol != "" {
			o.Set("symbol", wire.String(a.Symbol)).Set("symbolSha256", wire.String(a.SymbolSha256))
		}
		vs = append(vs, wire.ObjectValue(o))
	}
	return wire.Array(vs...)
}

func stringOrNull(s *string) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(*s)
}

func sizeOrNull(s *wire.Size) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(string(*s))
}

// KnowHowValue encodes the ledger in append order.
func KnowHowValue(entries []KnowHowEntry) wire.Value {
	vs := make([]wire.Value, 0, len(entries))
	for _, k := range entries {
		o := wire.NewObject()
		o.Set("seq", wire.String(string(k.Seq)))
		o.Set("operation", wire.String(k.Operation))
		switch k.Operation {
		case KnowHowAdd:
			o.Set("text", wire.String(k.Text))
			o.Set("anchors", KnowHowAnchorsValue(k.Anchors))
			o.Set("routes", wire.Strings(nonNilStrings(k.Routes)))
			o.Set("commit", wire.String(k.Commit))
			if k.Supersedes == nil {
				o.Set("supersedes", wire.Null())
			} else {
				o.Set("supersedes", wire.String(string(*k.Supersedes)))
			}
			o.Set("attempt", stringOrNull(k.Attempt))
			o.Set("generation", sizeOrNull(k.Generation))
			o.Set("evidencePath", stringOrNull(k.EvidencePath))
			o.Set("reason", stringOrNull(k.Reason))
		case KnowHowRetract:
			o.Set("note", wire.String(string(k.Note)))
			o.Set("reason", stringOrNull(k.Reason))
		default:
			o.Set("note", wire.String(string(k.Note)))
			o.Set("anchors", KnowHowAnchorsValue(k.Anchors))
			o.Set("commit", wire.String(k.Commit))
			o.Set("attempt", stringOrNull(k.Attempt))
			o.Set("generation", sizeOrNull(k.Generation))
		}
		o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(k.ActorID)).Set("role", wire.String(k.ActorRole))))
		o.Set("recordedAt", wire.String(string(k.RecordedAt)))
		vs = append(vs, wire.ObjectValue(o))
	}
	return wire.Array(vs...)
}

// KnowHowActiveSeqs returns the set of ADD entries that no later entry
// supersedes or retracts. A RECONFIRM changes no membership.
func KnowHowActiveSeqs(entries []KnowHowEntry) map[wire.Count]bool {
	active := map[wire.Count]bool{}
	for _, k := range entries {
		switch k.Operation {
		case KnowHowRetract:
			delete(active, k.Note)
		case KnowHowReconfirm:
		default:
			if k.Supersedes != nil {
				delete(active, *k.Supersedes)
			}
			active[k.Seq] = true
		}
	}
	return active
}

// ActiveKnowHow returns the active ADD entries in append order.
func ActiveKnowHow(entries []KnowHowEntry) []KnowHowEntry {
	active := KnowHowActiveSeqs(entries)
	out := []KnowHowEntry{}
	for _, k := range entries {
		if k.Operation == KnowHowAdd && active[k.Seq] {
			out = append(out, k)
		}
	}
	return out
}

// EffectiveKnowHow returns the active ADD entries in append order with each
// note's effective pins: the anchors and commit of its latest RECONFIRM, which
// is kept in Reconfirmed, else its own (KHN-V0-018).
func EffectiveKnowHow(entries []KnowHowEntry) []KnowHowEntry {
	latest := map[wire.Count]KnowHowEntry{}
	for _, k := range entries {
		if k.Operation == KnowHowReconfirm {
			latest[k.Note] = k
		}
	}
	out := ActiveKnowHow(entries)
	for i := range out {
		if r, ok := latest[out[i].Seq]; ok {
			out[i].Anchors, out[i].Commit, out[i].Reconfirmed = r.Anchors, r.Commit, &r
		}
	}
	return out
}

// KnowHowReconfirmRefusal applies wire.KnowHowReconfirmRefusal to anchors:
// a RECONFIRM re-pins exactly prior's paths and symbols and moves at least
// one pin in a way that makes the note STALE, a file anchor's blob or a
// symbol anchor's digest (KHN-V0-019).
func KnowHowReconfirmRefusal(prior, next []KnowHowAnchor) string {
	return wire.KnowHowReconfirmRefusal(KnowHowPins(prior), KnowHowPins(next))
}

// KnowHowPins is each anchor's freshness identity.
func KnowHowPins(anchors []KnowHowAnchor) []wire.KnowHowPin {
	out := make([]wire.KnowHowPin, len(anchors))
	for i, a := range anchors {
		out[i] = wire.KnowHowPin{Path: a.Path, Symbol: a.Symbol, Pin: a.Blob}
		if a.Symbol != "" {
			out[i].Pin = a.SymbolSha256
		}
	}
	return out
}

// validateKnowHow enforces the KHN-V0-002 relationships a closed type check
// cannot: seq is the 1-based position, a superseding ADD, a RETRACT and a
// RECONFIRM name an earlier ADD that is still active, an ADD carries a
// reason exactly when it supersedes, and a RECONFIRM re-pins the note's
// effective anchors with at least one STALE change (KHN-V0-019). A WORKER
// entry (KHN-V0-023) is only a non-superseding ADD that records its attempt
// and generation.
func (rec *Record) validateKnowHow() error {
	active := map[wire.Count]bool{}
	pins := map[wire.Count][]KnowHowAnchor{}
	for i, k := range rec.KnowHow {
		where := "/knowHow/" + idx(i)
		if k.Seq.Int() != int64(i+1) {
			return wire.Errorf(wire.CodeMalformed, where+"/seq", "seq %s is not the entry position %d", k.Seq, i+1)
		}
		target := k.Supersedes
		if k.Operation != KnowHowAdd {
			target = &k.Note
		}
		if target != nil && !active[*target] {
			return wire.Errorf(wire.CodeMalformed, where, "entry names note %s, which is not an earlier active note", *target)
		}
		if k.ActorRole == "WORKER" && (k.Operation != KnowHowAdd || k.Supersedes != nil || k.Attempt == nil || k.Generation == nil) {
			return wire.Errorf(wire.CodeMalformed, where+"/actor/role", "a WORKER entry is a non-superseding ADD that records its attempt and generation")
		}
		if k.Operation == KnowHowReconfirm {
			if why := KnowHowReconfirmRefusal(pins[k.Note], k.Anchors); why != "" {
				return wire.Errorf(wire.CodeMalformed, where+"/anchors", "%s", why)
			}
			pins[k.Note] = k.Anchors
			continue
		}
		if k.Operation == KnowHowAdd && (k.Supersedes == nil) != (k.Reason == nil) {
			return wire.Errorf(wire.CodeMalformed, where+"/reason", "an ADD carries a reason exactly when it supersedes")
		}
		if target != nil {
			delete(active, *target)
		}
		if k.Operation == KnowHowAdd {
			active[k.Seq] = true
			pins[k.Seq] = k.Anchors
		}
	}
	return nil
}
