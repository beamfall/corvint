package wire

import (
	"strings"
	"testing"
)

// TestLGLV0004_ASCIIFastPathAgreesWithHostileRune covers LGL-V0-004
// (proposed). The ASCII scan that lets checkHostile and the string parser
// skip rune decoding accepts exactly the ASCII bytes hostileRune accepts, in
// both modes, alone and inside a longer string; a string with any byte at or
// above 0x80 still takes the full check, which keeps refusing C1 and bidi
// controls and invalid UTF-8.
func TestLGLV0004_ASCIIFastPathAgreesWithHostileRune(t *testing.T) {
	for _, prose := range []bool{false, true} {
		for b := 0; b < 0x80; b++ {
			want := !hostileRune(rune(b), prose)
			for _, s := range []string{string(rune(b)), "ab" + string(rune(b)) + "cd"} {
				if got := plainASCII(s, prose); got != want {
					t.Fatalf("plainASCII(%q, %v) = %v, hostileRune accepts %v", s, prose, got, want)
				}
				if err := checkHostile(s, "x", prose); (err == nil) != want {
					t.Fatalf("checkHostile(%q, %v) = %v, hostileRune accepts %v", s, prose, err, want)
				}
			}
		}
		for s, ok := range map[string]bool{
			"caf\u00e9":                        true,
			"a\u0085b":                         false,
			"a\u202eb":                         false,
			"a\ufeffb":                         false,
			"a" + "\xff" + "b":                 false,
			strings.Repeat("a", 64) + "\u0080": false,
		} {
			if plainASCII(s, prose) {
				t.Fatalf("plainASCII(%q) accepted a non-ASCII string", s)
			}
			if err := checkHostile(s, "x", prose); (err == nil) != ok {
				t.Fatalf("checkHostile(%q, %v) = %v, want accept %v", s, prose, err, ok)
			}
		}
	}
	// The parser's string path keeps the position in its refusal.
	if _, err := Parse([]byte("{\"a\":\"x\\u0085\"}\n")); err == nil || !strings.Contains(err.Error(), "byte 5") || !strings.Contains(err.Error(), "U+0085") {
		t.Fatalf("hostile string refusal: %v", err)
	}
	if _, err := Parse([]byte("{\"a\":\"x\\u0001\"}\n")); err == nil || !strings.Contains(err.Error(), "U+0001") {
		t.Fatalf("hostile ASCII control refusal: %v", err)
	}
}
