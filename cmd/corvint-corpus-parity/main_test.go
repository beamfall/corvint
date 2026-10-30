package main

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/corpusindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

// corpusFixture commits one source file and one adoption provider record, then
// writes the canonical /2 corpus to corpus.json inside the repository root.
func corpusFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus parity command fixture")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	source := "package fixture\nfunc Count() int { return 1 }\n"
	for path, data := range map[string]string{"go.mod": "module example.invalid/corpus\n\ngo 1.27.1\n", "src/value.go": source} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700)
		if e := os.WriteFile(filepath.Join(root, path), []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	rev := git(t, root, "rev-parse", "HEAD")
	m, e := doccorpus.Inventory(context.Background(), root, rev, "src", "2026-09-30T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	input := m.Inputs[0]
	anchor := doccorpus.Anchor{Repository: m.Repository.ID, Revision: rev, Path: input.Path, Blob: input.Blob, SHA256: input.SHA256, Start: 2, End: 2, SpanSHA256: doccorpus.Digest([]byte("func Count() int { return 1 }\n")), Symbol: "Count", Authority: "external-provider", Kind: "declared", Reason: "fixture declared claim"}
	ev := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{anchor}, Limitations: []string{"fixture declaration; semantic validity unknown"}}
	p := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{{ID: "adapter:feature", Kind: "capability", Name: "count", Provider: "adapter", Evidence: ev}}, Claims: []doccorpus.Claim{{ID: "adapter:c1", Subject: "adapter:feature", Text: "count returns one", Provider: "adapter", Evidence: ev}}, Relations: []doccorpus.Relation{}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "claims", State: "present", Reason: "fixture claims"}, {Name: "subjects", State: "present", Reason: "fixture feature"}}}
	b, e := doccorpus.Encode(p)
	if e != nil {
		t.Fatal(e)
	}
	_ = os.Mkdir(filepath.Join(root, "evidence"), 0700)
	if e = os.WriteFile(filepath.Join(root, "evidence/provider.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "provider")
	pr := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "adapter", Kind: "records", Version: "1", Revision: pr, Record: "evidence/provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "evidence/provider.json", Revision: pr})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "evidence/provider.json", Revision: pr, Blob: git(t, root, "rev-parse", pr+":evidence/provider.json"), SHA256: doccorpus.Digest(b), Provider: "adapter", Purpose: "provider"})
	a, e := doccorpus.Build(context.Background(), root, m)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := doccorpus.Encode(a)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "corpus.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	return root
}

func invoke(args ...string) (int, []byte, string) {
	var out, errout bytes.Buffer
	code := run(context.Background(), args, &out, &errout)
	return code, out.Bytes(), errout.String()
}

func TestCorpusParityCommandEndToEnd(t *testing.T) {
	t.Run("DCP-V1-043 build query and recorded parity through the command boundary", func(t *testing.T) {
		root := corpusFixture(t)
		code, first, stderr := invoke("--mode", "build", "--root", root, "--artifact", "corpus.json")
		if code != 0 || len(first) == 0 || stderr != "" {
			t.Fatalf("build: %d %q", code, stderr)
		}
		code, second, _ := invoke("--mode", "build", "--root", root, "--artifact", "corpus.json")
		if code != 0 || !bytes.Equal(first, second) {
			t.Fatal("two command builds from identical inputs differ")
		}
		dir := t.TempDir()
		index := filepath.Join(dir, "index.json")
		if e := os.WriteFile(index, first, 0600); e != nil {
			t.Fatal(e)
		}
		pin := doccorpus.Digest(first)
		request := filepath.Join(dir, "query.json")
		if e := os.WriteFile(request, []byte(`{"operation":"get","id":"adapter:feature"}`), 0600); e != nil {
			t.Fatal(e)
		}
		code, receiptBytes, stderr := invoke("--mode", "query", "--index", index, "--sha256", pin, "--request", request)
		if code != 0 || stderr != "" {
			t.Fatalf("query: %d %q", code, stderr)
		}
		var receipt doccorpus.Receipt
		if e := stdjson.Unmarshal(receiptBytes, &receipt); e != nil || receipt.Envelope == nil || receipt.Envelope.SourceValidation != "index-digest-validated; source-revalidation-unavailable" || len(receipt.Results) == 0 {
			t.Fatalf("query receipt lacks trust envelope or result: %v %s", e, receiptBytes)
		}
		var answer doccorpus.Receipt
		if e := stdjson.Unmarshal(receiptBytes, &answer); e != nil {
			t.Fatal(e)
		}
		changed := answer
		changed.Freshness = "fresh"
		miss := doccorpus.Request{Operation: "get", ID: "adapter:absent"}
		recording := corpusindex.Recording{Schema: corpusindex.RecordingSchema, Questions: []corpusindex.Question{
			{ID: "q-equal", Tool: "get_feature", Request: doccorpus.Request{Operation: "get", ID: "adapter:feature"}, Answer: answer},
			{ID: "q-semantics", Tool: "get_feature", Request: doccorpus.Request{Operation: "get", ID: "adapter:feature"}, Answer: changed},
			{ID: "q-search", Tool: "search_docs", Request: doccorpus.Request{Operation: "search", Query: "count"}, Answer: answer},
			{ID: "q-miss", Tool: "get_feature", Request: miss, Answer: answer},
		}}
		recorded, e := corpusindex.Encode(recording)
		if e != nil {
			t.Fatal(e)
		}
		questions := filepath.Join(dir, "recording.json")
		if e = os.WriteFile(questions, recorded, 0600); e != nil {
			t.Fatal(e)
		}
		code, reportBytes, stderr := invoke("--mode", "parity", "--index", index, "--sha256", pin, "--request", questions)
		if code != 0 || stderr != "" {
			t.Fatalf("parity: %d %q", code, stderr)
		}
		var report corpusindex.ParityReport
		if e = stdjson.Unmarshal(reportBytes, &report); e != nil {
			t.Fatal(e)
		}
		if report.IndexSHA256 != pin || report.RecordingSHA256 != doccorpus.Digest(recorded) || len(report.Questions) != 4 {
			t.Fatalf("report does not bind its inputs: %s", reportBytes)
		}
		if got := report.Tools["get_feature"]; got.Agreement != 1 || got.Different+got.Refused != 2 {
			t.Fatalf("per-tool agreement not reported: %+v", report.Tools)
		}
		if got := report.Tools["search_docs"]; got.Agreement+got.Different+got.Refused != 1 || got.Agreement != 0 {
			t.Fatalf("operation mismatch counted as agreement: %+v", report.Tools)
		}
		code, again, _ := invoke("--mode", "parity", "--index", index, "--sha256", pin, "--request", questions)
		if code != 0 || !bytes.Equal(again, reportBytes) {
			t.Fatal("parity report bytes differ for identical inputs")
		}
		if code, out, _ := invoke("--mode", "parity", "--index", index, "--sha256", strings.Repeat("0", 64), "--request", questions); code == 0 || len(out) != 0 {
			t.Fatal("wrong operator pin produced a report")
		}
	})
}

func TestCorpusParityCommandFailuresExitNonzero(t *testing.T) {
	t.Run("DCP-V1-043 producer failure never exits zero with empty or partial output", func(t *testing.T) {
		root := corpusFixture(t)
		// A readable file that is not a corpus reaches the producer and must be
		// refused there; before DCP-V1-043 this exited zero with empty stdout.
		missing := filepath.Join("absent", "corpus.json")
		for name, args := range map[string][]string{
			"missing-artifact": {"--mode", "build", "--root", root, "--artifact", missing},
			"producer-refusal": {"--mode", "build", "--root", root, "--artifact", "evidence/provider.json"},
			"unknown-mode":     {"--mode", "serve", "--index", "x", "--sha256", "y", "--request", "z"},
			"mixed-flags":      {"--mode", "build", "--root", root, "--artifact", "corpus.json", "--sha256", "y"},
		} {
			code, out, stderr := invoke(args...)
			if code == 0 || len(out) != 0 || stderr == "" {
				t.Fatalf("%s: exit %d stdout %d bytes stderr %q", name, code, len(out), stderr)
			}
		}
	})
}
