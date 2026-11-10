package main

import (
	"strings"
	"testing"
)

// TestDCXV1563DeltaCurrentSideIsDecidedByPresence: for delta and carry-forward, an empty
// --manifest= beside --artifact, or an empty --artifact= or --manifest= alone, is refused with
// exit 2 and no output, as build refuses it (DCX-V1-495, DCX-V1-563; GitHub #1667, V1-2543).
func TestDCXV1563DeltaCurrentSideIsDecidedByPresence(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all")
	for _, op := range [][]string{{"delta"}, {"carry-forward", "--provider", "adapter"}} {
		for _, current := range [][]string{{"--manifest=", "--artifact=corpus.json"}, {"--artifact=corpus.json", "--manifest="}, {"--artifact="}, {"--manifest="}, {"--manifest=corpus-input.json", "--artifact="}} {
			args := append(append([]string{"docs", "corpus"}, op...), "--previous", "corpus.json")
			code, out, stderr := corpusCLI(t, root, append(args, current...)...)
			if code != 2 || out != "" || !strings.Contains(stderr, "requires exactly one of --artifact or --manifest") {
				t.Fatalf("%v %v: %d %q %s", op, current, code, out, stderr)
			}
		}
	}
	if got := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); got != status {
		t.Fatalf("a refused delta mutated the worktree:\n%s\n%s", status, got)
	}
}
