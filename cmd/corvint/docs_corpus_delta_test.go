package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCPV1360DeltaCLI drives `docs corpus delta` and `docs corpus carry-forward` through the
// CLI: both current-side forms page the same worklist, nothing is written, and the input forms
// and an undeclared provider are refused.
func TestDCPV1360DeltaCLI(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	if err := os.Rename(filepath.Join(root, "corpus.json"), filepath.Join(root, "previous.json")); err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "cache/demux.go", "package cache\n\n// Split returns key.\nfunc Split(key string) string { return key }\n")
	cemGit(t, root, "add", "cache")
	cemGit(t, root, "commit", "-qm", "change")
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all")

	receipts := map[string]doccorpus.DeltaReceipt{}
	for _, current := range [][]string{{"--artifact", "corpus.json"}, {"--manifest", "corpus-input.json"}} {
		args := append([]string{"docs", "corpus", "delta", "--previous", "previous.json", "--limit", "1"}, current...)
		code, out, stderr := corpusCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("%v: %d %s", current, code, stderr)
		}
		var receipt doccorpus.DeltaReceipt
		if err := json.Unmarshal([]byte(out), &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Schema != doccorpus.DeltaSchema || receipt.Limit != 1 || receipt.Current.Source != strings.TrimPrefix(current[0], "--") {
			t.Fatalf("%v receipt header: %+v", current, receipt)
		}
		receipts[current[0]] = receipt
	}
	byArtifact, byManifest := receipts["--artifact"], receipts["--manifest"]
	if !reflect.DeepEqual(byArtifact.Entries, byManifest.Entries) || !reflect.DeepEqual(byArtifact.Counts, byManifest.Counts) {
		t.Fatalf("current-side forms disagree:\n%+v\n%+v", byArtifact, byManifest)
	}
	if byArtifact.Counts[doccorpus.DeltaUncovered] == 0 || len(byArtifact.Entries) != 1 || byArtifact.Entries[0].Class != doccorpus.DeltaUncovered || !strings.HasPrefix(byArtifact.Entries[0].ID, "cache/demux.go:") {
		t.Fatalf("changed region not uncovered: %+v", byArtifact)
	}
	if got := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); got != status {
		t.Fatalf("delta mutated the worktree:\n%s\n%s", status, got)
	}

	for _, refused := range [][]string{
		{"delta", "--previous", "previous.json"},
		{"delta", "--previous", "previous.json", "--artifact", "corpus.json", "--manifest", "corpus-input.json"},
		{"delta", "--previous", "previous.json", "--artifact", "corpus.json", "--provider", "adapter"},
		{"carry-forward", "--previous", "previous.json", "--artifact", "corpus.json"},
		{"carry-forward", "--previous", "previous.json", "--artifact", "corpus.json", "--provider", "missing"},
		{"carry-forward", "--previous", "previous.json", "--artifact", "corpus.json", "--provider", "missing", "--limit", "1"},
	} {
		if code, out, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, refused...)...); code != 2 || out != "" {
			t.Fatalf("%v accepted: %d %s", refused, code, out)
		}
	}
}
