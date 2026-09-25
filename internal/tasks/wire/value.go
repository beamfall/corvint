// Package wire implements SPEC §2 (identity and canonical encoding), the §1
// numeric limits, the §11 closed detail codes and the taskman-command-result/0
// envelope of §3.3. It is standard-library only and has no knowledge of the
// journal or of any process; every other package builds on it.
//
// The value model is deliberately tiny: taskman profiles use only strings,
// booleans, null, arrays and objects. JSON numbers are never legal (Count and
// Size are decimal strings, §2), so the parser refuses them outright.
package wire

import "sort"

// Kind enumerates the JSON value kinds a taskman document may contain.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindString
	KindArray
	KindObject
)

func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindBool:
		return "boolean"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	}
	return "unknown"
}

// Value is one decoded JSON value.
type Value struct {
	Kind Kind
	Bool bool
	Str  string
	Arr  []Value
	Obj  *Object
}

// Object is a JSON object. Keys keeps the order in which members were parsed
// or set; canonical encoding always sorts by UTF-8 byte order regardless.
type Object struct {
	Keys []string
	Vals map[string]Value
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{Vals: map[string]Value{}}
}

// Set adds or replaces a member and returns the object for chaining.
func (o *Object) Set(key string, v Value) *Object {
	if _, ok := o.Vals[key]; !ok {
		o.Keys = append(o.Keys, key)
	}
	o.Vals[key] = v
	return o
}

// Get returns a member and whether it exists.
func (o *Object) Get(key string) (Value, bool) {
	v, ok := o.Vals[key]
	return v, ok
}

// SortedKeys returns the member keys in canonical (byte) order.
func (o *Object) SortedKeys() []string {
	keys := make([]string, len(o.Keys))
	copy(keys, o.Keys)
	sort.Strings(keys)
	return keys
}

// Null returns the JSON null value.
func Null() Value { return Value{Kind: KindNull} }

// Bool returns a JSON boolean.
func Bool(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// String returns a JSON string.
func String(s string) Value { return Value{Kind: KindString, Str: s} }

// Array returns a JSON array keeping the given (semantic) order.
func Array(vs ...Value) Value {
	if vs == nil {
		vs = []Value{}
	}
	return Value{Kind: KindArray, Arr: vs}
}

// ObjectValue wraps an object as a value.
func ObjectValue(o *Object) Value { return Value{Kind: KindObject, Obj: o} }

// StringOrNull returns a string value, or null when p is nil.
func StringOrNull(p *string) Value {
	if p == nil {
		return Null()
	}
	return String(*p)
}

// Strings returns an array of strings in the given order.
func Strings(ss []string) Value {
	vs := make([]Value, len(ss))
	for i, s := range ss {
		vs[i] = String(s)
	}
	return Array(vs...)
}

// SortedSet sorts values by canonical bytes (§2: non-semantic arrays) and
// reports a duplicate as MALFORMED.
func SortedSet(where string, vs []Value) (Value, error) {
	type keyed struct {
		key string
		v   Value
	}
	ks := make([]keyed, len(vs))
	for i, v := range vs {
		ks[i] = keyed{key: string(Encode(v)), v: v}
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].key < ks[j].key })
	out := make([]Value, len(ks))
	for i, k := range ks {
		if i > 0 && ks[i-1].key == k.key {
			return Value{}, Errorf(CodeMalformed, where, "duplicate element %s", k.key)
		}
		out[i] = k.v
	}
	return Array(out...), nil
}

// CheckSortedUnique verifies that an array is canonical-byte sorted without
// duplicates (§2 rule for non-semantic arrays).
func CheckSortedUnique(where string, vs []Value) error {
	prev := ""
	for i, v := range vs {
		key := string(Encode(v))
		if i > 0 {
			if key == prev {
				return Errorf(CodeMalformed, where, "duplicate element at index %d", i)
			}
			if key < prev {
				return Errorf(CodeMalformed, where, "array is not canonical-byte sorted at index %d", i)
			}
		}
		prev = key
	}
	return nil
}

// Equal reports whether two values encode to identical canonical bytes.
func Equal(a, b Value) bool {
	return string(Encode(a)) == string(Encode(b))
}
