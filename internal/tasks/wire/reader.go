package wire

import (
	"sort"
	"strings"
)

// Reader walks a parsed Value with closed-object checks and typed accessors.
// The first failure is recorded and every later call returns a zero value, so
// a decoder reads linearly and checks Err once. Locations are JSON-pointer
// style paths such as `/dependencies/2/gateId`.
type Reader struct {
	v     Value
	where string
	st    *readerState
}

type readerState struct {
	err *Error
}

// NewReader starts a reader at the document root (or any sub-value).
func NewReader(v Value, where string) *Reader {
	if where == "" {
		where = "/"
	}
	return &Reader{v: v, where: where, st: &readerState{}}
}

// Err returns the first failure, or nil.
func (r *Reader) Err() error {
	if r.st.err == nil {
		return nil
	}
	return r.st.err
}

// Fail records a failure at this reader's location unless one exists.
func (r *Reader) Fail(code, format string, args ...interface{}) {
	if r.st.err == nil {
		r.st.err = Errorf(code, r.where, format, args...)
	}
}

// Where returns the reader's location.
func (r *Reader) Where() string { return r.where }

// Value returns the underlying value.
func (r *Reader) Value() Value { return r.v }

func (r *Reader) child(v Value, seg string) *Reader {
	w := r.where
	if w == "/" {
		w = ""
	}
	return &Reader{v: v, where: w + "/" + seg, st: r.st}
}

// Closed requires the value to be an object with exactly the listed keys;
// missing and unknown keys are both MALFORMED, named in the message.
func (r *Reader) Closed(keys ...string) *Reader {
	if r.st.err != nil {
		return r
	}
	if r.v.Kind != KindObject || r.v.Obj == nil {
		r.Fail(CodeMalformed, "expected object, got %s", r.v.Kind)
		return r
	}
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	var missing, unknown []string
	for _, k := range keys {
		if _, ok := r.v.Obj.Vals[k]; !ok {
			missing = append(missing, k)
		}
	}
	for _, k := range r.v.Obj.Keys {
		if !want[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	if len(missing) > 0 || len(unknown) > 0 {
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, "missing keys: "+strings.Join(missing, ", "))
		}
		if len(unknown) > 0 {
			parts = append(parts, "unknown keys: "+strings.Join(unknown, ", "))
		}
		r.Fail(CodeMalformed, "closed object violated (%s)", strings.Join(parts, "; "))
	}
	return r
}

// Field returns a reader for a member of an object. Closed must have been
// called first; a missing member here is still reported.
func (r *Reader) Field(key string) *Reader {
	if r.st.err != nil {
		return r.child(Null(), key)
	}
	if r.v.Kind != KindObject || r.v.Obj == nil {
		r.Fail(CodeMalformed, "expected object, got %s", r.v.Kind)
		return r.child(Null(), key)
	}
	v, ok := r.v.Obj.Vals[key]
	if !ok {
		r.Fail(CodeMalformed, "missing key %q", key)
		return r.child(Null(), key)
	}
	return r.child(v, key)
}

// IsNull reports whether the value is JSON null.
func (r *Reader) IsNull() bool { return r.v.Kind == KindNull }

// String requires a string.
func (r *Reader) String() string {
	if r.st.err != nil {
		return ""
	}
	if r.v.Kind != KindString {
		r.Fail(CodeMalformed, "expected string, got %s", r.v.Kind)
		return ""
	}
	return r.v.Str
}

// Bool requires a boolean.
func (r *Reader) Bool() bool {
	if r.st.err != nil {
		return false
	}
	if r.v.Kind != KindBool {
		r.Fail(CodeMalformed, "expected boolean, got %s", r.v.Kind)
		return false
	}
	return r.v.Bool
}

// Enum requires one of the allowed strings.
func (r *Reader) Enum(allowed ...string) string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	r.Fail(CodeMalformed, "value %q not in {%s}", s, strings.Join(allowed, "|"))
	return ""
}

// Exact requires the string to equal one literal.
func (r *Reader) Exact(want string) string {
	return r.Enum(want)
}

func (r *Reader) adopt(err error) {
	if err != nil && r.st.err == nil {
		if e, ok := err.(*Error); ok {
			r.st.err = e
		} else {
			r.st.err = Errorf(CodeMalformed, r.where, "%v", err)
		}
	}
}

// Count requires a Count.
func (r *Reader) Count() Count {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	c, err := ParseCount(r.where, s)
	r.adopt(err)
	return c
}

// Size requires a Size.
func (r *Reader) Size() Size {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	c, err := ParseSize(r.where, s)
	r.adopt(err)
	return c
}

// Digest requires a Digest.
func (r *Reader) Digest() Digest {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	d, err := ParseDigest(r.where, s)
	r.adopt(err)
	return d
}

// DigestOrNull accepts a Digest or null.
func (r *Reader) DigestOrNull() *Digest {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	d := r.Digest()
	if r.st.err != nil {
		return nil
	}
	return &d
}

// OID requires a Git object id.
func (r *Reader) OID() string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	o, err := ParseOID(r.where, s)
	r.adopt(err)
	return o
}

// Identifier requires an Identifier.
func (r *Reader) Identifier() string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	id, err := ParseIdentifier(r.where, s)
	r.adopt(err)
	return id
}

// PathText requires a PathText (an absolute filesystem path, §2).
func (r *Reader) PathText() string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	p, err := ParsePathText(r.where, s)
	r.adopt(err)
	return p
}

// Label requires a label.
func (r *Reader) Label() string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	l, err := ParseLabel(r.where, s)
	r.adopt(err)
	return l
}

// LabelOrNull accepts a label or null.
func (r *Reader) LabelOrNull() *string {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	l := r.Label()
	if r.st.err != nil {
		return nil
	}
	return &l
}

// Prose requires prose of min..max bytes.
func (r *Reader) Prose(min, max int) string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	p, err := ParseProse(r.where, s, min, max)
	r.adopt(err)
	return p
}

// ProseOrNull accepts prose or null.
func (r *Reader) ProseOrNull(max int) *string {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	p := r.Prose(0, max)
	if r.st.err != nil {
		return nil
	}
	return &p
}

// Path requires a WQO Path.
func (r *Reader) Path() string {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	p, err := ParsePath(r.where, s)
	r.adopt(err)
	return p
}

// Timestamp requires a timestamp.
func (r *Reader) Timestamp() Timestamp {
	s := r.String()
	if r.st.err != nil {
		return ""
	}
	t, err := ParseTimestamp(r.where, s)
	r.adopt(err)
	return t
}

// TimestampOrNull accepts a timestamp or null.
func (r *Reader) TimestampOrNull() *Timestamp {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	t := r.Timestamp()
	if r.st.err != nil {
		return nil
	}
	return &t
}

// CountOrNull accepts a Count or null.
func (r *Reader) CountOrNull() *Count {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	c := r.Count()
	if r.st.err != nil {
		return nil
	}
	return &c
}

// SizeOrNull accepts a Size or null.
func (r *Reader) SizeOrNull() *Size {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	c := r.Size()
	if r.st.err != nil {
		return nil
	}
	return &c
}

// StringOrNull accepts an identifier-class string validated by fn, or null.
func (r *Reader) StringOrNull(fn func(*Reader) string) *string {
	if r.st.err != nil || r.IsNull() {
		return nil
	}
	s := fn(r)
	if r.st.err != nil {
		return nil
	}
	return &s
}

// TicketID requires a ticket ID.
func (r *Reader) TicketID() TicketID {
	s := r.String()
	if r.st.err != nil {
		return TicketID{}
	}
	id, err := ParseTicketID(r.where, s)
	r.adopt(err)
	return id
}

// QueueID requires a queue ID.
func (r *Reader) QueueID() QueueID {
	s := r.String()
	if r.st.err != nil {
		return QueueID{}
	}
	id, err := ParseQueueID(r.where, s)
	r.adopt(err)
	return id
}

// Array requires an array of at most max elements (max < 0 means unbounded
// beyond the §1 decode bound). When semantic is false the array must be
// canonical-byte sorted without duplicates (§2).
func (r *Reader) Array(max int, semantic bool) []*Reader {
	if r.st.err != nil {
		return nil
	}
	if r.v.Kind != KindArray {
		r.Fail(CodeMalformed, "expected array, got %s", r.v.Kind)
		return nil
	}
	if max >= 0 && len(r.v.Arr) > max {
		r.Fail(CodeLimitExceeded, "array longer than %d elements (%d)", max, len(r.v.Arr))
		return nil
	}
	if !semantic {
		if err := CheckSortedUnique(r.where, r.v.Arr); err != nil {
			r.adopt(err)
			return nil
		}
	}
	out := make([]*Reader, len(r.v.Arr))
	for i, e := range r.v.Arr {
		out[i] = r.child(e, itoa(i))
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

// Strings reads an array of strings, each validated by fn (for example
// (*Reader).Label), with the given bound and sortedness rule.
func (r *Reader) Strings(max int, semantic bool, fn func(*Reader) string) []string {
	items := r.Array(max, semantic)
	if r.st.err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s := fn(it)
		if r.st.err != nil {
			return nil
		}
		out = append(out, s)
	}
	return out
}
