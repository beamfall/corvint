package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// DCP-V1-268: `docs corpus audit sample|report` draw a reproducible sample and report it against
// the rederived corpus; with no verdicts the report abstains, and invalid options exit 2.
func TestDCPV1268CorpusAuditCLI(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	args := []string{"docs", "corpus", "audit", "sample", "--artifact", "corpus.json", "--seed", "9", "--size", "2", "--stratify", "kind"}
	code, out, stderr := corpusCLI(t, root, args...)
	if code != 0 {
		t.Fatalf("audit sample = %d %s", code, stderr)
	}
	if _, again, _ := corpusCLI(t, root, append([]string{"docs", "corpus", "audit-sample"}, args[4:]...)...); again != out {
		t.Fatal("the spelled and hyphenated forms, or two runs, gave different samples")
	}
	var sample doccorpus.AuditSample
	if err := json.Unmarshal([]byte(out), &sample); err != nil {
		t.Fatal(err)
	}
	if sample.Schema != doccorpus.AuditSampleSchema || sample.Design.Sampled != 2 || sample.Design.Stratify != "kind" || sample.Design.Population < 2 {
		t.Fatalf("sample = %s", out)
	}
	cemWrite(t, root, "sample.json", out)
	code, out, stderr = corpusCLI(t, root, "docs", "corpus", "audit", "report", "--artifact", "corpus.json", "--sample", "sample.json")
	if code != 0 {
		t.Fatalf("audit report = %d %s", code, stderr)
	}
	var report doccorpus.AuditReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != doccorpus.AuditReportSchema || report.Overall.Sampled != 2 || report.Overall.Unaudited != 2 || report.Overall.Accuracy.Defined || report.Overall.Accuracy.Value != nil {
		t.Fatalf("report without verdicts did not abstain: %s", out)
	}
	code, table, stderr := corpusCLI(t, root, "docs", "corpus", "audit-report", "--artifact", "corpus.json", "--sample", "sample.json", "--format", "table")
	if code != 0 || !strings.Contains(table, "abstain (0/0)") {
		t.Fatalf("table = %d %q %s", code, table, stderr)
	}
	for _, bad := range [][]string{
		{"audit-sample", "--artifact", "corpus.json", "--seed", "-1", "--size", "2"},
		{"audit-sample", "--artifact", "corpus.json", "--seed", "1", "--size", "0"},
		{"audit-sample", "--artifact", "corpus.json", "--seed", "1", "--size", "2", "--stratify", "author"},
		{"audit-report", "--artifact", "corpus.json", "--sample", "sample.json", "--format", "yaml"},
		{"audit-report", "--artifact", "corpus.json", "--sample", "corpus.json"},
	} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus"}, bad...)...); code != 2 {
			t.Fatalf("%v exit = %d, want 2", bad, code)
		}
	}
}
