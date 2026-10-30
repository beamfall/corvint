package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// CCE-V0-151: `e2e obligations --remedy-summary` writes e2e-remedy-summary/0 instead of the
// obligations document, and refuses --coverage.
func TestCCEV0151RemedySummaryCLI(t *testing.T) {
	root, base, head := fixture(t)
	var out bytes.Buffer
	args := []string{"e2e", "obligations", "--root", root, "--base", base, "--head", head, "--remedy-summary", "--remedy-commits", "5"}
	if err := run(context.Background(), args, &out); err != nil || !strings.Contains(out.String(), `"schema": "e2e-remedy-summary/0"`) {
		t.Fatalf("%v %s", err, out.String())
	}
	if err := run(context.Background(), append(args, "--coverage", "c.json"), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "does not combine") {
		t.Fatalf("--coverage: %v", err)
	}
}
