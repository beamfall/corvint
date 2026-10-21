package doccorpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDCPV1080GuideWitnessRequestExample proves the DCP-V1-080 walkthrough: the guide's marked
// REQUEST.json sample closed-decodes, its receipt is the observed external-provider full-file
// input it describes, and a real request whose receipt input carries the sample's anchor
// metadata imports a witness through the documented --receipt-input.
func TestDCPV1080GuideWitnessRequestExample(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "DOCUMENTATION-CORPUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide := string(raw)
	_, rest, found := strings.Cut(guide, "<!-- DCP-V1-080 witness request example -->\n```json\n")
	example, _, closed := strings.Cut(rest, "```")
	if !found || !closed || strings.Count(example, `"PROVIDER_MEMBER"`) != 1 {
		t.Fatal("guide lacks the marked DCP-V1-080 witness request example")
	}
	var sample BehaviorAdapterRequest
	if err := decode([]byte(strings.Replace(example, `"PROVIDER_MEMBER"`, legacyV1Member(t), 1)), &sample); err != nil {
		t.Fatalf("guide witness request example does not decode: %v\n%s", err, example)
	}
	if sample.Schema != BehaviorAdapterRequestSchema || sample.Source != sample.Revisions.E2E || sample.SourceRevision != sample.Source.Revision || len(sample.Inputs) != 1 {
		t.Fatalf("guide witness request example is not a provider request with one receipt input: %+v", sample)
	}
	receipt := sample.Inputs[0]
	if a := receipt.Anchor; receipt.ID != "receipt" || a.Authority != "external-provider" || a.Kind != "observed" || a.Path != "evidence/receipt.json" || a.Start != 1 || a.End != 1 || a.Repository != sample.Source.ID || a.Revision == sample.SourceRevision || a.SHA256 != a.SpanSHA256 {
		t.Fatalf("guide receipt input is not the documented observed external-provider full-file anchor: %+v", receipt)
	}
	for _, phrase := range []string{
		"witness-playwright --input evidence/request.json --receipt-input receipt --report evidence/report.json",
		"discover-playwright --migration evidence/migration.json",
		"--reporter=json > evidence/report.json", "report-foreign-config",
		PlaywrightEventAnnotation, PlaywrightCleanupAnnotation, PlaywrightFixtureAnnotation, PlaywrightRoleAnnotation,
		"missing_annotations", "internal/doccorpus/behavior-events.cjs", "DCP-V1-081", "DCP-V1-082",
	} {
		if !strings.Contains(guide, phrase) {
			t.Fatalf("guide walkthrough lacks %q", phrase)
		}
	}

	repo := newPlaywrightBehaviorRepo(t)
	request := repo.request(t, repo.discover(t), nil, nil, nil)
	overlaid := false
	for i, input := range request.Inputs {
		if input.ID != receipt.ID {
			continue
		}
		a := &request.Inputs[i].Anchor
		if a.Path != receipt.Anchor.Path || a.Start != receipt.Anchor.Start || a.End != receipt.Anchor.End || a.Repository != request.Source.ID || a.Revision == request.SourceRevision {
			t.Fatalf("committed receipt anchor differs from the guide's placement: %+v", *a)
		}
		a.Authority, a.Kind, a.Reason = receipt.Anchor.Authority, receipt.Anchor.Kind, receipt.Anchor.Reason
		overlaid = true
	}
	if !overlaid {
		t.Fatal("request lacks the receipt input")
	}
	bundle := importPlaywright(t, request, receipt.ID, repo.report)
	if len(bundle.Witnesses) != 1 || bundle.Witnesses[0].TestID != repo.receiptIDs["lists items"] || bundle.Witnesses[0].Document == "" || bundle.Witnesses[0].Observation.ID == "" {
		t.Fatalf("guide-shaped receipt input did not import the witness: %+v", bundle)
	}
}
