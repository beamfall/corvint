package contextindex

import (
	"math/rand"
	"runtime"
	"strconv"
	"testing"
)

// invertBenchCorpus is a deterministic synthetic corpus: many sources over a
// shared Zipf-skewed vocabulary, the shape that makes the posting builders'
// per-worker and merge key maps grow.
func invertBenchCorpus() []sourceTerms {
	const sources, vocabulary, perSource = 4096, 40000, 400
	random := rand.New(rand.NewSource(1))
	zipf := rand.NewZipf(random, 1.1, 1, vocabulary-1)
	corpus := make([]sourceTerms, sources)
	for source := range corpus {
		terms := make(map[string]uint32, perSource)
		words := make(map[string]struct{}, perSource)
		for count := 0; count < perSource; count++ {
			key := "term" + strconv.FormatUint(zipf.Uint64(), 10)
			terms[key]++
			words[key] = struct{}{}
		}
		corpus[source] = sourceTerms{terms: terms, words: words}
	}
	return corpus
}

// invertBenchWorkers runs the inversion sharded across the CPUs and again on
// one worker, so shard building and shard merging are both measured.
func invertBenchWorkers(b *testing.B, corpus []sourceTerms, invert func(workers int) termPostings) {
	sharded := min(runtime.NumCPU(), max(len(corpus)/sourcesPerWorker, 1))
	for name, workers := range map[string]int{"sharded": sharded, "serial": 1} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				runtime.KeepAlive(invert(workers))
			}
		})
	}
}

func BenchmarkInvertCounts(b *testing.B) {
	corpus := invertBenchCorpus()
	invertBenchWorkers(b, corpus, func(workers int) termPostings { return invertCountsWithWorkers(corpus, workers) })
}

func BenchmarkInvertPresence(b *testing.B) {
	corpus := invertBenchCorpus()
	pick := func(entry sourceTerms) map[string]struct{} { return entry.words }
	invertBenchWorkers(b, corpus, func(workers int) termPostings { return invertPresenceWithWorkers(corpus, workers, pick) })
}
