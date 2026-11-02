package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// plainPlaywrightReport is a real Playwright JSON report with no Corvint annotations or header.
var plainPlaywrightReport = filepath.Join("..", "..", "internal", "appflows", "testdata", "playwright-attribution.json")

// AFU-V1-904 (V1-3937): a plain Playwright JSON report ingested with no header flags is refused
// once, naming every required header flag as missing with its format; a sha256:-prefixed digest
// and a bad cleanup are named as invalid; the full header ingests.
func TestAFUV1904PlainPlaywrightReportNamesEveryHeaderFlag(t *testing.T) {
	root := flowIntentRepo(t)
	code, out, diagnostic := runFlowsCLI(root, "ingest", "--format", "playwright-json", "--from", plainPlaywrightReport)
	if code != 2 || out != "" || !strings.Contains(diagnostic, `"code": "invalid-arguments"`) {
		t.Fatalf("bare ingest %d %q %s", code, out, diagnostic)
	}
	required := []string{"--run-id", "--runner-version", "--source-commit", "--source-tree", "--build-artifact-digest",
		"--environment-id", "--environment-digest", "--fixture-id", "--fixture-digest"}
	last := -1
	for _, flag := range required {
		at := strings.Index(diagnostic, flag+" missing (")
		if at < 0 || at < last {
			t.Fatalf("refusal does not name %s missing in header order: %s", flag, diagnostic)
		}
		last = at
	}
	if strings.Contains(diagnostic, "--cleanup") || !strings.Contains(diagnostic, "without a sha256: prefix") {
		t.Fatalf("bare ingest names the defaulted --cleanup or omits the digest format: %s", diagnostic)
	}

	digest := strings.Repeat("a", 64)
	header := []string{"--run-id", "run-1", "--runner-version", "1.62.1", "--source-commit", strings.Repeat("1", 40),
		"--source-tree", strings.Repeat("2", 40), "--environment-id", "ci", "--environment-digest", digest,
		"--fixture-id", "seed", "--fixture-digest", digest}
	bad := append([]string{"ingest", "--format", "playwright-json", "--from", plainPlaywrightReport,
		"--build-artifact-digest", "sha256:" + digest, "--cleanup", "skipped"}, header...)
	code, out, diagnostic = runFlowsCLI(root, bad...)
	want := "flows ingest run header: --build-artifact-digest invalid (a bare 64-character lowercase hex sha256 digest, without a sha256: prefix); --cleanup invalid (done, failed or not-declared)"
	if code != 2 || out != "" || !strings.Contains(diagnostic, want) || strings.Contains(diagnostic, "sha256:"+digest) {
		t.Fatalf("invalid ingest %d %q %s", code, out, diagnostic)
	}

	good := append([]string{"ingest", "--format", "playwright-json", "--from", plainPlaywrightReport,
		"--build-artifact-digest", digest, "--cleanup", "done"}, header...)
	if code, out, diagnostic = runFlowsCLI(root, good...); code != 0 || strings.Count(out, "\n") != 4 {
		t.Fatalf("full-header ingest %d %q %s", code, out, diagnostic)
	}
}

// AFU-V1-904: help shows the run header unbracketed and states each value format.
func TestAFUV1904Help(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if code := runContext(context.Background(), []string{"help", "flows"}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	help := strings.Join(strings.Fields(out.String()), " ")
	for _, want := range []string{
		"--from FILE --run-id ID --runner-version VERSION --source-commit SHA --source-tree SHA [--source-clean] --build-artifact-digest DIGEST --environment-id ID --environment-digest DIGEST --fixture-id ID --fixture-digest DIGEST [--cleanup STATE]",
		"Every --format, playwright-json included, requires the run header",
		"bare 64-hex sha256 digests (no sha256: prefix)", "--cleanup is done, failed or not-declared",
		"One refusal (exit 2, invalid-arguments) names every missing or invalid header flag with its format",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("flows help lacks %q", want)
		}
	}
	if strings.Contains(help, "[--run-id ID]") {
		t.Fatal("flows help still brackets --run-id as optional")
	}
}
