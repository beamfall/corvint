package attest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

// canonicalMarshal encodes v as canonical JSON: object keys sorted in plain
// byte order at every level, no insignificant whitespace, UTF-8, and numbers
// written exactly as decoded (json.Number preserves the original literal
// text). It supports the value shapes Statement builds: nil, bool,
// json.Number, string, map[string]any, and []any. A string or object key that
// is not valid UTF-8 is refused, never replaced, so two distinct byte strings
// cannot encode as one.
func canonicalMarshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// errInvalidUTF8 refuses a string canonicalMarshal cannot write unchanged.
var errInvalidUTF8 = errors.New("attest: canonical JSON string is not valid UTF-8")

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch value := v.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if value {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case json.Number:
		buf.WriteString(value.String())
		return nil
	case string:
		if !utf8.ValidString(value) {
			return errInvalidUTF8
		}
		writeCanonicalString(buf, value)
		return nil
	case map[string]any:
		return writeCanonicalObject(buf, value)
	case []any:
		return writeCanonicalArray(buf, value)
	default:
		return fmt.Errorf("attest: unsupported canonical JSON value of type %T", v)
	}
}

func writeCanonicalObject(buf *bytes.Buffer, object map[string]any) error {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buf.WriteByte('{')
	for index, key := range keys {
		if !utf8.ValidString(key) {
			return errInvalidUTF8
		}
		if index > 0 {
			buf.WriteByte(',')
		}
		writeCanonicalString(buf, key)
		buf.WriteByte(':')
		if err := writeCanonical(buf, object[key]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func writeCanonicalArray(buf *bytes.Buffer, array []any) error {
	buf.WriteByte('[')
	for index, value := range array {
		if index > 0 {
			buf.WriteByte(',')
		}
		if err := writeCanonical(buf, value); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

// lineSeparator and paragraphSeparator (U+2028, U+2029) are valid inside a
// JSON string but illegal inside a JavaScript string literal. They are
// escaped unconditionally, independent of the '<'/'>'/'&' HTML-escaping
// question, which writeCanonicalString never applies.
const (
	lineSeparator      rune = ' '
	paragraphSeparator rune = ' '
)

// writeCanonicalString writes s as a JSON string literal, escaping only what
// encoding/json requires (quote, backslash, and control characters below
// 0x20) plus U+2028 and U+2029. It never escapes '<', '>', or '&'.
func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if b >= 0x20 && b != '"' && b != '\\' {
				i++
				continue
			}
			if start < i {
				buf.WriteString(s[start:i])
			}
			switch b {
			case '\\', '"':
				buf.WriteByte('\\')
				buf.WriteByte(b)
			case '\n':
				buf.WriteString(`\n`)
			case '\r':
				buf.WriteString(`\r`)
			case '\t':
				buf.WriteString(`\t`)
			case '\b':
				buf.WriteString(`\b`)
			case '\f':
				buf.WriteString(`\f`)
			default:
				fmt.Fprintf(buf, `\u%04x`, b)
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == lineSeparator || r == paragraphSeparator {
			if start < i {
				buf.WriteString(s[start:i])
			}
			fmt.Fprintf(buf, `\u%04x`, r)
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf.WriteString(s[start:])
	}
	buf.WriteByte('"')
}
