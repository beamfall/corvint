package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// corpusInfoValidation is the validation an info receipt reports.
func corpusInfoValidation(t *testing.T, out string) string {
	t.Helper()
	var receipt struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &receipt); err != nil || len(receipt.Results) == 0 {
		t.Fatalf("info receipt: %v %s", err, out)
	}
	value, _ := receipt.Results[0]["validation"].(string)
	return value
}

// TestDCPV1413CorpusReadsWithoutRederivation covers every read row of GitHub #1598 (info, search,
// get, flow, coverage, locate and gaps, plus the corpus MCP first call): each answers from the
// verified artifact, so deleting a pinned input blob from the object store, which any
// rederivation must read, changes no receipt byte, while validate, --verify-rederive and
// docs_validate still rederive and refuse. --timings adds one stderr summary and leaves stdout
// unchanged (DCP-V1-410, DCP-V1-413, DCP-V1-415, DCP-V1-416).
func TestDCPV1413CorpusReadsWithoutRederivation(t *testing.T) {
	// A test binary's builder label is always `unrecorded`; trust it as an attested build would be
	// (DCP-V1-410).
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	writeCorpusFixture(t, root, rev, "cache")
	reads := [][]string{
		{"info"},
		{"search", "--query", "Split"},
		{"get", "--id", "native:cache/demux.go"},
		{"flow", "--id", "native:cache/demux.go"},
		{"coverage"},
		{"locate", "--path", "cache/demux.go"},
		{"gaps"},
	}
	read := func(args ...string) (int, string, string) {
		return corpusCLI(t, root, append(append([]string{"docs", "corpus"}, args...), "--artifact", "corpus.json")...)
	}
	before := map[string]string{}
	for _, args := range reads {
		code, out, stderr := read(args...)
		if code != 0 {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
		before[strings.Join(args, " ")] = out
	}
	if got := corpusInfoValidation(t, before["info"]); got != doccorpus.ValidationVerified {
		t.Fatalf("info validation %q", got)
	}
	for _, args := range [][]string{{"validate"}, {"info", "--verify-rederive"}} {
		code, out, stderr := read(args...)
		if code != 0 || corpusInfoValidation(t, out) != doccorpus.ValidationRederived {
			t.Fatalf("%v: %d %s %s", args, code, stderr, out)
		}
	}
	code, timed, stderr := read("search", "--query", "Split", "--timings")
	if code != 0 || timed != before["search --query Split"] {
		t.Fatalf("--timings changed stdout: %d %s", code, stderr)
	}
	if !strings.HasPrefix(stderr, "corvint docs corpus timings: validation=artifact-verified ") || strings.Count(stderr, "\n") != 1 {
		t.Fatalf("timings summary: %q", stderr)
	}
	for _, phase := range []string{"parse_ms=", "verify_ms=", "freshness_ms=", "index_ms=", "query_ms=", "encode_ms=", "total_ms="} {
		if !strings.Contains(stderr, phase) {
			t.Fatalf("timings summary lacks %s: %q", phase, stderr)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "corpus-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := doccorpus.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	blob := m.Inputs[0].Blob
	if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	for _, args := range reads {
		code, out, stderr := read(args...)
		if code != 0 || out != before[strings.Join(args, " ")] {
			t.Fatalf("%v after deleting a pinned blob: %d %s", args, code, stderr)
		}
	}
	for _, args := range [][]string{{"validate"}, {"info", "--verify-rederive"}} {
		if code, _, _ := read(args...); code != 2 {
			t.Fatalf("%v read a deleted pinned blob: %d", args, code)
		}
	}
	registry, startup := corpusbridge.New(root, "corpus.json")
	if startup != nil {
		t.Fatal(startup)
	}
	_, text, failure, protocol, timing := registry.CallTimed(context.Background(), "corvint.docs_search", []byte(`{"query":"Split"}`))
	if failure != nil || protocol != nil || text != before["search --query Split"] || timing.Load != "first-call" {
		t.Fatalf("MCP first call: %v %v %+v", failure, protocol, timing)
	}
	if _, _, failure, _ := registry.Call(context.Background(), "corvint.docs_validate", []byte(`{}`)); failure == nil {
		t.Fatal("docs_validate read a deleted pinned blob")
	}
}

// TestDCPV1432JoinedCorpusAndDeltaReuseVerifiedArtifact: a native read joined with --corpus
// admits the corpus through the verified read, and the current side of docs corpus delta answers
// from the verified artifact, so deleting a pinned input blob, which any rederivation must read,
// changes no delta byte, while delta --verify-rederive still rederives and refuses (DCP-V1-432).
// No read writes a file, persisted index or sidecar beside the artifact (DCP-V1-433).
func TestDCPV1432JoinedCorpusAndDeltaReuseVerifiedArtifact(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	artifact := writeCorpusFixture(t, root, rev, "cache")
	cemWrite(t, root, "previous.json", artifact)
	listing := func() string {
		t.Helper()
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, entry.Name()+" "+info.ModTime().String())
		}
		return strings.Join(names, "\n")
	}
	files := listing()
	defer func() {
		if after := listing(); after != files {
			t.Errorf("reads changed the repository root:\n%s\nwant\n%s", after, files)
		}
	}()
	code, joined, stderr := corpusCLI(t, root, "affected", "--corpus=corpus.json")
	if code != 0 {
		t.Fatalf("joined read: %d %s", code, stderr)
	}
	var native struct {
		Documentation struct {
			Receipt json.RawMessage `json:"receipt"`
		} `json:"documentation"`
	}
	if err := json.Unmarshal([]byte(joined), &native); err != nil {
		t.Fatal(err)
	}
	if got := corpusInfoValidation(t, string(native.Documentation.Receipt)); got != doccorpus.ValidationVerified {
		t.Fatalf("joined corpus validation %q", got)
	}
	delta := []string{"docs", "corpus", "delta", "--previous", "previous.json", "--artifact", "corpus.json"}
	code, before, stderr := corpusCLI(t, root, delta...)
	if code != 0 || !strings.Contains(before, "current artifact was admitted by the verified read") {
		t.Fatalf("delta: %d %s %s", code, stderr, before)
	}
	raw, err := os.ReadFile(filepath.Join(root, "corpus-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := doccorpus.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	blob := m.Inputs[0].Blob
	if err := os.Remove(filepath.Join(root, ".git", "objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	if code, after, stderr := corpusCLI(t, root, delta...); code != 0 || after != before {
		t.Fatalf("delta after deleting a pinned blob: %d %s", code, stderr)
	}
	if code, _, _ := corpusCLI(t, root, append(delta, "--verify-rederive")...); code != 2 {
		t.Fatalf("delta --verify-rederive read a deleted pinned blob: %d", code)
	}
}
