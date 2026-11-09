package main

import (
	"context"
	"io"
	"os"
	"testing"
)

// BenchmarkTaskContext measures the task-context packet against a real
// repository so a claim about its cost always has a reproducible baseline.
//
//	CORVINT_PERF_CORPUS=/path/to/repo go test ./cmd/corvint \
//	  -run XXX -bench BenchmarkTaskContext -benchtime 5x -cpuprofile cpu.out
//
// It measures warm, in-process iterations, so it attributes cost; it is not
// GPK-V0-016 evidence, which requires the fresh-process interleaved runner.

// perfCorpus names the repository under test. It is supplied out of band so the
// benchmark measures a representative tree rather than a synthetic fixture.
func perfCorpus(b *testing.B) string {
	b.Helper()
	root := os.Getenv("CORVINT_PERF_CORPUS")
	if root == "" {
		b.Skip("set CORVINT_PERF_CORPUS to a repository root")
	}
	return root
}

// perfChangedPath names a path the corpus tracks inside a non-root Go package,
// which is what the impact-backed events require of their input.
func perfChangedPath() string {
	if path := os.Getenv("CORVINT_PERF_CHANGED_PATH"); path != "" {
		return path
	}
	return "cmd/corvint/main.go"
}

func BenchmarkTaskContext(b *testing.B) {
	options := taskContextOptions{
		root: perfCorpus(b), task: "change the context command", subject: perfChangedPath(), limit: taskContextDefaultLimit,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if code := runTaskContext(context.Background(), options, io.Discard, io.Discard); code != 0 {
			b.Fatalf("context exited %d", code)
		}
	}
}
