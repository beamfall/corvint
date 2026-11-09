package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Handler-level flows tests live in internal/command/flows. These remain here because they exercise
// root dispatch, the `how` alias, root help or the compiled binary.

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
