package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCXV1360DeltaCLI drives `docs corpus delta` and `docs corpus carry-forward` through the
// CLI: both current-side forms page the same worklist, nothing is written, and the input forms
// and an undeclared provider are refused.
func TestDCXV1360DeltaCLI(t *testing.T) {
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

// TestDCXV1425DeltaUnavailableCheckoutCLI drives `docs corpus delta` over a retained artifact
// whose declared additional checkout has disappeared: the delta still classifies, the missing
// repository's anchors are unresolved with the reason, the primary repository's anchors carry
// forward, and the receipt says the current side was not rederived (DCX-V1-425).
func TestDCXV1425DeltaUnavailableCheckoutCLI(t *testing.T) {
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	m, err := doccorpus.Inventory(context.Background(), root, rev, "cache", "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(t.TempDir(), "lib")
	libDoc := "# Lib\n\nThe lib parses quokka values.\n"
	cemWrite(t, lib, "doc.md", libDoc)
	cemGit(t, lib, "init", "-q")
	cemGit(t, lib, "add", ".")
	cemGit(t, lib, "commit", "-qm", "lib")
	libRev := cemGit(t, lib, "rev-parse", "HEAD")
	rel, err := filepath.Rel(root, lib)
	if err != nil {
		t.Fatal(err)
	}
	rootDoc := "package cache\n\nfunc Split(key string) string { return key }\n"
	anchor := func(repository, revision, path, blob, text string, line int) doccorpus.Anchor {
		excerpt := strings.SplitAfter(text, "\n")[line-1]
		return doccorpus.Anchor{Repository: repository, Revision: revision, Path: path, Blob: blob, SHA256: doccorpus.Digest([]byte(text)), Start: line, End: line, SpanSHA256: doccorpus.Digest([]byte(excerpt)), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}
	}
	evidence := func(a doccorpus.Anchor) doccorpus.Evidence {
		return doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{a}, Limitations: []string{"delta fixture assertion"}}
	}
	libBlob := cemGit(t, lib, "rev-parse", libRev+":doc.md")
	record := doccorpus.ProviderRecord{Schema: doccorpus.ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository,
		Subjects: []doccorpus.Subject{
			{ID: "adapter:lib", Kind: "capability", Name: "Lib quokka parser", Provider: "adapter", Evidence: evidence(anchor(libRev, libRev, "doc.md", libBlob, libDoc, 3))},
			{ID: "adapter:split", Kind: "capability", Name: "Cache split", Provider: "adapter", Evidence: evidence(anchor(m.Repository.ID, rev, "cache/demux.go", cemGit(t, root, "rev-parse", rev+":cache/demux.go"), rootDoc, 3))},
		},
		Claims: []doccorpus.Claim{}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{},
		Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "fixture"}},
	}
	data, err := doccorpus.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "evidence/provider.json", string(data))
	cemGit(t, root, "add", "evidence")
	cemGit(t, root, "commit", "-qm", "provider")
	providerRev := cemGit(t, root, "rev-parse", "HEAD")
	m.AdditionalRepositories = []doccorpus.AdditionalRepository{{ID: libRev, Revision: libRev, Root: filepath.ToSlash(rel)}}
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRev, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "evidence/provider.json", Revision: providerRev}, doccorpus.Scope{Path: "doc.md", Revision: libRev})
	m.Inputs = append(m.Inputs,
		doccorpus.Input{Path: "evidence/provider.json", Revision: providerRev, Blob: cemGit(t, root, "rev-parse", providerRev+":evidence/provider.json"), SHA256: doccorpus.Digest(data), Provider: "adapter", Purpose: "provider"},
		doccorpus.Input{Path: "doc.md", Revision: libRev, Blob: libBlob, SHA256: doccorpus.Digest([]byte(libDoc)), Provider: "adapter", Purpose: "evidence"})
	manifest, err := doccorpus.Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "corpus-input.json", string(manifest))
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json")
	if code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	cemWrite(t, root, "corpus.json", out)
	if err := os.Rename(lib, lib+"-gone"); err != nil {
		t.Fatal(err)
	}

	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "delta", "--previous", "corpus.json", "--artifact", "corpus.json", "--limit", "256", "--uncovered", "changed")
	if code != 0 {
		t.Fatalf("delta with an unavailable checkout: %d %s", code, stderr)
	}
	var receipt doccorpus.DeltaReceipt
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	entries := map[string]doccorpus.DeltaEntry{}
	for _, e := range receipt.Entries {
		entries[e.ID] = e
	}
	if e := entries["adapter:lib"]; e.Class != doccorpus.DeltaUnresolved || len(e.Anchors) != 1 || e.Anchors[0].Repository != libRev || e.Anchors[0].Current != nil || !strings.Contains(e.Reason, "checkout is unavailable") {
		t.Fatalf("anchor in the unavailable checkout: %+v", e)
	}
	if e := entries["adapter:split"]; e.Class != doccorpus.DeltaCarryForward || len(e.Anchors) != 1 || e.Anchors[0].Current == nil {
		t.Fatalf("primary anchor did not classify: %+v", e)
	}
	if !strings.Contains(strings.Join(receipt.Limitations, "\n"), "current artifact was not rederived because the checkout of additional repository "+libRev+" is unavailable") {
		t.Fatalf("receipt does not disclose the unverified current side: %v", receipt.Limitations)
	}
	// Carry-forward emits records for the next build, and a manifest cannot be built without the
	// checkout, so both still refuse.
	for _, refused := range [][]string{
		{"carry-forward", "--previous", "corpus.json", "--artifact", "corpus.json", "--provider", "adapter"},
		{"delta", "--previous", "corpus.json", "--manifest", "corpus-input.json"},
	} {
		if code, out, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, refused...)...); code == 0 || out != "" {
			t.Fatalf("%v accepted without the checkout: %d %s", refused, code, out)
		}
	}
}

// TestDCXV1421DeltaViewsCLI drives the delta views through the CLI: --uncovered changed keeps
// only changed regions and counts the uncited subjects, --anchorless list and --format jsonl
// are admitted, and an invalid mode, a jsonl stream with paging, and a view option on
// carry-forward are refused (DCX-V1-421..423).
func TestDCXV1421DeltaViewsCLI(t *testing.T) {
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

// TestDCRV1460DeltaCLIVerifiedRead drives `docs corpus delta --artifact` through the CLI: the
// default path admits the current artifact through the verified read and runs no build, its
// paged and jsonl output is byte-identical to --verify-rederive, --timings writes one phase line
// with the comparison and encode phases to stderr and leaves stdout unchanged, and a current
// artifact the verified read refuses is refused (DCR-V1-460..462).
func TestDCRV1460DeltaCLIVerifiedRead(t *testing.T) {
	// A test binary's builder label is always `unrecorded`; trust it as an attested build would be
	// (DCR-V1-410). Not parallel: it counts builds.
	defer doccorpus.TrustUnattestedBuilderForTest()()
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	if err := os.Rename(filepath.Join(root, "corpus.json"), filepath.Join(root, "previous.json")); err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "cache/demux.go", "package cache\n\n// Split returns key.\nfunc Split(key string) string { return key }\n")
	cemGit(t, root, "add", "cache")
	cemGit(t, root, "commit", "-qm", "change")
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	delta := func(args ...string) (string, string, int64) {
		t.Helper()
		before := doccorpus.BuildsForTest()
		code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "delta", "--previous", "previous.json", "--artifact", "corpus.json"}, args...)...)
		if code != 0 {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
		return out, stderr, doccorpus.BuildsForTest() - before
	}
	for _, view := range [][]string{{"--limit", "256"}, {"--format", "jsonl"}, {"--format", "jsonl", "--anchorless", "list", "--uncovered", "changed"}} {
		verified, _, builds := delta(view...)
		if builds != 0 {
			t.Fatalf("%v: delta ran %d builds on the verified path", view, builds)
		}
		rederived, _, builds := delta(append(view, "--verify-rederive")...)
		if builds == 0 {
			t.Fatalf("%v: --verify-rederive ran no build", view)
		}
		if verified != rederived || !strings.Contains(verified, `"uncovered"`) {
			t.Fatalf("%v: verified delta differs from --verify-rederive:\n%s\n%s", view, verified, rederived)
		}
		for _, mode := range [][]string{nil, {"--verify-rederive"}} {
			timed, stderr, _ := delta(append(append(view, mode...), "--timings")...)
			validation := doccorpus.ValidationVerified
			if mode != nil {
				validation = doccorpus.ValidationRederived
			}
			if timed != verified || !strings.HasPrefix(stderr, "corvint docs corpus timings: validation="+validation+" ") || strings.Count(stderr, "\n") != 1 {
				t.Fatalf("%v %v --timings: stdout changed or summary %q", view, mode, stderr)
			}
			for _, phase := range []string{"parse_ms=", "verify_ms=", "freshness_ms=", "index_ms=", "query_ms=", "encode_ms=", "compare_ms=", "total_ms="} {
				if !strings.Contains(stderr, phase) {
					t.Fatalf("timings summary lacks %s: %q", phase, stderr)
				}
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "corpus.json", " "+string(raw))
	for _, mode := range [][]string{nil, {"--verify-rederive"}} {
		if code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus", "delta", "--previous", "previous.json", "--artifact", "corpus.json"}, mode...)...); code != 2 || out != "" || !strings.Contains(stderr, "corpus-refused") {
			t.Fatalf("%v: tampered current side not refused: %d %s %s", mode, code, out, stderr)
		}
	}
	for _, flag := range []string{"--verify-rederive", "--timings"} {
		if code, out, _ := corpusCLI(t, root, "docs", "corpus", "carry-forward", "--previous", "previous.json", "--artifact", "previous.json", "--provider", "adapter", flag); code != 2 || out != "" {
			t.Fatalf("carry-forward accepted %s: %d %s", flag, code, out)
		}
	}
}
