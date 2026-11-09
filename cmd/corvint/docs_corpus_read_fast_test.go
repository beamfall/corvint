package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
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

// TestDCRV1413CorpusReadsWithoutRederivation covers every read row of GitHub #1598 (info, search,
// get, flow, coverage, locate and gaps, plus the corpus MCP first call): each answers from the
// verified artifact, so deleting a pinned input blob from the object store, which any
// rederivation must read, changes no receipt byte, while validate, --verify-rederive and
// docs_validate still rederive and refuse. --timings adds one stderr summary and leaves stdout
// unchanged (DCR-V1-410, DCR-V1-413, DCR-V1-415, DCR-V1-416).
func TestDCRV1413CorpusReadsWithoutRederivation(t *testing.T) {
	// A test binary's builder label is always `unrecorded`; trust it as an attested build would be
	// (DCR-V1-410).
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

// TestDCRV1432JoinedCorpusReusesVerifiedArtifact: a native read joined with --corpus admits the
// corpus through the verified read (DCR-V1-432); the delta current side is DCR-V1-460's. No read
// writes a file, persisted index or sidecar beside the artifact (DCR-V1-433).
func TestDCRV1432JoinedCorpusReusesVerifiedArtifact(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	writeCorpusFixture(t, root, rev, "cache")
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
}

// TestDCRV1431CorpusRefusalJoinsFreshness: a query read refused after its live freshness
// observation started, by its query index or by its arguments, cancels and joins the observation
// before it returns, so no observation outlives the read or writes its timing after the --timings
// summary. The observation blocks until it is cancelled. Run under -race.
func TestDCRV1431CorpusRefusalJoinsFreshness(t *testing.T) {
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	writeCorpusFixture(t, root, rev, "cache")
	var started, finished, cancelled atomic.Int32
	defer func(saved func(context.Context, string, *doccorpus.Artifact) (doccorpus.FreshnessOverlay, error)) {
		corpusLiveFreshness = saved
	}(corpusLiveFreshness)
	corpusLiveFreshness = func(ctx context.Context, _ string, _ *doccorpus.Artifact) (doccorpus.FreshnessOverlay, error) {
		started.Add(1)
		defer finished.Add(1)
		select {
		case <-ctx.Done():
			cancelled.Add(1)
			return doccorpus.FreshnessOverlay{}, ctx.Err()
		case <-time.After(time.Minute):
			return doccorpus.FreshnessOverlay{}, nil
		}
	}
	refusals := []struct {
		name  string
		args  []string
		lower bool
	}{
		{"index refusal", []string{"concept", "--query", "cache"}, true},
		{"claims argument error", []string{"claims"}, false},
		{"claims status filter", []string{"claims", "--state", "bogus"}, false},
	}
	for _, refusal := range refusals {
		restore := func() {}
		if refusal.lower {
			restore = doccorpus.SetIndexBoundsForTest(1, 1)
		}
		before := started.Load()
		code, out, stderr := corpusCLI(t, root, append(append([]string{"docs", "corpus"}, refusal.args...), "--artifact", "corpus.json", "--timings")...)
		restore()
		if code == 0 || out != "" {
			t.Fatalf("%s: %d %s", refusal.name, code, out)
		}
		if started.Load() != before+1 || finished.Load() != started.Load() || cancelled.Load() != started.Load() {
			t.Fatalf("%s: observation started %d, finished %d, cancelled %d before the read returned: %s", refusal.name, started.Load(), finished.Load(), cancelled.Load(), stderr)
		}
	}
}

// TestDCRV1467CorpusPreHeaderRefusalJoinsPrimary: a read refused before its artifact header
// arrives, because the artifact is missing or malformed, cancels and joins the primary
// observation it started at read start before the command returns, and starts no header
// freshness. It is serial because it replaces the package's primary observation.
func TestDCRV1467CorpusPreHeaderRefusalJoinsPrimary(t *testing.T) {
	root := taskContextRepository(t)
	if err := os.WriteFile(filepath.Join(root, "malformed.json"), []byte("{\"schema\": "), 0o600); err != nil {
		t.Fatal(err)
	}
	var started, finished, cancelled, header atomic.Int32
	defer doccorpus.SetPrimaryObservationForTest(func(ctx context.Context, _ string) (*contextindex.TreeObservation, error) {
		started.Add(1)
		defer finished.Add(1)
		select {
		case <-ctx.Done():
			cancelled.Add(1)
			// Ending slowly after the cancel makes a read that cancels without joining
			// return before finished is counted.
			time.Sleep(200 * time.Millisecond)
			return nil, ctx.Err()
		case <-time.After(time.Minute):
			return nil, nil
		}
	})()
	defer func(saved func(context.Context, string, *doccorpus.Artifact) (doccorpus.FreshnessOverlay, error)) {
		corpusLiveFreshness = saved
	}(corpusLiveFreshness)
	corpusLiveFreshness = func(context.Context, string, *doccorpus.Artifact) (doccorpus.FreshnessOverlay, error) {
		header.Add(1)
		return doccorpus.FreshnessOverlay{}, nil
	}
	for _, artifact := range []string{"missing.json", "malformed.json"} {
		before := started.Load()
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "concept", "--query", "cache", "--artifact", artifact)
		if code == 0 || out != "" {
			t.Fatalf("%s: %d %s", artifact, code, out)
		}
		if started.Load() != before+1 || finished.Load() != started.Load() || cancelled.Load() != started.Load() {
			t.Fatalf("%s: primary observation started %d, finished %d, cancelled %d before the read returned: %s", artifact, started.Load(), finished.Load(), cancelled.Load(), stderr)
		}
		if header.Load() != 0 {
			t.Fatalf("%s: header freshness started %d times on a pre-header refusal", artifact, header.Load())
		}
	}
}
