package main

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// AFU-V1-112 AFU-V1-119
func TestAFUV1112FlowsIngestCoverageCLI(t *testing.T) {
	root := flowIntentRepo(t)
	rev := func(arg string) string {
		c := exec.Command("git", "rev-parse", arg)
		c.Dir = root
		out, err := c.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	commit, tree, dir := rev("HEAD"), rev("HEAD^{tree}"), t.TempDir()
	record := appflows.TestRunEvidence{Schema: appflows.RunEvidenceSchema, Authority: appflows.AuthorityIngested, RunID: "run-1", Runner: appflows.RunRunner{Name: "playwright", Version: "1.0.0"},
		Source: appflows.RunSource{Commit: commit, Tree: tree, Clean: true}, BuildArtifactDigest: strings.Repeat("c", 64),
		Environment: appflows.RunDigestRef{ID: "env", Digest: strings.Repeat("e", 64)}, Fixture: appflows.RunDigestRef{ID: "fixture", Digest: strings.Repeat("f", 64)},
		TestKey: "checkout.spec.ts > chromium > pays", Project: "chromium", Cleanup: "done", NegativeControls: []appflows.NegativeControl{},
		Attempts: []appflows.RunAttempt{{Ordinal: 1, Outcome: "passed", DurationMS: 5, AssertionAnchors: []appflows.RunAnchor{}, Attachments: []appflows.RunAttachment{}}}}
	line, err := appflows.EncodeRunEvidence(record)
	if err != nil {
		t.Fatal(err)
	}
	evidence, trace := filepath.Join(dir, "run.jsonl"), filepath.Join(dir, "trace.lcov")
	if err = os.WriteFile(evidence, line, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(trace, []byte("SF:app.js\nDA:1,1\nend_of_record\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := flowSnapshot(t, root)
	code, out, diagnostic := runFlowsCLI(root, "ingest-coverage", "--format", "lcov", "--from", trace, "--evidence", evidence, "--revision", commit)
	if code != 0 || !strings.Contains(out, `"schema":"`+appflows.CoverageIngestSchema+`"`) || !strings.Contains(out, `"type":"tested_by"`) || !strings.Contains(out, `"from":"source:app.js"`) {
		t.Fatalf("ingest-coverage %d %s %s", code, out, diagnostic)
	}
	if !maps.Equal(before, flowSnapshot(t, root)) {
		t.Fatal("ingest-coverage wrote to the repository")
	}
	if code, out, diagnostic = runFlowsCLI(root, "ingest-coverage", "--format", "lcov", "--from", trace); code != 2 || out != "" || !strings.Contains(diagnostic, "flows ingest-coverage requires --format v8|istanbul|rspec|lcov") {
		t.Fatalf("missing flags exit %d %q %s", code, out, diagnostic)
	}
	if !strings.Contains(flowsIntentHelp, "flows ingest-coverage --format v8|istanbul|rspec|lcov --from FILE") {
		t.Fatal("flows help does not list ingest-coverage")
	}
	if code, out, diagnostic = runFlowsCLI(root, "ingest-coverage", "--format", "lcov", "--from", trace, "--evidence", evidence, "--revision", strings.Repeat("b", 40)); code != 2 || out != "" || !strings.Contains(diagnostic, "coverage-invalid-revision") {
		t.Fatalf("unknown revision exit %d %q %s", code, out, diagnostic)
	}
}
