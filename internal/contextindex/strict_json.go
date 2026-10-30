package contextindex

import (
	"unicode/utf16"
	"unicode/utf8"
)

// The strict reader below accepts exactly the documents internal/cem/wire.Parse
// accepts, which is the reading the application map's manifest decoder applies.
// contextindex is inside the offline corvint-tasks source archive closure
// (PUB-V0-012, PUB-V0-013) and internal/cem/wire is not, so the rules are kept
// here rather than imported; TestStrictJSONMatchesWireParse pins the parity.

const (
	strictJSONMaxDepth   = 64
	strictJSONMaxInteger = 9007199254740991
)

// strictJSON reports whether text is one JSON document without duplicate
// members, the reading the application map's manifest decoder enforces.
func strictJSON(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	s := &strictJSONReader{data: text}
	s.skipSpace()
	if !s.value(0) {
		return false
	}
	s.skipSpace()
	return s.pos == len(s.data)
}

type strictJSONReader struct {
	data string
	pos  int
}

func (s *strictJSONReader) skipSpace() {
	for s.pos < len(s.data) {
		switch s.data[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

func (s *strictJSONReader) value(depth int) bool {
	if depth > strictJSONMaxDepth || s.pos >= len(s.data) {
		return false
	}
	switch s.data[s.pos] {
	case '{':
		return s.object(depth)
	case '[':
		return s.array(depth)
	case '"':
		_, ok := s.str()
		return ok
	case 't':
		return s.literal("true")
	case 'f':
		return s.literal("false")
	case 'n':
		return s.literal("null")
	default:
		return s.number()
	}
}

func (s *strictJSONReader) literal(word string) bool {
	if len(s.data)-s.pos < len(word) || s.data[s.pos:s.pos+len(word)] != word {
		return false
	}
	s.pos += len(word)
	return true
}

// number admits only a non-negative integer within the wire integer range.
func (s *strictJSONReader) number() bool {
	start := s.pos
	for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
		s.pos++
	}
	if s.pos < len(s.data) {
		switch s.data[s.pos] {
		case '.', 'e', 'E', '+', '-':
			return false
		}
	}
	digits := s.data[start:s.pos]
	if len(digits) == 0 || len(digits) > 16 || (len(digits) > 1 && digits[0] == '0') {
		return false
	}
	var number int64
	for i := 0; i < len(digits); i++ {
		number = number*10 + int64(digits[i]-'0')
	}
	return number <= strictJSONMaxInteger
}

// str reads one string and returns its decoded value, refusing raw control
// bytes, unknown escapes and unpaired surrogates.
func (s *strictJSONReader) str() (string, bool) {
	if s.pos >= len(s.data) || s.data[s.pos] != '"' {
		return "", false
	}
	s.pos++
	var out []byte
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		switch {
		case c == '"':
			s.pos++
			return string(out), true
		case c == '\\':
			decoded, ok := s.escape()
			if !ok {
				return "", false
			}
			out = append(out, decoded...)
		case c < 0x20:
			return "", false
		default:
			out = append(out, c)
			s.pos++
		}
	}
	return "", false
}

func (s *strictJSONReader) escape() ([]byte, bool) {
	if s.pos+1 >= len(s.data) {
		return nil, false
	}
	switch marker := s.data[s.pos+1]; marker {
	case '"', '\\', '/':
		s.pos += 2
		return []byte{marker}, true
	case 'b', 'f', 'n', 'r', 't':
		s.pos += 2
		return []byte{map[byte]byte{'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}[marker]}, true
	case 'u':
	default:
		return nil, false
	}
	first, ok := s.hex4(s.pos + 2)
	if !ok {
		return nil, false
	}
	s.pos += 6
	if !utf16.IsSurrogate(first) {
		return utf8.AppendRune(nil, first), true
	}
	if first >= 0xDC00 || len(s.data)-s.pos < 6 || s.data[s.pos] != '\\' || s.data[s.pos+1] != 'u' {
		return nil, false
	}
	second, ok := s.hex4(s.pos + 2)
	if !ok {
		return nil, false
	}
	combined := utf16.DecodeRune(first, second)
	if combined == utf8.RuneError {
		return nil, false
	}
	s.pos += 6
	return utf8.AppendRune(nil, combined), true
}

func (s *strictJSONReader) hex4(start int) (rune, bool) {
	if start+4 > len(s.data) {
		return 0, false
	}
	var value rune
	for i := start; i < start+4; i++ {
		c := s.data[i]
		value <<= 4
		switch {
		case c >= '0' && c <= '9':
			value |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			value |= rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			value |= rune(c-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func (s *strictJSONReader) array(depth int) bool {
	s.pos++
	s.skipSpace()
	if s.pos < len(s.data) && s.data[s.pos] == ']' {
		s.pos++
		return true
	}
	for {
		if !s.value(depth + 1) {
			return false
		}
		s.skipSpace()
		if s.pos >= len(s.data) {
			return false
		}
		switch s.data[s.pos] {
		case ',':
			s.pos++
			s.skipSpace()
		case ']':
			s.pos++
			return true
		default:
			return false
		}
	}
}

func (s *strictJSONReader) object(depth int) bool {
	s.pos++
	s.skipSpace()
	if s.pos < len(s.data) && s.data[s.pos] == '}' {
		s.pos++
		return true
	}
	seen := map[string]bool{}
	for {
		s.skipSpace()
		key, ok := s.str()
		if !ok || seen[key] {
			return false
		}
		seen[key] = true
		s.skipSpace()
		if s.pos >= len(s.data) || s.data[s.pos] != ':' {
			return false
		}
		s.pos++
		s.skipSpace()
		if !s.value(depth + 1) {
			return false
		}
		s.skipSpace()
		if s.pos >= len(s.data) {
			return false
		}
		switch s.data[s.pos] {
		case ',':
			s.pos++
		case '}':
			s.pos++
			return true
		default:
			return false
		}
	}
}
