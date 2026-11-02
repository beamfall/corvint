package appflows

import (
	"os"
	"path/filepath"
	"testing"
)

// AFU-V1-904: CheckRunHeader passes a header whose records encode, and for each field set
// invalid it names exactly that field while the record validator refuses the same header.
func TestAFUV1904CheckRunHeaderAgreesWithValidator(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "playwright-attribution.json"))
	if err != nil {
		t.Fatal(err)
	}
	if problems := CheckRunHeader(runHeader()); len(problems) != 0 {
		t.Fatalf("valid header problems %+v", problems)
	}
	if _, err := IngestRunEvidence(FormatPlaywrightJSON, raw, runHeader()); err != nil {
		t.Fatalf("valid header refused: %v", err)
	}
	for field, spoil := range map[string]func(*RunHeader){
		"run_id":                func(h *RunHeader) { h.RunID = "-run" },
		"runner_version":        func(h *RunHeader) { h.RunnerVersion = "" },
		"source_commit":         func(h *RunHeader) { h.Source.Commit = "abc" },
		"source_tree":           func(h *RunHeader) { h.Source.Tree = "" },
		"build_artifact_digest": func(h *RunHeader) { h.BuildArtifactDigest = "sha256:" + h.BuildArtifactDigest },
		"environment_id":        func(h *RunHeader) { h.Environment.ID = "" },
		"environment_digest":    func(h *RunHeader) { h.Environment.Digest = "ABC" },
		"fixture_id":            func(h *RunHeader) { h.Fixture.ID = "a b" },
		"fixture_digest":        func(h *RunHeader) { h.Fixture.Digest = "" },
		"cleanup":               func(h *RunHeader) { h.Cleanup = "skipped" },
	} {
		h := runHeader()
		spoil(&h)
		problems := CheckRunHeader(h)
		if len(problems) != 1 || problems[0].Field != field || problems[0].Format == "" {
			t.Fatalf("%s: problems %+v", field, problems)
		}
		if _, err := IngestRunEvidence(FormatPlaywrightJSON, raw, h); err == nil {
			t.Fatalf("%s: the record validator accepted a header CheckRunHeader refuses", field)
		}
	}
}
