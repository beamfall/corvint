package contextindex

import (
	"os"
	"testing"
)

// The one-shot tokenisers' allocation readings (CRK-V0-001..003, V1-3273):
// a role or evidence line is a few dozen bytes, so the interner a long-lived
// worker sizes for a source window must not be allocated for it.
const benchmarkLine = "func (compiler *taskContextCompiler) lexicalHits() []lexicalHit { // BM25 posting walk"

var benchmarkWanted = map[string]struct{}{"compiler": {}, "lexical": {}, "hits": {}, "posting": {}, "absent": {}}

func BenchmarkCountTermsLine(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = countTerms(benchmarkLine)
	}
}

func BenchmarkLineTokens(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = lineTokens(benchmarkLine, benchmarkWanted)
	}
}

func BenchmarkRoleTerms(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = roleTerms(benchmarkLine, benchmarkWanted)
	}
}

func BenchmarkScanWordsFile(b *testing.B) {
	data, err := os.ReadFile("lexrank.go")
	if err != nil {
		b.Fatal(err)
	}
	text := string(data)
	b.ReportAllocs()
	for b.Loop() {
		_ = scanWords(text)
	}
}
