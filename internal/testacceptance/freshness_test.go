package testacceptance

import (
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"strings"
	"testing"
)

// PTF-V0-004/006/008: an early actual serving mismatch has no native test row;
// missing later observations cannot erase it. Legacy requests stay separate.
func TestPTFV0EarlyStaleWithoutRow(t *testing.T) {
	source := []byte("current source")
	config := jstestprovider.FreshnessConfig{ArtifactPath: "app.html", SourceDigest: strings.TrimPrefix(Hash(source), "sha256:"), DocumentURL: "http://127.0.0.1:4394/"}
	native := jstestprovider.Receipt{Profile: jstestprovider.FreshnessProfile, Freshness: &jstestprovider.FreshnessBinding{ArtifactPath: config.ArtifactPath, Source: source, SourceDigest: config.SourceDigest, DocumentURL: config.DocumentURL, ArtifactAfter: strings.Repeat("b", 64)}}
	raw, e := jstestprovider.EncodeFreshness(native)
	if e != nil {
		t.Fatal(e)
	}
	request := Request{Schema: FreshRequestSchema, Repeat: 2, Tests: []Test{{ID: "counter"}}, Freshness: &FreshnessOptions{Provider: config}}
	report := Report{Runs: []Run{{Kind: "repeat", Ordinal: 1, NativeReceipt: raw, ReceiptSHA256: Hash(raw)}, {Kind: "repeat", Ordinal: 2}}}
	classify(&report, request)
	if report.Verdict != "rejected" || report.Assessments[0].Validity.Freshness.State != testvalidity.FreshnessStale {
		t.Fatalf("observed stale lost: %+v", report)
	}
	request.Schema = RequestSchema
	if Validate(request) == nil {
		t.Fatal("legacy request admitted freshness fields")
	}
}
