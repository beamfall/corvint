// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-2180: a complete sliced source run can generate and replay a different
// output partition. Partial logs must never pass by using the output count.
func TestAFPV02180RebalanceSourceAndOutputShardCounts(t *testing.T) {
	root, head := repoWith(t, allowBig, twoSlices)
	fakeList(t, map[string][]string{"example.org/big": everyTest})
	source := logs(t,
		lifecycle("example.org/big", 15, "TestA", "TestB", "TestC")+lifecycle("example.org/small", 5, "TestS"),
		lifecycle("example.org/big", 20, catchAll...))
	args := []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "4", "--log-shards", "2"}
	var notes bytes.Buffer
	code, err := run(context.Background(), "generate", append(args, source...), nil, &notes)
	if code != 0 || err != nil || !strings.Contains(notes.String(), "35000ms into 4 slices") {
		t.Fatalf("generate code=%d err=%v notes=%s", code, err, &notes)
	}
	candidate, err := os.ReadFile(filepath.Join(root, slicesPath))
	if err != nil {
		t.Fatal(err)
	}
	candidatePath := write(t, filepath.Join(t.TempDir(), "candidate.json"), string(candidate))
	// Replay must combine with the source's two slices, then place four slices.
	write(t, filepath.Join(root, slicesPath), twoSlices)
	var out bytes.Buffer
	replayArgs := []string{"--root", root, "--shards", "4", "--log-shards", "2", "--slices", candidatePath}
	code, err = run(context.Background(), "replay", append(replayArgs, source...), &out, &notes)
	if code != 0 || err != nil || !strings.Contains(out.String(), "ideal 10.0s over 4 shards") ||
		!strings.Contains(out.String(), "1 whole packages, 4 test slices") {
		t.Fatalf("replay code=%d err=%v output=%s", code, err, &out)
	}
	for _, mode := range []string{"generate", "replay"} {
		for _, count := range []string{"1", "3", "65", "-1"} {
			bad := []string{"--root", root, "--shards", "4", "--log-shards", count}
			code, err = run(context.Background(), mode, append(bad, source...), &out, &notes)
			if code != 2 || err == nil {
				t.Fatalf("%s log-shards=%s: code=%d err=%v", mode, count, code, err)
			}
		}
	}
}
