package wire

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// appendStringPerByte is appendString as it was before LGL-V0-802: one
// append per byte, a rune at a time above ASCII.
func appendStringPerByte(b []byte, s string) []byte {
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

// fastpathStrings are random strings over the bytes the two string paths
// treat differently: plain ASCII, every escape, controls, multi-byte UTF-8
// and invalid UTF-8.
func fastpathStrings(n int) []string {
	alphabet := []string{"a", "Z", "0", " ", "/", "\"", "\\", "\t", "\n", "\r", "\x00", "\x1f", "\x7f", "é", "€", "𝄞", "\xff", "\xe2\x22", "\xc3"}
	r := rand.New(rand.NewSource(802))
	out := []string{"", "plain", strings.Repeat("x", 5000), strings.Repeat("\\\"", 300)}
	for len(out) < n {
		var sb strings.Builder
		for k := r.Intn(40); k > 0; k-- {
			sb.WriteString(alphabet[r.Intn(len(alphabet))])
		}
		out = append(out, sb.String())
	}
	return out
}

// LGL-V0-802: the run-copying canonical string encoder writes exactly the
// bytes the per-byte encoder wrote, for valid and invalid UTF-8 alike.
func TestLGLV0802_EncodeStringMatchesPerByte(t *testing.T) {
	for _, s := range fastpathStrings(20000) {
		if got, want := string(appendString([]byte("p"), s)), string(appendStringPerByte([]byte("p"), s)); got != want {
			t.Fatalf("appendString(%q) = %q, want %q", s, got, want)
		}
	}
}

// LGL-V0-802: the parser's escape-free string path returns what the general
// path returns, and every refusal keeps its code, reason and position.
func TestLGLV0802_ParseStringFastPathMatchesGeneral(t *testing.T) {
	for _, s := range fastpathStrings(20000) {
		if !utf8.ValidString(s) {
			continue
		}
		doc := EncodeFile(ObjectValue(NewObject().Set("k", String(s)).Set("z", String(s+"!"))))
		v, err := Parse(doc)
		if hostile := checkHostile(s+"!", "", true); hostile != nil {
			if err == nil {
				t.Fatalf("Parse accepted hostile %q", s)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Parse(%q): %v", doc, err)
		}
		if got := v.Obj.Vals["k"].Str; got != s {
			t.Fatalf("Parse round trip of %q = %q", s, got)
		}
	}
	for _, tc := range []struct{ doc, want string }{
		{"\"abc\n", "MALFORMED: byte 0: unterminated string"},
		{"\"ab\x01c\"\n", "MALFORMED: byte 3: raw control byte 0x01 inside string"},
		{"\"ab\\qc\"\n", "MALFORMED: byte 5: invalid escape \\q"},
		{"\"ab\\/c\"\n", "non-canonical string escape form"},
		{"\"ab\\u0041\"\n", "non-canonical string escape form"},
		{"\"a\\tb\"\n", ""},
		{"\"\xe2\x80\xae\"\n", "MALFORMED: byte 0: hostile code point U+202E"},
	} {
		_, err := Parse([]byte(tc.doc))
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("Parse(%q): %v", tc.doc, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("Parse(%q) = %v, want %q", tc.doc, err, tc.want)
		}
	}
}

func BenchmarkLGLV0802_EncodeString(b *testing.B) {
	s := strings.Repeat("ticket body text with \"quotes\" and a newline\n", 40)
	b.SetBytes(int64(len(s)))
	buf := make([]byte, 0, 4096)
	for i := 0; i < b.N; i++ {
		buf = appendString(buf[:0], s)
	}
}

func BenchmarkLGLV0802_EncodeStringPerByte(b *testing.B) {
	s := strings.Repeat("ticket body text with \"quotes\" and a newline\n", 40)
	b.SetBytes(int64(len(s)))
	buf := make([]byte, 0, 4096)
	for i := 0; i < b.N; i++ {
		buf = appendStringPerByte(buf[:0], s)
	}
}
