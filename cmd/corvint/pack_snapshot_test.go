package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// packReadInvocations are the read verbs the pack must answer exactly as the gob snapshot and
// the build do: impact over each tracked fixture path and over a pair, and the instruction load
// set, which reads instruction bodies (decision 0597 removed the task packet these once used).
func packReadInvocations(root string) [][]string {
	invocations := [][]string{
		{"--root", root, "impact", "cache/demux.go", "cache/demux_test.go"},
		{"--root", root, "context", "--subject", "cache/demux.go", "--instruction-host", "codex",
			"--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "default"},
	}
	for _, path := range []string{"cache/demux.go", "cache/demux_test.go", "go.mod", "testing/features.yaml"} {
		invocations = append(invocations, []string{"--root", root, "impact", path})
	}
	return invocations
}

// TestPackSnapshotReadsWithoutChangingAByte pins IDX-SNAP-V0-015's identity row: on every read
// invocation the pack hit emits the bytes the build and the gob hit emit.
func TestPackSnapshotReadsWithoutChangingAByte(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	invocations := packReadInvocations(root)
	run := func(snapshotFormat string) []string {
		t.Helper()
		return candidatePackets(t, invocations, []string{"CORVINT_SNAPSHOT_FORMAT=" + snapshotFormat})
	}
	cold := run("")
	receiptOut, stderr, code := runCandidateWithEnvironment(t, "", []string{"CORVINT_SNAPSHOT_FORMAT=pack"}, "--root", root, "index")
	if code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr)
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptOut, &receipt); err != nil {
		t.Fatal(err)
	}
	packBytes, _ := receipt["pack_bytes"].(float64)
	if packBytes <= 0 {
		t.Fatalf("receipt names no pack: %s", receiptOut)
	}
	packPaths, err := filepath.Glob(filepath.Join(contextindex.SnapshotDirectory(root), "*.aip"))
	if err != nil || len(packPaths) != 1 {
		t.Fatalf("pack paths = %v, err = %v", packPaths, err)
	}
	if _, err := os.Stat(packPaths[0]); err != nil {
		t.Fatal(err)
	}
	for position, pack := range run("pack") {
		if pack != cold[position] {
			t.Fatalf("pack hit differs from the build for %v:\n%s\n%s", invocations[position], cold[position], pack)
		}
	}
	// A corrupt body is refused when a read reads it: the verb recompiles
	// through the loader that refuses the whole pack and answers from the gob.
	packed, err := os.ReadFile(packPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	body := bytes.Index(packed, []byte("func Split(key string) string { return key }"))
	if body < 0 {
		t.Fatal("the pack holds no demux.go body")
	}
	packed[body] ^= 0xff
	if err := os.WriteFile(packPaths[0], packed, 0o600); err != nil {
		t.Fatal(err)
	}
	for position, refused := range run("pack") {
		if refused != cold[position] {
			t.Fatalf("a refused pack body changed the packet for %v:\n%s\n%s", invocations[position], cold[position], refused)
		}
	}
	for position, gob := range run("") {
		if gob != cold[position] {
			t.Fatalf("gob hit differs from the build for %v:\n%s\n%s", invocations[position], cold[position], gob)
		}
	}
	if _, err := os.Stat(contextindex.SnapshotDirectory(root)); err != nil {
		t.Fatal(err)
	}
}

// candidatePackets runs the candidate once per invocation under overrides and
// returns each stdout in invocation order. The first runs alone, so it meets
// the store a serial loop leaves it and settles any cache it fills; the rest
// are read verbs over that same state, run on a bounded pool (V1-2740: 40
// serial helper processes took 150-200 s on a loaded CI host). Any nonzero
// exit fails the test, naming the first failing invocation.
func candidatePackets(t *testing.T, invocations [][]string, overrides []string) []string {
	t.Helper()
	packets := make([]string, len(invocations))
	failures := make([]string, len(invocations))
	one := func(position int) {
		stdout, stderr, code, err := runCandidateCapture("", overrides, invocations[position]...)
		switch {
		case err != nil:
			failures[position] = err.Error()
		case code != 0:
			failures[position] = fmt.Sprintf("exit %d: %s", code, stderr)
		}
		packets[position] = string(stdout)
	}
	if len(invocations) > 0 {
		one(0)
	}
	if len(invocations) > 1 && failures[0] == "" {
		next := make(chan int)
		var workers sync.WaitGroup
		for range min(runtime.GOMAXPROCS(0), 8, len(invocations)-1) {
			workers.Go(func() {
				for position := range next {
					one(position)
				}
			})
		}
		for position := 1; position < len(invocations); position++ {
			next <- position
		}
		close(next)
		workers.Wait()
	}
	for position, failure := range failures {
		if failure != "" {
			t.Fatalf("%v: %s", invocations[position], failure)
		}
	}
	return packets
}
