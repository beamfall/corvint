package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCX-V1-343, DCX-V1-346, DCX-V1-349, DCX-V1-350, DCX-V1-352: the claim mention check is served
// by CLI and MCP, `docs corpus patterns` reports the patterns, `--stratify patterns` is admitted,
// and the opt-in build thresholds refuse a corpus above them without writing it.
func TestDCXV1349CorpusPatternsCLI(t *testing.T) {
	root, m := typedCorpusCLIFixtureWith(t, func(p *doccorpus.ProviderRecord, _ doccorpus.Evidence) {
		p.Claims[0].Text = "`Checkout()` is declared beside `missingThing`"
	})
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "get", "--artifact", "corpus.json", "--id", "proof:claim")
	if code != 0 {
		t.Fatalf("get = %d %s", code, stderr)
	}
	var receipt doccorpus.Receipt
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	check, ok := receipt.ClaimMentions["proof:claim"]
	if !ok || check.State != doccorpus.MentionsPartiallyUnresolved || check.InSpan != 1 || check.Unresolved != 1 {
		t.Fatalf("CLI claim_mentions = %+v", receipt.ClaimMentions)
	}
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"id": "proof:claim"})
	_, text, failure, protocol := registry.Call(context.Background(), "corvint.docs_get", args)
	if failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	var served doccorpus.Receipt
	if err := json.Unmarshal([]byte(text), &served); err != nil || served.ClaimMentions["proof:claim"].State != check.State {
		t.Fatalf("MCP claim_mentions = %+v %v", served.ClaimMentions, err)
	}

	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "patterns", "--artifact", "corpus.json")
	if code != 0 {
		t.Fatalf("patterns = %d %s", code, stderr)
	}
	var report doccorpus.CorpusPatternReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != doccorpus.CorpusPatternsSchema || report.Population != 1 || len(report.Patterns) != len(doccorpus.CorpusPatternNames) || report.Patterns[0].Count != 0 {
		t.Fatalf("report = %s", out)
	}
	if _, again, _ := corpusCLI(t, root, "docs", "corpus", "patterns", "--artifact", "corpus.json"); again != out {
		t.Fatal("patterns report is nondeterministic")
	}
	code, table, stderr := corpusCLI(t, root, "docs", "corpus", "patterns", "--artifact", "corpus.json", "--format", "table")
	if code != 0 || !strings.Contains(table, "examined 1  flagged 0") || !strings.Contains(table, "uniform-shape") {
		t.Fatalf("table = %d %q %s", code, table, stderr)
	}

	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "audit-sample", "--artifact", "corpus.json", "--seed", "3", "--size", "1", "--stratify", "patterns")
	var sample doccorpus.AuditSample
	if code != 0 || json.Unmarshal([]byte(out), &sample) != nil || sample.Design.Stratify != "patterns" || len(sample.Strata) == 0 {
		t.Fatalf("patterns sample = %d %s %s", code, out, stderr)
	}

	raw, encodeErr := doccorpus.Encode(m)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	cemWrite(t, root, "corpus-input.json", string(raw))
	code, plain, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json")
	if code != 0 {
		t.Fatalf("build = %d %s", code, stderr)
	}
	code, met, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json", "--max-unresolved-mention-rate", "1", "--max-pattern-rate=0")
	if code != 0 || met != plain {
		t.Fatalf("met thresholds changed the build: %d %s", code, stderr)
	}
	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json", "--max-unresolved-mention-rate", "0.5")
	if code == 0 || out != "" || !strings.Contains(stderr, "unresolved-mention rate 1/1 = 1 exceeds 0.5") {
		t.Fatalf("threshold refusal = %d %q %s", code, out, stderr)
	}
	for _, bad := range [][]string{
		{"build", "--manifest", "corpus-input.json", "--max-unresolved-mention-rate", "2"},
		{"build", "--manifest", "corpus-input.json", "--max-unresolved-mention-rate="},
		{"build", "--manifest", "corpus-input.json", "--max-unresolved-mention-rate", ""},
		{"build", "--manifest", "corpus-input.json", "--max-pattern-rate="},
		{"build", "--manifest", "corpus-input.json", "--max-pattern-rate", " "},
		{"build", "--manifest", "corpus-input.json", "--max-pattern-rate", "nope=0.1"},
		{"build", "--manifest", "corpus-input.json", "--max-pattern-rate", "anchor-cluster=0.1,anchor-cluster=0.2"},
		{"patterns", "--artifact", "corpus.json", "--format", "yaml"},
		{"patterns"},
	} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, bad...)...); code != 2 {
			t.Fatalf("%v exit = %d, want 2", bad, code)
		}
	}
}
