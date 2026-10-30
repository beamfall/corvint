package main

import (
	"bytes"
	"testing"
)

// TCP-V0-021 reports the successful loader observation on stderr only.
func TestBenchSnapshotTraceObservesHitWithoutChangingPacket(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	args := []string{"--root", root, "context", "--task", "does `Split` keep empty keys"}
	plain, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_BENCH_SNAPSHOT_TRACE="}, args...)
	if code != 0 || stderr != "" {
		t.Fatalf("plain: %d %s", code, stderr)
	}
	cold, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_BENCH_SNAPSHOT_TRACE=1"}, args...)
	if code != 0 || stderr != "corvint-bench-snapshot: hit=false\n" {
		t.Fatalf("cold: %d %s", code, stderr)
	}
	if _, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_BENCH_SNAPSHOT_TRACE=1"}, "--root", root, "index"); code != 0 {
		t.Fatalf("index: %d %s", code, stderr)
	}
	warm, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_BENCH_SNAPSHOT_TRACE=1"}, args...)
	if code != 0 || stderr != "corvint-bench-snapshot: hit=true\n" {
		t.Fatalf("warm: %d %s", code, stderr)
	}
	if !bytes.Equal(plain, cold) || !bytes.Equal(plain, warm) {
		t.Fatal("diagnostic changed packet")
	}
}
