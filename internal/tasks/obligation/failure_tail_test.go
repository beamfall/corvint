package obligation

import (
	"strings"
	"testing"
)

// TestTCDV0165_FailureTail: the tail is the last n non-blank lines oldest
// first, cleaned and bounded; the failing line is the last actionable one,
// else the last line; a line matching the secret screen is withheld.
func TestTCDV0165_FailureTail(t *testing.T) {
	token := "sk" + "-" + strings.Repeat("q", 24)
	log := "one\n\n\x1b[31mFAIL x\x1b[0m\ntoken " + token + "\n" + strings.Repeat("z", 300) + "\ndone\n\n"
	tail, failing := FailureTail(log, 4)
	want := []string{"FAIL x", ExcerptWithheld, strings.Repeat("z", MaxExcerptBytes), "done"}
	if strings.Join(tail, "|") != strings.Join(want, "|") || failing != "FAIL x" {
		t.Fatalf("tail %q failing %q", tail, failing)
	}
	if tail, failing := FailureTail("a\nb\n", 5); strings.Join(tail, "|") != "a|b" || failing != "b" {
		t.Fatalf("tail %q failing %q", tail, failing)
	}
	if tail, failing := FailureTail("\n \n", 5); tail != nil || failing != "" {
		t.Fatalf("blank log: tail %q failing %q", tail, failing)
	}
	// A secret beyond the excerpt bound is withheld although the bounded
	// excerpt no longer contains it.
	long := strings.Repeat("y", MaxExcerptBytes) + " " + token
	if tail, _ := FailureTail(long, 1); len(tail) != 1 || tail[0] != ExcerptWithheld {
		t.Fatalf("long secret line: %q", tail)
	}

}

// TestTCDV0165_FailureTailKeyBlockBeforeTail: a private-key block whose
// BEGIN line lies before the tail window withholds every body line in the
// tail, terminated or cut off, and quotes the lines after its END marker.
func TestTCDV0165_FailureTailKeyBlockBeforeTail(t *testing.T) {
	body := "MIIEkeyBODY" + strings.Repeat("Q", 52)
	block := "-----BEGIN " + "RSA PRIVATE KEY-----\n" + strings.Repeat(body+"\n", 30)
	end := "-----END " + "RSA PRIVATE KEY-----\n"
	for name, c := range map[string]struct {
		log      string
		withheld int
		failing  string
	}{
		"terminated":   {"build\n" + block + end + "after\nFAIL x\n", 18, "FAIL x"},
		"unterminated": {"build\n" + block, 20, ExcerptWithheld},
	} {
		tail, failing := FailureTail(c.log, 20)
		if len(tail) != 20 || failing != c.failing {
			t.Fatalf("%s: tail %q failing %q", name, tail, failing)
		}
		withheld := 0
		for _, line := range tail {
			if strings.Contains(line, "keyBODY") || strings.Contains(line, "PRIVATE KEY") {
				t.Fatalf("%s: key material emitted: %q", name, tail)
			}
			if line == ExcerptWithheld {
				withheld++
			}
		}
		if withheld != c.withheld {
			t.Fatalf("%s: %d withheld lines, want %d: %q", name, withheld, c.withheld, tail)
		}
	}
}
