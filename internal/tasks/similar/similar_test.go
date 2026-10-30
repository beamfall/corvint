package similar

import (
	"bytes"
	"strings"
	"testing"
)

// TestTCLV0002_RankIsDeterministicWithEvidence: ties break by ID, a doc that
// matches nothing is never a candidate, and each match names its fields.
func TestTCLV0002_RankIsDeterministicWithEvidence(t *testing.T) {
	docs := []Doc{
		{ID: "B-2", Title: "lease expires", Status: "OPEN"},
		{ID: "A-1", Title: "lease expires", Status: "OPEN"},
		{ID: "C-3", Title: "unrelated words", Status: "OPEN"},
	}
	got := NewIndex(docs).Rank("lease expires", nil, 10)
	if len(got) != 2 || got[0].Doc.ID != "A-1" || got[1].Doc.ID != "B-2" {
		t.Fatalf("rank: %+v", got)
	}
	if m := got[0].Matches; len(m) != 2 || m[0].Fields[0] != FieldTitle {
		t.Fatalf("matches: %+v", m)
	}
	if got := NewIndex(docs).Rank("lease", func(d *Doc) bool { return d.ID != "A-1" }, 10); len(got) != 1 || got[0].Doc.ID != "B-2" {
		t.Fatalf("filter: %+v", got)
	}
}

// TestTCLV0008_RecurrenceLines counts only well-formed line-leading markers.
func TestTCLV0008_RecurrenceLines(t *testing.T) {
	body := "x\n" + RecurrenceLine("2026-10-30", "a") + "\nsee Recurrence 2026-10-30: inline\n" + RecurrenceLine("2026-10-31", "b")
	if n := Recurrences(body); n != 2 {
		t.Fatalf("recurrences %d", n)
	}
	if ValidContext("") == "" || ValidContext("a\nb") == "" || ValidContext("ok") != "" {
		t.Fatal("ValidContext")
	}
}

// TestTCLV0006_DigestRoundTrip: the text digest parses back, and a missing
// header or a count mismatch is refused.
func TestTCLV0006_DigestRoundTrip(t *testing.T) {
	tree := strings.Repeat("a", 64)
	var buf bytes.Buffer
	in := []DigestEntry{{ID: "V1-1", Title: "one\ttab"}, {ID: "V1-2", Title: "two", Recurrences: 3}}
	if err := WriteDigest(&buf, tree, in); err != nil {
		t.Fatal(err)
	}
	d, err := ParseDigest(bytes.NewReader(buf.Bytes()))
	if err != nil || d.Tree != tree || len(d.Entries) != 2 || d.Entries[0].Title != "one tab" || d.Entries[1].Recurrences != 3 || d.Entries[1].Title != "two" {
		t.Fatalf("round trip: %+v %v", d, err)
	}
	for _, bad := range []string{"", "V1-1\tx\n", "# " + DigestSchema + " tree " + tree + " tickets 2\nV1-1\tx\n"} {
		if _, err := ParseDigest(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
