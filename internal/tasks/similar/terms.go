package similar

import "github.com/Beamfall/corvint/internal/contextindex"

// This is the package's only import of the context compiler (TCL-V0-003,
// decision 0397 addendum): the tokeniser and BM25 term score are the
// compiler's own, so the duplicate check and context retrieval rank the
// same way.

// queryTerms are the compiler's task terms of a query.
func queryTerms(text string) []string { return contextindex.LexicalTaskTerms(text) }

// fieldTerms calls fn for each of a text's compiler body terms with its
// frequency, without the task stop words, which a query never holds.
type fieldTerms struct{ counter *contextindex.LexicalCounter }

func newFieldTerms() fieldTerms { return fieldTerms{counter: contextindex.NewLexicalCounter()} }

func (f fieldTerms) each(text string, fn func(term string, tf uint32)) {
	f.counter.Each(text, func(term string, tf uint32) {
		if !contextindex.LexicalStopWord(term) {
			fn(term, tf)
		}
	})
}

func idf(corpus, postings int) float64 { return contextindex.LexicalIDF(corpus, postings) }

func bodyScore(idf, tf, length, average float64) float64 {
	return contextindex.LexicalTermScore(idf, tf, length, average)
}

func fieldScore(idf float64) float64 { return contextindex.LexicalFieldScore(idf) }
