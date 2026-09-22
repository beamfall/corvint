// Package wire reads and validates CEM 0.1/0.2 wire documents.
//
// The reader is a strict JSON parser implementing interop/cem-0.1/ALGORITHMS.md
// "JSON and version dispatch": it rejects malformed UTF-8, duplicate object keys
// at any depth, non-integer or non-finite numbers, integers outside
// 0..9007199254740991, unpaired surrogates after JSON decoding, and inputs
// nested deeper than the documented operational depth bound.
package wire

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// MaxWireInteger is the largest integer a CEM wire document may carry.
const MaxWireInteger = 9007199254740991

// maxDepth is a documented operational nesting bound, far above any CEM shape.
const maxDepth = 64

// Kind enumerates strict JSON value kinds.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindString
	KindArray
	KindObject
)

// Value is one strict JSON value.
type Value struct {
	Kind Kind
	Bool bool
	Int  int64
	Str  string
	Arr  []Value
	Obj  *Object
}

// Object preserves member order and rejects duplicate keys at insertion.
type Object struct {
	Keys   []string
	Values map[string]Value
}

// Get returns the member value and whether it is present.
func (o *Object) Get(key string) (Value, bool) {
	value, ok := o.Values[key]
	return value, ok
}

type parser struct {
	data []byte
	pos  int
}

func invalid(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.InvalidJSON, format, args...)
}

// Parse reads one strict JSON document occupying the entire input.
func Parse(data []byte) (Value, error) {
	if !utf8.Valid(data) {
		return Value{}, invalid("input is not valid UTF-8")
	}
	p := &parser{data: data}
	p.skipSpace()
	value, err := p.parseValue(0)
	if err != nil {
		return Value{}, err
	}
	p.skipSpace()
	if p.pos != len(p.data) {
		return Value{}, invalid("trailing bytes after JSON value")
	}
	return value, nil
}

func (p *parser) skipSpace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *parser) parseValue(depth int) (Value, error) {
	if depth > maxDepth {
		return Value{}, invalid("JSON nesting exceeds the depth bound")
	}
	if p.pos >= len(p.data) {
		return Value{}, invalid("unexpected end of JSON input")
	}
	switch p.data[p.pos] {
	case '{':
		return p.parseObject(depth)
	case '[':
		return p.parseArray(depth)
	case '"':
		text, err := p.parseString()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: text}, nil
	case 't':
		return p.parseLiteral("true", Value{Kind: KindBool, Bool: true})
	case 'f':
		return p.parseLiteral("false", Value{Kind: KindBool})
	case 'n':
		return p.parseLiteral("null", Value{Kind: KindNull})
	default:
		return p.parseNumber()
	}
}

func (p *parser) parseLiteral(word string, value Value) (Value, error) {
	if p.pos+len(word) > len(p.data) || string(p.data[p.pos:p.pos+len(word)]) != word {
		return Value{}, invalid("invalid JSON literal")
	}
	p.pos += len(word)
	return value, nil
}

func (p *parser) parseNumber() (Value, error) {
	start := p.pos
	if p.pos < len(p.data) && p.data[p.pos] == '-' {
		return Value{}, invalid("negative numbers are outside the wire integer range")
	}
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c >= '0' && c <= '9' {
			p.pos++
			continue
		}
		if c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			return Value{}, invalid("non-integer numbers are invalid")
		}
		break
	}
	digits := p.data[start:p.pos]
	if len(digits) == 0 {
		return Value{}, invalid("invalid JSON number")
	}
	if len(digits) > 1 && digits[0] == '0' {
		return Value{}, invalid("leading zeros are invalid")
	}
	if len(digits) > 16 {
		return Value{}, invalid("integer exceeds the wire integer range")
	}
	var number int64
	for _, digit := range digits {
		number = number*10 + int64(digit-'0')
	}
	if number > MaxWireInteger {
		return Value{}, invalid("integer exceeds the wire integer range")
	}
	return Value{Kind: KindInt, Int: number}, nil
}

func (p *parser) parseString() (string, error) {
	if p.data[p.pos] != '"' {
		return "", invalid("expected string")
	}
	p.pos++
	var out []byte
	for {
		if p.pos >= len(p.data) {
			return "", invalid("unterminated string")
		}
		c := p.data[p.pos]
		switch {
		case c == '"':
			p.pos++
			return string(out), nil
		case c == '\\':
			decoded, err := p.parseEscape()
			if err != nil {
				return "", err
			}
			out = append(out, decoded...)
		case c < 0x20:
			return "", invalid("raw control byte in string")
		default:
			out = append(out, c)
			p.pos++
		}
	}
}

func (p *parser) parseEscape() ([]byte, error) {
	if p.pos+1 >= len(p.data) {
		return nil, invalid("unterminated escape")
	}
	marker := p.data[p.pos+1]
	simple := map[byte]byte{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
	if replacement, ok := simple[marker]; ok {
		p.pos += 2
		return []byte{replacement}, nil
	}
	if marker != 'u' {
		return nil, invalid("invalid escape sequence")
	}
	first, err := p.parseHex4(p.pos + 2)
	if err != nil {
		return nil, err
	}
	p.pos += 6
	if utf16.IsSurrogate(rune(first)) {
		if first >= 0xDC00 {
			return nil, invalid("unpaired low surrogate")
		}
		if p.pos+6 > len(p.data) || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
			return nil, invalid("unpaired high surrogate")
		}
		second, err := p.parseHex4(p.pos + 2)
		if err != nil {
			return nil, err
		}
		combined := utf16.DecodeRune(rune(first), rune(second))
		if combined == utf8.RuneError {
			return nil, invalid("unpaired high surrogate")
		}
		p.pos += 6
		return utf8.AppendRune(nil, combined), nil
	}
	return utf8.AppendRune(nil, rune(first)), nil
}

func (p *parser) parseHex4(start int) (int, error) {
	if start+4 > len(p.data) {
		return 0, invalid("truncated \\u escape")
	}
	value := 0
	for _, c := range p.data[start : start+4] {
		value <<= 4
		switch {
		case c >= '0' && c <= '9':
			value |= int(c - '0')
		case c >= 'a' && c <= 'f':
			value |= int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			value |= int(c-'A') + 10
		default:
			return 0, invalid("invalid \\u escape")
		}
	}
	return value, nil
}

func (p *parser) parseArray(depth int) (Value, error) {
	p.pos++
	p.skipSpace()
	items := []Value{}
	if p.pos < len(p.data) && p.data[p.pos] == ']' {
		p.pos++
		return Value{Kind: KindArray, Arr: items}, nil
	}
	for {
		item, err := p.parseValue(depth + 1)
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
		p.skipSpace()
		if p.pos >= len(p.data) {
			return Value{}, invalid("unterminated array")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
			p.skipSpace()
		case ']':
			p.pos++
			return Value{Kind: KindArray, Arr: items}, nil
		default:
			return Value{}, invalid("invalid array separator")
		}
	}
}

func (p *parser) parseObject(depth int) (Value, error) {
	p.pos++
	p.skipSpace()
	object := &Object{Values: map[string]Value{}}
	if p.pos < len(p.data) && p.data[p.pos] == '}' {
		p.pos++
		return Value{Kind: KindObject, Obj: object}, nil
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return Value{}, invalid("expected object key")
		}
		key, err := p.parseString()
		if err != nil {
			return Value{}, err
		}
		if _, duplicate := object.Values[key]; duplicate {
			return Value{}, invalid("duplicate object key %q", key)
		}
		p.skipSpace()
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return Value{}, invalid("expected ':' after object key")
		}
		p.pos++
		p.skipSpace()
		member, err := p.parseValue(depth + 1)
		if err != nil {
			return Value{}, err
		}
		object.Keys = append(object.Keys, key)
		object.Values[key] = member
		p.skipSpace()
		if p.pos >= len(p.data) {
			return Value{}, invalid("unterminated object")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return Value{Kind: KindObject, Obj: object}, nil
		default:
			return Value{}, invalid("invalid object separator")
		}
	}
}
