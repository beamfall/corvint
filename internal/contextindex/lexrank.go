package contextindex

import "math"

// The exported lexical ranking primitives below are the context compiler's
// own tokeniser and BM25 term score (lexicalHits), shared so another ranking
// of short documents, such as the Tasks duplicate check (TCL-V0-003), uses
// the same retrieval method instead of a second one.

// LexicalK1 and LexicalB are lexicalHits' BM25 parameters.
const (
	LexicalK1 = 1.2
	LexicalB  = 0.3
)

// LexicalTaskTerms returns the distinct query terms the context compiler
// derives from a task: camel- and snake-split lowercase runs of at least two
// bytes, plus each whole camel identifier, minus the task stop words.
func LexicalTaskTerms(text string) []string { return taskLexicalTerms(text) }

// LexicalStopWord reports whether term is one of the task stop words.
func LexicalStopWord(term string) bool {
	_, stop := taskStopWords[term]
	return stop
}

// LexicalIDF is lexicalHits' inverse document frequency of a term with
// postings documents in a corpus of corpus documents.
func LexicalIDF(corpus, postings int) float64 {
	return math.Log(1 + (float64(corpus)-float64(postings)+0.5)/(float64(postings)+0.5))
}

// LexicalTermScore is lexicalHits' length-normalised BM25 body credit of a
// term with frequency tf in a document of length tokens, average the corpus
// mean length.
func LexicalTermScore(idf, tf, length, average float64) float64 {
	norm := LexicalK1 * (1 - LexicalB + LexicalB*length/average)
	return idf * tf * (LexicalK1 + 1) / (tf + norm)
}

// LexicalCounter is LexicalTermCounts with its interning reused across
// texts, for a caller that tokenises many; it is not safe for concurrent
// use.
type LexicalCounter struct{ counter *termCounter }
