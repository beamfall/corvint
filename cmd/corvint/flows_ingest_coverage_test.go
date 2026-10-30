package main

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// AFU-V1-112 AFU-V1-119 AFU-V1-120
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
	report, evidence, unbound, trace := filepath.Join(dir, "go-test.json"), filepath.Join(dir, "run.jsonl"), filepath.Join(dir, "unbound.jsonl"), filepath.Join(dir, "trace.lcov")
	if err := os.WriteFile(trace, []byte("SF:app.js\nDA:1,1\nend_of_record\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(`{"Action":"run","Package":"shop","Test":"TestPay"}`+"\n"+`{"Action":"pass","Package":"shop","Test":"TestPay","Elapsed":0.1}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	header := []string{"ingest", "--format", "go-test-json", "--from", report, "--run-id", "run-1", "--runner-version", "1.0.0", "--source-commit", commit, "--source-tree", tree,
		"--source-clean", "--build-artifact-digest", strings.Repeat("c", 64), "--environment-id", "env", "--environment-digest", strings.Repeat("e", 64),
		"--fixture-id", "fixture", "--fixture-digest", strings.Repeat("f", 64), "--cleanup", "done"}
	// AFU-V1-120: the producer pins the coverage file the run wrote; without the pin the run's
	// record does not bind that coverage.
	for file, extra := range map[string][]string{evidence: {"--coverage-artifact", "lcov=" + trace}, unbound: nil} {
		code, out, diagnostic := runFlowsCLI(root, append(slices.Clone(header), extra...)...)
		if code != 0 || out == "" {
			t.Fatalf("flows ingest %d %s %s", code, out, diagnostic)
		}
		if err := os.WriteFile(file, []byte(out), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, diagnostic := runFlowsCLI(root, append(slices.Clone(header), "--coverage-artifact", "no-file")...); code != 2 || out != "" || !strings.Contains(diagnostic, "flows ingest requires") {
		t.Fatalf("malformed --coverage-artifact exit %d %q %s", code, out, diagnostic)
	}
	if code, out, diagnostic := runFlowsCLI(root, "ingest-coverage", "--format", "lcov", "--from", trace, "--evidence", unbound, "--revision", commit); code != 0 ||
		strings.Contains(out, `"type":"tested_by"`) || !strings.Contains(out, `"reason":"coverage-not-bound-to-run"`) {
		t.Fatalf("unbound ingest-coverage %d %s %s", code, out, diagnostic)
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
