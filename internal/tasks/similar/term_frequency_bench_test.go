package similar

import (
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// V1-3273: the index takes each doc field's term frequencies in one pass
// through a counter it reuses across docs (fieldTerms), and ranking reads
// the body frequency the posting kept; nothing rebuilds them per candidate.
// The per-call form the ticket describes -- a fresh frequency map from
// LexicalTermCounts for every field -- is the baseline these compare with.

func corpusFields(b *testing.B) []string {
	b.Helper()
	var fields []string
	for _, r := range loadCorpus(b) {
		d := recordDoc(r)
		fields = append(fields, d.ID, d.Title, d.Body)
		fields = append(fields, d.Labels...)
		fields = append(fields, d.Paths...)
	}
	return fields
}

func BenchmarkV13273_TermFrequencies(b *testing.B) {
	fields := corpusFields(b)
	b.Run("map-per-call", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, f := range fields {
				for term, tf := range contextindex.LexicalTermCounts(f) {
					if !contextindex.LexicalStopWord(term) {
						_ = tf
					}
				}
			}
		}
	})
	b.Run("reused-counter", func(b *testing.B) {
		b.ReportAllocs()
		terms := newFieldTerms()
		for i := 0; i < b.N; i++ {
			for _, f := range fields {
				terms.each(f, func(string, uint32) {})
			}
		}
	})
}

// TestV13273_ReusedCounterMatchesPerCallCounts pins that the reused counter
// yields exactly the per-call frequencies, stop words aside, for every
// corpus field.
func TestV13273_ReusedCounterMatchesPerCallCounts(t *testing.T) {
	terms := newFieldTerms()
	for _, r := range loadCorpus(t) {
		d := recordDoc(r)
		for _, f := range append([]string{d.ID, d.Title, d.Body}, append(d.Labels, d.Paths...)...) {
			want := map[string]uint32{}
			for term, tf := range contextindex.LexicalTermCounts(f) {
				if !contextindex.LexicalStopWord(term) {
					want[term] = tf
				}
			}
			got := map[string]uint32{}
			terms.each(f, func(term string, tf uint32) {
				if _, dup := got[term]; dup {
					t.Fatalf("%s: term %q reported twice", d.ID, term)
				}
				got[term] = tf
			})
			if len(got) != len(want) {
				t.Fatalf("%s: %d terms, want %d", d.ID, len(got), len(want))
			}
			for term, tf := range want {
				if got[term] != tf {
					t.Fatalf("%s: %q tf %d, want %d", d.ID, term, got[term], tf)
				}
			}
		}
	}
}
