package contextindex

import "testing"

func emptyIndex() *Index {
	return &Index{
		Revision: "r", Sources: map[string]Source{},
		Features: map[string]Record{}, Scenarios: map[string]Record{},
		Documents: map[string]Record{}, Markers: map[string][]Marker{},
		Imports: map[string]map[string]struct{}{},
	}
}

// TestReceiptOmitsExtractionWhenClean is the property that keeps this change
// byte-compatible with every existing receipt. A new top-level key that were
// always present would move the bytes of every receipt ever produced, including
// the pinned parity corpus, which contains no file in any of the newly admitted
// languages and therefore can never produce a note.
func TestReceiptOmitsExtractionWhenClean(t *testing.T) {
	built, err := receipt(emptyIndex(), "query", map[string]any{}, nil, 10, "")
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	if _, present := built["extraction"]; present {
		t.Fatalf("clean index produced an extraction key: %v", built["extraction"])
	}
}

// TestReceiptReportsExtractionNotes is the other half: when a walk did stop
// short, the receipt says so. Without this the truncation would be invisible,
// which is the defect this whole channel exists to prevent.
func TestReceiptReportsExtractionNotes(t *testing.T) {
	index := emptyIndex()
	index.ExtractionNotes = []ExtractionNote{
		{"a.rs", noteSymbolCap},
		{"b.swift", noteUnterminatedComment},
	}
	built, err := receipt(index, "query", map[string]any{}, nil, 10, "")
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	summary, present := built["extraction"].(map[string]any)
	if !present {
		t.Fatalf("noted index produced no extraction key: %v", built)
	}
	if summary["count"] != 2 {
		t.Errorf("count = %v, want 2", summary["count"])
	}
	samples, ok := summary["samples"].([]any)
	if !ok || len(samples) != 2 {
		t.Fatalf("samples = %v, want 2", summary["samples"])
	}
	first, _ := samples[0].(map[string]any)
	if first["path"] != "a.rs" || first["reason"] != noteSymbolCap {
		t.Errorf("first sample = %v", first)
	}
}

// TestExtractionSamplesAreBounded keeps one pathological repository from
// pushing an unbounded list into every receipt, matching how exclusions are
// capped. The count still reports the true total, so the bound narrows the
// sample without misstating the scale.
func TestExtractionSamplesAreBounded(t *testing.T) {
	index := emptyIndex()
	for counter := 0; counter < maxExclusionSamples*3; counter++ {
		index.ExtractionNotes = append(index.ExtractionNotes, ExtractionNote{"f.rs", noteSymbolCap})
	}
	summary, noted := extractionSummary(index)
	if !noted {
		t.Fatal("expected a summary")
	}
	if summary["count"] != maxExclusionSamples*3 {
		t.Errorf("count = %v, want %d", summary["count"], maxExclusionSamples*3)
	}
	if samples := summary["samples"].([]any); len(samples) != maxExclusionSamples {
		t.Errorf("samples = %d, want %d", len(samples), maxExclusionSamples)
	}
}

// TestSortNotesIsDeterministic guards the merge across the concurrent source
// chunks: notes are gathered per worker, so an unsorted merge would let the
// receipt's sample vary between runs over an identical tree.
func TestSortNotesIsDeterministic(t *testing.T) {
	notes := []ExtractionNote{
		{"b.rs", noteSymbolCap},
		{"a.rs", noteUnterminatedString},
		{"a.rs", noteLineCap},
	}
	sortNotes(notes)
	// Path first, then reason text: "file ends inside..." sorts before
	// "line cap reached...", so the two a.rs notes come back in that order.
	want := []ExtractionNote{
		{"a.rs", noteUnterminatedString},
		{"a.rs", noteLineCap},
		{"b.rs", noteSymbolCap},
	}
	for index, note := range want {
		if notes[index] != note {
			t.Errorf("notes[%d] = %v, want %v", index, notes[index], note)
		}
	}
}
