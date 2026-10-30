package pythongrammar

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchCorpus loads every real Python file in the repository so the allocation
// benchmarks measure the same input shape the impact command walks.
func benchCorpus(b *testing.B) [][]byte {
	b.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	corpus := make([][]byte, 0, 128)
	bytesTotal := 0
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		corpus = append(corpus, body)
		bytesTotal += len(body)
		return nil
	}
	for _, dir := range []string{"src", "tests", "tools", "experiments", "conformance"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), walk)
	}
	if len(corpus) == 0 {
		b.Skip("no python corpus")
	}
	b.Logf("corpus files=%d bytes=%d", len(corpus), bytesTotal)
	return corpus
}

type discardSink struct{ count int }

func (sink *discardSink) Add(kind, subject, predicate, value string, line, column int) Failure {
	sink.count++
	return ""
}

// BenchmarkLexCorpus measures lexer allocation over the whole corpus.
func BenchmarkLexCorpus(b *testing.B) {
	corpus := benchCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		for _, body := range corpus {
			LexPython312(body)
		}
	}
}

// BenchmarkParseCorpus measures end-to-end grammar allocation over the corpus.
func BenchmarkParseCorpus(b *testing.B) {
	corpus := benchCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		for _, body := range corpus {
			sink := &discardSink{}
			ParsePython312Subset("bench.py", body, sink)
		}
	}
}

// BenchmarkStringPrefix isolates the string-prefix classifier.
func BenchmarkStringPrefix(b *testing.B) {
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		nativeStringPrefix("")
		nativeStringPrefix("f")
		nativeStringPrefix("rb")
		nativeStringPrefix("zz")
	}
}
