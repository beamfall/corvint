// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// This file is the independent consumer's commitment codec, required by
// CF-V0-028: "an independent consumer MUST reproduce every reference result
// byte-for-byte from the same raw inputs and Git objects. Corvint
// producer-to-Corvint verifier tests alone cannot satisfy this interoperability
// requirement."
//
// It is authored solely from the CF-V0-019 clause text. It deliberately does
// not use encoding/json: that decoder replaces invalid UTF-8 and unpaired
// surrogates with U+FFFD and accepts duplicate object keys last-wins, each of
// which CF-V0-019 names as invalid input. A lenient parser would silently
// convert three required rejections into acceptances.
//
// CF-V0-019, verbatim obligations implemented below:
//   - values are only null, booleans, Unicode-scalar strings, arrays, objects;
//   - JSON numbers are forbidden;
//   - a duplicate object key, invalid UTF-8, a BOM, an unpaired surrogate, or a
//     forbidden value is invalid input;
//   - object keys sort by their raw UTF-8 bytes; arrays preserve declared order;
//   - serialization emits no whitespace and no terminal LF;
//   - true, false and null are written literally;
//   - `"` escapes as `\"` and `\` as `\\`;
//   - every U+0000..U+001F scalar escapes as lowercase `\u00xx`;
//   - every other scalar is emitted as its original UTF-8 bytes, with no
//     Unicode normalization and no optional escaping;
//   - a verifier MUST parse, reserialize, and require byte equality before
//     hashing;
//   - a complete document is codec(document) plus exactly one LF.

// Kind enumerates the closed CF-V0-019 value set. There is deliberately no
// number kind: JSON numbers are forbidden, so they are a parse error rather
// than a representable value.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindString
	KindArray
	KindObject
)

// Value is one codec value.
type Value struct {
	Kind Kind
	Bool bool
	Str  string
	Arr  []Value
	Obj  []Member
}

// Member is one object entry in declared parse order. Serialization sorts by
// raw UTF-8 key bytes; parse order is retained only so duplicate-key rejection
// and error reporting can name the offending position.
type Member struct {
	Key string
	Val Value
}

// Codec error reasons. These name the CF-V0-019 clause phrase that rejected the
// input; they are the vector-file `reason` vocabulary.
const (
	ReasonBOM               = "bom"
	ReasonInvalidUTF8       = "invalid-utf8"
	ReasonDuplicateKey      = "duplicate-key"
	ReasonUnpairedSurrogate = "unpaired-surrogate"
	ReasonNumberForbidden   = "number-forbidden"
	ReasonSyntax            = "syntax"
)

// CodecError carries the rejection reason so a vector can assert *why* the
// input was refused, not merely that it was.
type CodecError struct {
	Reason string
	Offset int
	Detail string
}

func (e *CodecError) Error() string {
	return fmt.Sprintf("codec: %s at byte %d: %s", e.Reason, e.Offset, e.Detail)
}

// Reason returns the rejection reason of a codec error, or "" for any other
// error value.
func Reason(err error) string {
	var ce *CodecError
	if errors.As(err, &ce) {
		return ce.Reason
	}
	return ""
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// Parse decodes raw bytes under the CF-V0-019 admitted value set. It is strict
// by construction: every clause-named invalid input is an error, never a
// repaired value.
func Parse(raw []byte) (Value, error) {
	if len(raw) >= 3 && raw[0] == bom[0] && raw[1] == bom[1] && raw[2] == bom[2] {
		return Value{}, &CodecError{Reason: ReasonBOM, Offset: 0, Detail: "input begins with a UTF-8 BOM"}
	}
	if !utf8.Valid(raw) {
		return Value{}, &CodecError{Reason: ReasonInvalidUTF8, Offset: firstInvalidUTF8(raw), Detail: "input is not well-formed UTF-8"}
	}
	p := &parser{src: raw}
	p.skipSpace()
	v, err := p.value()
	if err != nil {
		return Value{}, err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return Value{}, &CodecError{Reason: ReasonSyntax, Offset: p.pos, Detail: "trailing bytes after the top-level value"}
	}
	return v, nil
}

func firstInvalidUTF8(raw []byte) int {
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size <= 1 {
			return i
		}
		i += size
	}
	return len(raw)
}

type parser struct {
	src []byte
	pos int
}

func (p *parser) fail(reason, detail string) error {
	return &CodecError{Reason: reason, Offset: p.pos, Detail: detail}
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) value() (Value, error) {
	if p.pos >= len(p.src) {
		return Value{}, p.fail(ReasonSyntax, "unexpected end of input")
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		s, err := p.stringLiteral()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: s}, nil
	case p.literal("true"):
		return Value{Kind: KindBool, Bool: true}, nil
	case p.literal("false"):
		return Value{Kind: KindBool, Bool: false}, nil
	case p.literal("null"):
		return Value{Kind: KindNull}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		return Value{}, p.fail(ReasonNumberForbidden,
			"JSON numbers are forbidden; a count, ordinal or offset is a base-10 string")
	default:
		return Value{}, p.fail(ReasonSyntax, fmt.Sprintf("unexpected byte %q", c))
	}
}

// literal consumes an exact token when it is present. NaN, Infinity and
// -Infinity are not listed and therefore fall through to the number branch or
// the syntax branch, which is the required rejection either way.
func (p *parser) literal(tok string) bool {
	if strings.HasPrefix(string(p.src[p.pos:]), tok) {
		p.pos += len(tok)
		return true
	}
	return false
}

func (p *parser) array() (Value, error) {
	p.pos++ // '['
	out := Value{Kind: KindArray, Arr: []Value{}}
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		return out, nil
	}
	for {
		p.skipSpace()
		el, err := p.value()
		if err != nil {
			return Value{}, err
		}
		out.Arr = append(out.Arr, el)
		p.skipSpace()
		if p.pos >= len(p.src) {
			return Value{}, p.fail(ReasonSyntax, "unterminated array")
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return out, nil
		default:
			return Value{}, p.fail(ReasonSyntax, "expected ',' or ']'")
		}
	}
}

func (p *parser) object() (Value, error) {
	p.pos++ // '{'
	out := Value{Kind: KindObject, Obj: []Member{}}
	seen := map[string]bool{}
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		return out, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			return Value{}, p.fail(ReasonSyntax, "expected an object key string")
		}
		key, err := p.stringLiteral()
		if err != nil {
			return Value{}, err
		}
		if seen[key] {
			// CF-V0-019: "Input with a duplicate object key ... is invalid."
			// Never last-wins, never first-wins.
			return Value{}, p.fail(ReasonDuplicateKey, "duplicate object key")
		}
		seen[key] = true
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return Value{}, p.fail(ReasonSyntax, "expected ':' after an object key")
		}
		p.pos++
		p.skipSpace()
		val, err := p.value()
		if err != nil {
			return Value{}, err
		}
		out.Obj = append(out.Obj, Member{Key: key, Val: val})
		p.skipSpace()
		if p.pos >= len(p.src) {
			return Value{}, p.fail(ReasonSyntax, "unterminated object")
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return out, nil
		default:
			return Value{}, p.fail(ReasonSyntax, "expected ',' or '}'")
		}
	}
}

func (p *parser) stringLiteral() (string, error) {
	p.pos++ // opening quote
	var sb strings.Builder
	for {
		if p.pos >= len(p.src) {
			return "", p.fail(ReasonSyntax, "unterminated string")
		}
		c := p.src[p.pos]
		switch {
		case c == '"':
			p.pos++
			return sb.String(), nil
		case c < 0x20:
			return "", p.fail(ReasonSyntax, "raw control character in string")
		case c == '\\':
			r, err := p.escape()
			if err != nil {
				return "", err
			}
			sb.WriteRune(r)
		default:
			r, size := utf8.DecodeRune(p.src[p.pos:])
			sb.WriteRune(r)
			p.pos += size
		}
	}
}

func (p *parser) escape() (rune, error) {
	p.pos++ // backslash
	if p.pos >= len(p.src) {
		return 0, p.fail(ReasonSyntax, "unterminated escape")
	}
	c := p.src[p.pos]
	p.pos++
	switch c {
	case '"':
		return '"', nil
	case '\\':
		return '\\', nil
	case '/':
		return '/', nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		return p.unicodeEscape()
	default:
		return 0, p.fail(ReasonSyntax, fmt.Sprintf("unknown escape \\%c", c))
	}
}

// unicodeEscape decodes \uXXXX, joining a well-formed surrogate pair and
// rejecting any surrogate that is not part of one. CF-V0-019 admits only
// Unicode-scalar strings, and a lone surrogate is not a scalar.
func (p *parser) unicodeEscape() (rune, error) {
	first, err := p.hex4()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(rune(first)) {
		return rune(first), nil
	}
	if p.pos+1 < len(p.src) && p.src[p.pos] == '\\' && p.src[p.pos+1] == 'u' {
		save := p.pos
		p.pos += 2
		second, err := p.hex4()
		if err != nil {
			return 0, err
		}
		if joined := utf16.DecodeRune(rune(first), rune(second)); joined != utf8.RuneError {
			return joined, nil
		}
		p.pos = save
	}
	return 0, p.fail(ReasonUnpairedSurrogate,
		fmt.Sprintf("\\u%04x is a lone surrogate and denotes no Unicode scalar", first))
}

func (p *parser) hex4() (uint32, error) {
	if p.pos+4 > len(p.src) {
		return 0, p.fail(ReasonSyntax, "truncated \\u escape")
	}
	var v uint32
	for i := 0; i < 4; i++ {
		c := p.src[p.pos+i]
		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | uint32(c-'0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | uint32(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v<<4 | uint32(c-'A'+10)
		default:
			return 0, p.fail(ReasonSyntax, "non-hex digit in \\u escape")
		}
	}
	p.pos += 4
	return v, nil
}

const hexDigits = "0123456789abcdef"

// Codec serializes a value to its exact canonical bytes: no whitespace, no
// terminal LF.
func Codec(v Value) []byte {
	var sb strings.Builder
	writeValue(&sb, v)
	return []byte(sb.String())
}

func writeValue(sb *strings.Builder, v Value) {
	switch v.Kind {
	case KindNull:
		sb.WriteString("null")
	case KindBool:
		if v.Bool {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case KindString:
		writeString(sb, v.Str)
	case KindArray:
		sb.WriteByte('[')
		for i, el := range v.Arr {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeValue(sb, el)
		}
		sb.WriteByte(']')
	case KindObject:
		members := make([]Member, len(v.Obj))
		copy(members, v.Obj)
		// Go string comparison is a raw byte comparison, which is exactly the
		// CF-V0-019 order. It is NOT UTF-16 code-unit order: for keys U+FF3A
		// (ef bc ba) and U+10000 (f0 90 80 80) the two orders disagree, and the
		// raw-byte order places U+FF3A first.
		sort.SliceStable(members, func(i, j int) bool { return members[i].Key < members[j].Key })
		sb.WriteByte('{')
		for i, m := range members {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeString(sb, m.Key)
			sb.WriteByte(':')
			writeValue(sb, m.Val)
		}
		sb.WriteByte('}')
	}
}

func writeString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			sb.WriteString(`\"`)
		case r == '\\':
			sb.WriteString(`\\`)
		case r <= 0x1F:
			sb.WriteString(`\u00`)
			sb.WriteByte(hexDigits[(r>>4)&0xF])
			sb.WriteByte(hexDigits[r&0xF])
		default:
			// Original UTF-8 bytes, no normalization, no optional escaping.
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
}

// IsCanonical implements the CF-V0-019 verifier obligation: parse, reserialize,
// and require byte equality before hashing.
func IsCanonical(raw []byte) (bool, error) {
	v, err := Parse(raw)
	if err != nil {
		return false, err
	}
	return string(Codec(v)) == string(raw), nil
}

// IsCompleteDocument reports whether raw is codec(document) plus exactly one LF.
func IsCompleteDocument(raw []byte) (bool, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return false, nil
	}
	body := raw[:len(raw)-1]
	if len(body) > 0 && body[len(body)-1] == '\n' {
		return false, nil
	}
	ok, err := IsCanonical(body)
	if err != nil {
		return false, err
	}
	return ok, nil
}
