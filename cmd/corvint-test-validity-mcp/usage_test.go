package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// MMR-V0-016: --help and -h alone print usage on stdout and exit 0; the
// usage names the unified corvint-mcp equivalent for the one-release alias.
func TestMMRV0016HelpPrintsUsage(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		exit := run(context.Background(), []string{flag}, strings.NewReader(""), &stdout, &stderr)
		if exit != 0 || stderr.Len() != 0 || !strings.HasPrefix(stdout.String(), "usage: corvint-test-validity-mcp ") ||
			!strings.Contains(stdout.String(), "corvint-mcp --root ROOT --tool-profile test-validity") {
			t.Fatalf("%s exit=%d stdout=%q stderr=%q", flag, exit, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := run(context.Background(), []string{"--help", "--version"}, strings.NewReader(""), &stdout, &stderr); exit != 2 || stdout.Len() != 0 {
		t.Fatalf("--help beside another argument exit=%d stdout=%q", exit, stdout.String())
	}
}
