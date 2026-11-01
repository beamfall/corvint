package similar

import (
	"fmt"
	"strings"
	"testing"
)

// TestTCLV0990_OneRareTermIsNotDuplicate: a candidate that reaches the
// similarity and term floors only through its body (the V1-3517 shape: a long
// unrelated ticket, or one shared rare term among common words) is ranked but
// is not a duplicate; one matched title term restores the verdict
// (TCL-V0-990).
func TestTCLV0990_OneRareTermIsNotDuplicate(t *testing.T) {
	body := Candidate{Similarity: 0.63, Matches: []Match{
		{Term: "clone", IDF: 4.71, Fields: []string{FieldBody}},
		{Term: "run", IDF: 1.05, Fields: []string{FieldBody}},
		{Term: "fails", IDF: 1.67, Fields: []string{FieldBody}},
	}}
	if body.Duplicate() || body.Anchored() {
		t.Fatalf("body-only matches: duplicate=%v anchored=%v", body.Duplicate(), body.Anchored())
	}
	titled := body
	titled.Matches = append([]Match{{Term: "hook", IDF: 3.99, Fields: []string{FieldTitle, FieldBody}}}, body.Matches...)
	if !titled.Duplicate() {
		t.Fatalf("a title-anchored candidate must stay a duplicate: %+v", titled)
	}

	// Index level: common words fill most tickets; the unrelated ticket holds
	// the query's words only in a long body.
	var docs []Doc
	for i := 0; i < 40; i++ {
		docs = append(docs, Doc{ID: fmt.Sprintf("F-%02d", i), Title: "hook run fails outside layout", Body: fmt.Sprintf("filler topic%d", i)})
	}
	long := strings.Repeat("relay cache frames codec vectors waiver ", 20) + "the hook run fails outside the layout of a clone"
	docs = append(docs, Doc{ID: "CORE-0832", Title: "Represent relay cache frames", Body: long})
	got := NewIndex(docs).Rank("hook run fails outside the layout of a clone", func(d *Doc) bool { return d.ID == "CORE-0832" }, 5)
	if len(got) != 1 {
		t.Fatalf("CORE-0832 must still be ranked: %+v", got)
	}
	if got[0].Similarity < DuplicateSimilarity || len(got[0].Matches) < DuplicateMinTerms {
		t.Fatalf("fixture must reach the similarity and term floors, else it tests nothing: %s", got[0].Reason())
	}
	if got[0].Duplicate() {
		t.Fatalf("a body-only match reached a duplicate verdict: %s", got[0].Reason())
	}
}
