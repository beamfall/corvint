package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// AFU-V1-131 (V1-2874) and the flows diagnostic writer (V1-2873): a Playwright JSON run of the
// tests a playwright-list import linked is keyed as the import keys them, so each ingested record
// joins its imported link; a report whose files cannot be placed is still ingested, and each of its
// tests is named on stderr with the reason, exit 0.
func TestAFUV1131IngestJoinsPlaywrightListImport(t *testing.T) {
	root := flowIntentRepo(t)
	tests := filepath.Join(root, "tests")
	if err := os.Mkdir(tests, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tests, "cart.spec.ts"), []byte("test.describe('guest', () => { test('pays', () => {}) })\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir, err := json.Marshal(tests)
	if err != nil {
		t.Fatal(err)
	}
	suites := `"suites":[{"title":"cart.spec.ts","file":"cart.spec.ts","specs":[],"suites":[
  {"title":"guest","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium","expectedStatus":"passed","status":"expected","results":[{"status":"passed","duration":5}]}]}]},
  {"title":"member","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium","expectedStatus":"passed","status":"expected","results":[{"status":"passed","duration":7}]}]}]}]}]}`
	config := `{"config":{"rootDir":` + string(dir) + `,"projects":[{"name":"chromium","testDir":` + string(dir) + `}]},`
	scratch := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(scratch, name)
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	list, report, unplaced := write("list.json", config+suites), write("report.json", config+suites), write("bare.json", "{"+strings.Replace(suites, `"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium","expectedStatus":"passed","status":"expected","results":[{"status":"passed","duration":7`, `"pays again","file":"cart.spec.ts","tests":[{"projectName":"chromium","expectedStatus":"passed","status":"expected","results":[{"status":"passed","duration":7`, 1))
	if code, _, diagnostic := runFlowsCLI(root, "import", "--flows", "flows", "--from", list, "--format", "playwright-list"); code != 0 {
		t.Fatalf("import %d %s", code, diagnostic)
	}
	commitFlows(t, root)
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	header := []string{"--run-id", "run-1", "--runner-version", "1.50.0", "--source-commit", strings.TrimSpace(string(head)),
		"--source-tree", strings.TrimSpace(string(tree)), "--source-clean", "--build-artifact-digest", digest,
		"--environment-id", "ci", "--environment-digest", digest, "--fixture-id", "seed", "--fixture-digest", digest, "--cleanup", "done"}
	code, out, diagnostic := runFlowsCLI(root, append([]string{"ingest", "--format", "playwright-json", "--from", report}, header...)...)
	if code != 0 || diagnostic != "" {
		t.Fatalf("ingest %d %s", code, diagnostic)
	}
	stored := write("runs.jsonl", out)
	records, err := appflows.ReadRunEvidence([]string{stored})
	if err != nil || len(records) != 2 {
		t.Fatalf("records %v %s", err, out)
	}
	set, err := appflows.LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	linked := map[string]bool{}
	for _, f := range set.Flows {
		for _, l := range f.Links {
			linked[l.Target.TestKey] = true
		}
	}
	for _, r := range records {
		if !linked[r.TestKey] {
			t.Fatalf("ingested key %q joins no imported link %v", r.TestKey, linked)
		}
	}
	code, out, diagnostic = runFlowsCLI(root, append([]string{"ingest", "--format", "playwright-json", "--from", unplaced}, header...)...)
	if code != 0 || strings.Count(out, "\n") != 2 || strings.Count(diagnostic, "does not join playwright-list import keys") != 2 ||
		!strings.Contains(diagnostic, `test "cart.spec.ts > pays" does not join`) || !strings.Contains(diagnostic, "declares neither config.rootDir") {
		t.Fatalf("unplaced ingest %d %q %s", code, out, diagnostic)
	}
}
