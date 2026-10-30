package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

// The harness event path has two cost classes, and the preregistered perf
// manifest measures only one of them. session-start with any startSource other
// than "compact", post-tool, stop and session-end answer from the Git probe
// alone; user-prompt, file-change and compact session-start compile the full
// context index. These benchmarks cover all three shapes against a real
// repository so a claim about the event path always has a reproducible
// baseline behind it.
//
//	CORVINT_PERF_CORPUS=/path/to/repo go test ./cmd/corvint \
//	  -run XXX -bench BenchmarkHarness -benchtime 5x -cpuprofile cpu.out
//
// They measure warm, in-process iterations, so they attribute cost; they are not
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

func benchmarkHarnessEvent(b *testing.B, event, input string) {
	arguments := []string{
		"--root", perfCorpus(b), "harness", "event",
		"--host", "claude-code", "--host-version", "1.0.0",
		"--surface", "plugin", "--adapter-version", "1.0.0",
		"--event", event, "--input", "-",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if code := runContext(context.Background(), arguments, bytes.NewReader([]byte(input)), io.Discard, io.Discard); code != 0 {
			b.Fatalf("%s exited %d", event, code)
		}
	}
}

// Probe-only events: no index, so the cost is the Git probe bracket.

func BenchmarkHarnessSessionStart(b *testing.B) {
	benchmarkHarnessEvent(b, "session-start", `{"startSource":"startup"}`)
}

func BenchmarkHarnessPostTool(b *testing.B) {
	benchmarkHarnessEvent(b, "post-tool", `{"changedPaths":["`+perfChangedPath()+`"]}`)
}

func BenchmarkHarnessStop(b *testing.B) {
	benchmarkHarnessEvent(b, "stop", `{"stopHookActive":false}`)
}

// Query-index event: the narrow build, capped at 500 ms p95.

func BenchmarkHarnessUserPrompt(b *testing.B) {
	benchmarkHarnessEvent(b, "user-prompt", `{"task":"where is the session heartbeat written"}`)
}

// Full-index events. Both are classed non-query by GPK-V0-017 and neither is
// preregistered in conformance/perf-v0/manifest.json, so their thresholds are
// NOT_RUN rather than met.

func BenchmarkHarnessFileChange(b *testing.B) {
	benchmarkHarnessEvent(b, "file-change", `{"paths":["`+perfChangedPath()+`"]}`)
}

func BenchmarkHarnessCompactSessionStart(b *testing.B) {
	benchmarkHarnessEvent(b, "session-start", `{"startSource":"compact"}`)
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
