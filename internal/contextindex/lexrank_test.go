package contextindex

import (
	"math"
	"testing"
)

// TestTCLV0003_ScoreMatchesContextCompiler pins the exported ranking
// primitives to lexicalHits' inline BM25 so the Tasks duplicate check and
// the context compiler cannot drift (TCL-V0-003).
func TestTCLV0003_ScoreMatchesContextCompiler(t *testing.T) {
	const k1, b = 1.2, 0.3
	corpus := 3257.0
	for _, postings := range []int{1, 7, 300, 3257} {
		want := math.Log(1 + (corpus-float64(postings)+0.5)/(float64(postings)+0.5))
		if got := LexicalIDF(int(corpus), postings); got != want {
			t.Fatalf("idf(%d) = %v, want %v", postings, got, want)
		}
		for _, tf := range []float64{1, 3} {
			for _, length := range []float64{4, 40, 400} {
				norm := k1 * (1 - b + b*length/37.5)
				want := want * tf * (k1 + 1) / (tf + norm)
				if got := LexicalTermScore(LexicalIDF(int(corpus), postings), tf, length, 37.5); math.Abs(got-want) > 1e-12 {
					t.Fatalf("score(%d,%v,%v) = %v, want %v", postings, tf, length, got, want)
				}
			}
		}
	}
	if got := LexicalTaskTerms("How does stripFinalNewline handle the heavy_sh reason?"); !equalStrings(got, taskLexicalTerms("How does stripFinalNewline handle the heavy_sh reason?")) {
		t.Fatalf("LexicalTaskTerms = %v", got)
	}
	if !LexicalStopWord("the") || LexicalStopWord("heavy") {
		t.Fatal("stop words differ from taskStopWords")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
