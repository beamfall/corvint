// Package wp3codec implements the WP3 commitment codec: the dependency-free
// canonical JSON subset frozen under "Canonical evaluation commitments" in
// docs/specs/lexical-relevance-floor-v0.md and restated word-for-word by
// docs/specs/change-frontier-v0.md CF-V0-019.
//
// CF-V0-019 requires this definition to be "reused, not independently
// approximated". No shared Go implementation existed when Change Frontier V0
// was built, and neither near neighbour could be reused without emitting
// different bytes:
//
//   - internal/lrf/canonical.go is the LRF *wire* serializer. It emits JSON
//     numbers for the inputs tuple and a terminal LF, both of which this codec
//     forbids.
//   - internal/cem/wire.CanonicalString uses a different escape profile: it
//     emits the short forms \b \f \n \r \t where this codec requires the
//     lowercase \u00xx form for every U+0000..U+001F scalar.
//
// This package is therefore the single implementation every consumer shares.
// It lives outside internal/frontier precisely so that internal/lrf and
// internal/tcq can adopt it without an import cycle, and it depends on nothing
// but the standard library so that "dependency-free" stays true.
package wp3codec

import (
	"bytes"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxDepth bounds recursion so a hostile nesting chain cannot exhaust the
// stack before the caller's own resource ceiling notices.
const maxDepth = 64

// ErrInvalid is the sentinel every codec rejection wraps. Callers translate it
// to their own profile code; the reason text is diagnostic only and must not
// be rendered into a wire envelope (CF-V0-024).
var ErrInvalid = errors.New("wp3codec: invalid canonical value")

func invalid(reason string) error { return invalidError{reason: reason} }

type invalidError struct{ reason string }

func (e invalidError) Error() string { return "wp3codec: " + e.reason }

func (e invalidError) Is(target error) bool { return target == ErrInvalid }

// Kind enumerates the only value types the codec admits. JSON numbers are
// deliberately absent: every count, ordinal, and offset travels as a decimal
// string instead.
type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindString
	KindArray
	KindObject
)

// Member is one object entry. Keys are compared and sorted by their raw UTF-8
// bytes, never by code point or locale.
type Member struct {
	Key   string
	Value Value
}

// Value is an immutable canonical value. The zero Value is JSON null, which
// makes an accidentally unset field encode as null rather than as a panic.
type Value struct {
	kind    Kind
	boolean bool
	text    string
	items   []Value
	members []Member
}

// Kind reports which admitted type this value carries.
func (v Value) Kind() Kind { return v.kind }

// Null returns the JSON null value.
func Null() Value { return Value{kind: KindNull} }

// Bool returns a boolean value.
func Bool(value bool) Value { return Value{kind: KindBool, boolean: value} }

// String returns a string value. Validity of the scalars is checked at encode
// time so that construction stays total.
func String(text string) Value { return Value{kind: KindString, text: text} }

// Decimal returns the codec representation of a count, ordinal, or offset: a
// base-10 string with no sign and no leading zero except "0". A negative input
// has no canonical form, so it is rejected rather than silently signed.
func Decimal(value int64) (Value, error) {
	if value < 0 {
		return Value{}, invalid("decimal value must not be negative")
	}
	return String(strconv.FormatInt(value, 10)), nil
}

// Array returns an array value preserving the declared order of items.
func Array(items ...Value) Value {
	return Value{kind: KindArray, items: append([]Value(nil), items...)}
}

// Strings returns an array of string values, preserving declared order.
func Strings(items []string) Value {
	values := make([]Value, 0, len(items))
	for _, item := range items {
		values = append(values, String(item))
	}
	return Value{kind: KindArray, items: values}
}

// Object returns an object value. Members are stored sorted by raw UTF-8 key
// bytes so that encoding is a straight walk and two objects built in different
// declaration orders are indistinguishable.
func Object(members ...Member) Value {
	sorted := append([]Member(nil), members...)
	sort.SliceStable(sorted, func(left, right int) bool {
		return sorted[left].Key < sorted[right].Key
	})
	return Value{kind: KindObject, members: sorted}
}

// Encode serializes one value to its exact canonical bytes: no whitespace, no
// terminal LF, keys sorted by raw UTF-8 bytes, arrays in declared order.
func Encode(value Value) ([]byte, error) {
	var out bytes.Buffer
	if err := encodeValue(&out, value, 0); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeValue(out *bytes.Buffer, value Value, depth int) error {
	if depth > maxDepth {
		return invalid("nesting exceeds the codec depth bound")
	}
	switch value.kind {
	case KindNull:
		out.WriteString("null")
		return nil
	case KindBool:
		return encodeBool(out, value.boolean)
	case KindString:
		return encodeString(out, value.text)
	case KindArray:
		return encodeArray(out, value.items, depth)
	case KindObject:
		return encodeObject(out, value.members, depth)
	}
	return invalid("unknown value kind")
}

func encodeBool(out *bytes.Buffer, value bool) error {
	if value {
		out.WriteString("true")
		return nil
	}
	out.WriteString("false")
	return nil
}

func encodeArray(out *bytes.Buffer, items []Value, depth int) error {
	out.WriteByte('[')
	for index, item := range items {
		if index > 0 {
			out.WriteByte(',')
		}
		if err := encodeValue(out, item, depth+1); err != nil {
			return err
		}
	}
	out.WriteByte(']')
	return nil
}

func encodeObject(out *bytes.Buffer, members []Member, depth int) error {
	if err := rejectDuplicateKeys(members); err != nil {
		return err
	}
	out.WriteByte('{')
	for index, member := range members {
		if index > 0 {
			out.WriteByte(',')
		}
		if err := encodeString(out, member.Key); err != nil {
			return err
		}
		out.WriteByte(':')
		if err := encodeValue(out, member.Value, depth+1); err != nil {
			return err
		}
	}
	out.WriteByte('}')
	return nil
}

// rejectDuplicateKeys enforces the clause's duplicate-key rule on the emit
// side too: a duplicate would otherwise produce bytes no verifier can accept,
// which is a worse failure than refusing to write them.
func rejectDuplicateKeys(members []Member) error {
	for index := 1; index < len(members); index++ {
		if members[index].Key == members[index-1].Key {
			return invalid("duplicate object key")
		}
	}
	return nil
}

// encodeString writes the frozen escape profile: only " and \ take short
// escapes, every U+0000..U+001F scalar becomes lowercase \u00xx, and every
// other scalar is emitted as its original UTF-8 bytes with no normalization
// and no optional escaping.
func encodeString(out *bytes.Buffer, text string) error {
	if err := validateScalars(text); err != nil {
		return err
	}
	out.WriteByte('"')
	for _, scalar := range text {
		switch {
		case scalar == '"':
			out.WriteString(`\"`)
		case scalar == '\\':
			out.WriteString(`\\`)
		case scalar < 0x20:
			out.WriteString(`\u00`)
			out.WriteString(lowerHexPair(byte(scalar)))
		default:
			out.WriteRune(scalar)
		}
	}
	out.WriteByte('"')
	return nil
}

func lowerHexPair(value byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[value>>4], digits[value&0x0f]})
}

// validateScalars rejects the exact inputs the clause names invalid: invalid
// UTF-8 and unpaired surrogates. Go strings can hold both, so this cannot be
// left to the type system.
func validateScalars(text string) error {
	if !utf8.ValidString(text) {
		return invalid("string is not valid UTF-8")
	}
	for _, scalar := range text {
		if scalar >= 0xD800 && scalar <= 0xDFFF {
			return invalid("string contains an unpaired surrogate")
		}
	}
	return nil
}

// Verify performs the check CF-V0-019 requires before hashing: parse the
// candidate bytes, reserialize them, and require byte equality. This
// comparison — not Parse — is what enforces canonicity. Because Encode sorts
// keys, strips whitespace, and emits only the minimal escape form, one
// comparison rejects mis-ordered keys, insignificant whitespace, short and
// uppercase-hex and optional escapes, and a trailing LF, all at once.
func Verify(data []byte) error {
	parsed, err := Parse(data)
	if err != nil {
		return err
	}
	reserialized, err := Encode(parsed)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, reserialized) {
		return invalid("bytes are not the canonical serialization of their own value")
	}
	return nil
}

// Parse reads the JSON subset the codec admits. It is deliberately NOT limited
// to canonical bytes: CF-V0-019 gives a closed list of what makes input invalid
// — "a duplicate object key, invalid UTF-8, a BOM, an unpaired surrogate, or a
// forbidden value" — and insignificant whitespace, the short escapes, and
// uppercase hex are not on it. The clause then mandates a separate "parse,
// reserialize, and require byte equality" step, which would be dead code if
// Parse already rejected every non-canonical byte sequence. So Parse accepts
// the broader subset and decodes it to scalars, Encode emits the one canonical
// form, and Verify's comparison is the thing that refuses non-canonical bytes.
// That division is also what lets this package canonicalize input at all, which
// an independent consumer needs under CF-V0-028.
//
// JSON numbers stay a parse error: they are "a forbidden value" by the clause's
// own words, not a spelling of an admitted one.
func Parse(data []byte) (Value, error) {
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return Value{}, invalid("input begins with a byte order mark")
	}
	if !utf8.Valid(data) {
		return Value{}, invalid("input is not valid UTF-8")
	}
	reader := &scanner{data: data}
	value, err := reader.readValue(0)
	if err != nil {
		return Value{}, err
	}
	// Trailing whitespace — a terminal LF above all — parses and is then caught
	// by Verify's byte comparison. The complete-document framing
	// (codec(document) plus exactly one LF) is a separate layer that lives in
	// the consuming profile, not here.
	reader.skipWhitespace()
	if reader.offset != len(data) {
		return Value{}, invalid("trailing bytes after the top-level value")
	}
	return value, nil
}

type scanner struct {
	data   []byte
	offset int
}

func (s *scanner) peek() (byte, bool) {
	if s.offset >= len(s.data) {
		return 0, false
	}
	return s.data[s.offset], true
}

// skipWhitespace consumes insignificant whitespace between tokens. It is not a
// canonicity hole: Encode emits none, so any byte skipped here makes the
// reserialized form differ and Verify rejects it.
func (s *scanner) skipWhitespace() {
	for s.offset < len(s.data) {
		switch s.data[s.offset] {
		case ' ', '\t', '\n', '\r':
			s.offset++
		default:
			return
		}
	}
}

func (s *scanner) expect(char byte) error {
	s.skipWhitespace()
	current, ok := s.peek()
	if !ok || current != char {
		return invalid("expected " + string(char))
	}
	s.offset++
	return nil
}

func (s *scanner) readValue(depth int) (Value, error) {
	if depth > maxDepth {
		return Value{}, invalid("nesting exceeds the codec depth bound")
	}
	s.skipWhitespace()
	current, ok := s.peek()
	if !ok {
		return Value{}, invalid("unexpected end of input")
	}
	switch current {
	case 'n':
		return Null(), s.readLiteral("null")
	case 't':
		return Bool(true), s.readLiteral("true")
	case 'f':
		return Bool(false), s.readLiteral("false")
	case '"':
		text, err := s.readString()
		return String(text), err
	case '[':
		return s.readArray(depth)
	case '{':
		return s.readObject(depth)
	}
	// A JSON number lands here. The clause names it a forbidden value, so it is
	// rejected at parse rather than normalized into an admitted type.
	return Value{}, invalid("forbidden or unexpected value")
}

func (s *scanner) readLiteral(literal string) error {
	if !bytes.HasPrefix(s.data[s.offset:], []byte(literal)) {
		return invalid("malformed literal")
	}
	s.offset += len(literal)
	return nil
}

func (s *scanner) readArray(depth int) (Value, error) {
	if err := s.expect('['); err != nil {
		return Value{}, err
	}
	items := []Value{}
	s.skipWhitespace()
	if current, ok := s.peek(); ok && current == ']' {
		s.offset++
		return Value{kind: KindArray, items: items}, nil
	}
	for {
		item, err := s.readValue(depth + 1)
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
		s.skipWhitespace()
		current, ok := s.peek()
		if !ok {
			return Value{}, invalid("unterminated array")
		}
		s.offset++
		if current == ']' {
			return Value{kind: KindArray, items: items}, nil
		}
		if current != ',' {
			return Value{}, invalid("expected , or ] in array")
		}
	}
}

func (s *scanner) readObject(depth int) (Value, error) {
	if err := s.expect('{'); err != nil {
		return Value{}, err
	}
	members := []Member{}
	seen := map[string]struct{}{}
	s.skipWhitespace()
	if current, ok := s.peek(); ok && current == '}' {
		s.offset++
		return Value{kind: KindObject, members: members}, nil
	}
	for {
		member, err := s.readMember(depth)
		if err != nil {
			return Value{}, err
		}
		if _, duplicate := seen[member.Key]; duplicate {
			return Value{}, invalid("duplicate object key")
		}
		seen[member.Key] = struct{}{}
		members = append(members, member)
		s.skipWhitespace()
		current, ok := s.peek()
		if !ok {
			return Value{}, invalid("unterminated object")
		}
		s.offset++
		if current == '}' {
			return Object(members...), nil
		}
		if current != ',' {
			return Value{}, invalid("expected , or } in object")
		}
	}
}

func (s *scanner) readMember(depth int) (Member, error) {
	key, err := s.readString()
	if err != nil {
		return Member{}, err
	}
	if err := s.expect(':'); err != nil {
		return Member{}, err
	}
	value, err := s.readValue(depth + 1)
	if err != nil {
		return Member{}, err
	}
	return Member{Key: key, Value: value}, nil
}

func (s *scanner) readString() (string, error) {
	if err := s.expect('"'); err != nil {
		return "", err
	}
	var out strings.Builder
	for {
		current, ok := s.peek()
		if !ok {
			return "", invalid("unterminated string")
		}
		s.offset++
		switch {
		case current == '"':
			return out.String(), validateScalars(out.String())
		case current == '\\':
			if err := s.readEscape(&out); err != nil {
				return "", err
			}
		case current < 0x20:
			return "", invalid("raw control byte inside a string")
		default:
			out.WriteByte(current)
		}
	}
}

// shortEscapes is the JSON escape set Parse decodes. \b \f \n \r \t and \/ are
// admitted spellings of ordinary scalars, not admitted OUTPUT: Encode re-emits
// the first five as lowercase \u00xx and the solidus raw, so a document using
// them fails the byte comparison.
var shortEscapes = map[byte]byte{
	'"':  '"',
	'\\': '\\',
	'/':  '/',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
}

func (s *scanner) readEscape(out *strings.Builder) error {
	current, ok := s.peek()
	if !ok {
		return invalid("unterminated escape")
	}
	s.offset++
	// Every JSON escape decodes to its scalar. Only two of them survive Encode
	// in escaped form; the rest re-emit differently and are refused by Verify's
	// byte comparison, which is where CF-V0-019 puts that judgement.
	decoded, admitted := shortEscapes[current]
	if admitted {
		out.WriteByte(decoded)
		return nil
	}
	if current == 'u' {
		return s.readUnicodeEscape(out)
	}
	return invalid("unknown escape")
}

func (s *scanner) readUnicodeEscape(out *strings.Builder) error {
	first, err := s.readHexQuad()
	if err != nil {
		return err
	}
	if first >= 0xDC00 && first <= 0xDFFF {
		return invalid("string contains an unpaired surrogate")
	}
	if first < 0xD800 || first > 0xDBFF {
		out.WriteRune(rune(first))
		return nil
	}
	// The low half must follow immediately: expect would skip whitespace, which
	// is not insignificant inside a string and would admit raw control bytes.
	if !bytes.HasPrefix(s.data[s.offset:], []byte(`\u`)) {
		return invalid("string contains an unpaired surrogate")
	}
	s.offset += 2
	second, err := s.readHexQuad()
	if err != nil {
		return err
	}
	if second < 0xDC00 || second > 0xDFFF {
		return invalid("string contains an unpaired surrogate")
	}
	out.WriteRune(rune(0x10000 + (first-0xD800)<<10 + (second - 0xDC00)))
	return nil
}

func (s *scanner) readHexQuad() (rune, error) {
	if s.offset+4 > len(s.data) {
		return 0, invalid("truncated \\u escape")
	}
	quad := string(s.data[s.offset : s.offset+4])
	// Uppercase hex is an admitted spelling on the way in; Encode always writes
	// the lowercase form, so \\u000A round-trips to \\u000a and Verify rejects it.
	parsed, err := strconv.ParseUint(quad, 16, 32)
	if err != nil || !isPlainHexQuad(quad) {
		return 0, invalid("malformed \\u escape")
	}
	s.offset += 4
	return rune(parsed), nil
}

// Text returns the scalar contents of a string value, and "" for every other
// kind. A verifier pairs it with Kind rather than trusting the empty string.
func (v Value) Text() string { return v.text }

// Boolean returns the contents of a boolean value.
func (v Value) Boolean() bool { return v.boolean }

// Items returns a copy of an array value's elements in declared order.
func (v Value) Items() []Value { return append([]Value(nil), v.items...) }

// Members returns a copy of an object value's entries in canonical key order.
func (v Value) Members() []Member { return append([]Member(nil), v.members...) }

// Lookup returns the member with the given key.
func (v Value) Lookup(key string) (Value, bool) {
	for _, member := range v.members {
		if member.Key == key {
			return member.Value, true
		}
	}
	return Value{}, false
}

// Keys returns an object value's keys in canonical order.
func (v Value) Keys() []string {
	keys := make([]string, 0, len(v.members))
	for _, member := range v.members {
		keys = append(keys, member.Key)
	}
	return keys
}

// isPlainHexQuad rejects the spellings strconv tolerates but JSON does not,
// such as a sign or an underscore, so only four hex digits are accepted.
func isPlainHexQuad(quad string) bool {
	if len(quad) != 4 {
		return false
	}
	for index := 0; index < len(quad); index++ {
		digit := quad[index]
		hex := digit >= '0' && digit <= '9' ||
			digit >= 'a' && digit <= 'f' ||
			digit >= 'A' && digit <= 'F'
		if !hex {
			return false
		}
	}
	return true
}
