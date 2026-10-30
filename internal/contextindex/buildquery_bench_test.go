package contextindex

import (
	"context"
	"os"
	"testing"
)

// BenchmarkBuildQueryRepository measures the query build against a real
// repository named by CORVINT_BENCH_REPO, because query latency is a function of
// repository size and the fixtures are too small to expose it. It skips when
// the variable is unset so the ordinary test run stays hermetic.
func BenchmarkBuildQueryRepository(b *testing.B) {
	root := os.Getenv("CORVINT_BENCH_REPO")
	if root == "" {
		b.Skip("set CORVINT_BENCH_REPO to a repository path to run this benchmark")
	}
	task := "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"
	for b.Loop() {
		index, err := BuildQuery(context.Background(), root, task)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := EvalQuery(context.Background(), index, task, 10, nil); err != nil {
			b.Fatal(err)
		}
	}
}
