package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

const stabilityRegistryJSON = `{"schema":"flows-run-registry/0","policy":{"id":"e2e","thresholds":[
{"scope":"one-spec","required_repetitions":1,"minimum_passed":1,"maximum_failed":0,"maximum_timed_out":0,"maximum_interrupted":0,"maximum_infrastructure_failed":0,"maximum_skipped":0,"maximum_flaky":0,"maximum_retry_consumed":0},
{"scope":"feature-batch","required_repetitions":2,"minimum_passed":2,"maximum_failed":0,"maximum_timed_out":0,"maximum_interrupted":0,"maximum_infrastructure_failed":0,"maximum_skipped":0,"maximum_flaky":0,"maximum_retry_consumed":0},
{"scope":"suite","required_repetitions":3,"minimum_passed":3,"maximum_failed":0,"maximum_timed_out":0,"maximum_interrupted":0,"maximum_infrastructure_failed":0,"maximum_skipped":0,"maximum_flaky":0,"maximum_retry_consumed":0}]},
"aggregates":[{"id":"pays","scope":"one-spec","test_key":"shop > TestPay","planned":1,
"contributions":[{"run_id":"run-1","run_kind":"planned-repetition","repetition":1,"infrastructure_attempts":[]}]}]}`

// AFU-V1-042
func TestAFUV1FlowsCLIStabilityIsReadOnly(t *testing.T) {
	root := t.TempDir()
	shopGit(t, root, "init", "-q")
	shopWrite(t, root, map[string]string{"runs/registry.json": stabilityRegistryJSON})
	shopCommit(t, root, "registry")
	digest := strings.Repeat("a", 64)
	record := appflows.TestRunEvidence{Schema: appflows.RunEvidenceSchema, Authority: appflows.AuthorityIngested, RunID: "run-1",
		Runner: appflows.RunRunner{Name: "go-test", Version: "go1.27.1"}, Source: appflows.RunSource{Commit: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40), Clean: true},
		BuildArtifactDigest: digest, Environment: appflows.RunDigestRef{ID: "ci", Digest: digest}, Fixture: appflows.RunDigestRef{ID: "seed", Digest: digest}, TestKey: "shop > TestPay",
		Attempts: []appflows.RunAttempt{{Ordinal: 1, Outcome: "passed", AssertionAnchors: []appflows.RunAnchor{}, Attachments: []appflows.RunAttachment{}}},
		Cleanup:  "done", NegativeControls: []appflows.NegativeControl{}}
	line, err := appflows.EncodeRunEvidence(record)
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(t.TempDir(), "runs.jsonl")
	if err = os.WriteFile(evidence, line, 0600); err != nil {
		t.Fatal(err)
	}
	before := flowSnapshot(t, root)
	code, out, diagnostic := runFlowsCLI(root, "stability", "--registry", "runs/registry.json", "--evidence", evidence)
	if code != 0 || !strings.Contains(out, `"schema":"flows-run-stability/0"`) || !strings.Contains(out, `"verdict":"clean"`) {
		t.Fatalf("stability %d %s %s", code, out, diagnostic)
	}
	code, out, diagnostic = runFlowsCLI(root, "stability", "--registry", "runs/registry.json")
	if code != 2 || out != "" || !strings.Contains(diagnostic, "run-registry-missing-record") {
		t.Fatalf("a missing record must refuse with no report: %d %q %s", code, out, diagnostic)
	}
	after := flowSnapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d to %d", len(before), len(after))
	}
	for p, sum := range before {
		if after[p] != sum {
			t.Fatalf("%s changed", p)
		}
	}
}
