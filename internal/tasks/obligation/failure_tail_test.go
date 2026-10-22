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
