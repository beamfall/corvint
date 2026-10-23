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
