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

// TestDCPV1421DeltaViewsCLI drives the delta views through the CLI: --uncovered changed keeps
// only changed regions and counts the uncited subjects, --anchorless list and --format jsonl
// are admitted, and an invalid mode, a jsonl stream with paging, and a view option on
// carry-forward are refused (DCP-V1-421..423).
func TestDCPV1421DeltaViewsCLI(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	run := func(args ...string) string {
		t.Helper()
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "delta", "--previous", "corpus.json", "--artifact", "corpus.json"}, args...)...)
		if code != 0 {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
		return out
	}
	var all, changed doccorpus.DeltaReceipt
	if err := json.Unmarshal([]byte(run("--limit", "256", "--anchorless", "list")), &all); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(run("--limit", "256", "--uncovered", "changed")), &changed); err != nil {
		t.Fatal(err)
	}
	subjects := 0
	for _, e := range all.Entries {
		if e.Class == doccorpus.DeltaUncovered && e.RecordKind == "subject" {
			subjects++
		}
	}
	for _, e := range changed.Entries {
		if e.Class == doccorpus.DeltaUncovered {
			t.Fatalf("no-op delta under --uncovered changed listed %+v", e)
		}
	}
	if changed.UncoveredSubjectsOmitted != subjects {
		t.Fatalf("uncited subjects: listed %d, omitted %d", subjects, changed.UncoveredSubjectsOmitted)
	}
	lines := strings.Split(strings.TrimSuffix(run("--format", "jsonl", "--anchorless", "list"), "\n"), "\n")
	var header doccorpus.DeltaStreamHeader
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil || header.Schema != doccorpus.DeltaStreamSchema || header.Total != all.Total || len(lines) != all.Total+1 {
		t.Fatalf("jsonl stream: %+v %v (%d lines, paged total %d)", header, err, len(lines), all.Total)
	}
	for _, refused := range [][]string{
		{"delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--format", "jsonl", "--limit", "1"},
		{"delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--format", "jsonl", "--offset", "0"},
		{"delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--format", "csv"},
		{"delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--uncovered", "some"},
		{"delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--anchorless", "none"},
		{"carry-forward", "--previous", "corpus.json", "--artifact", "corpus.json", "--provider", "adapter", "--uncovered", "changed"},
	} {
		if code, out, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, refused...)...); code != 2 || out != "" {
			t.Fatalf("%v accepted: %d %s", refused, code, out)
		}
	}
}
