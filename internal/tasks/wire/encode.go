package wire

import "unicode/utf8"

// Encode returns the canonical body of a value (WQO §4.1): closed objects with
// keys in UTF-8 byte order, no insignificant whitespace, only `\t`, `\n`,
// `\r`, `\"` and `\\` as short escapes, `\u00xx` (lowercase) for any other
// control, never `\/`, never an optional `\u` escape, raw UTF-8 otherwise.
// The trailing LF is not part of the body; see EncodeFile.
func Encode(v Value) []byte {
	return appendValue(nil, v)
}

// EncodeFile returns the on-disk and transport form: canonical body plus
// exactly one LF (§2).
func EncodeFile(v Value) []byte {
	return append(Encode(v), '\n')
}

func appendValue(b []byte, v Value) []byte {
	switch v.Kind {
	case KindNull:
		return append(b, "null"...)
	case KindBool:
		if v.Bool {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case KindString:
		return appendString(b, v.Str)
	case KindArray:
		b = append(b, '[')
		for i, e := range v.Arr {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendValue(b, e)
		}
		return append(b, ']')
	case KindObject:
		b = append(b, '{')
		if v.Obj != nil {
			for i, k := range v.Obj.SortedKeys() {
				if i > 0 {
					b = append(b, ',')
				}
				b = appendString(b, k)
				b = append(b, ':')
				b = appendValue(b, v.Obj.Vals[k])
			}
		}
		return append(b, '}')
	}
	return append(b, "null"...)
}

const hexdigits = "0123456789abcdef"

func appendString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch c {
			case '"':
				b = append(b, '\\', '"')
			case '\\':
				b = append(b, '\\', '\\')
			case '\t':
				b = append(b, '\\', 't')
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			default:
				if c < 0x20 {
					b = append(b, '\\', 'u', '0', '0', hexdigits[c>>4], hexdigits[c&0xF])
				} else {
					b = append(b, c)
				}
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b = append(b, s[i:i+size]...)
		i += size
	}
	return append(b, '"')
}
