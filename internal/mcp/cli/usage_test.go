package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// MCPV0-113: --help prints usage and exits 0; a relative --root is refused by
// naming the absolute-path requirement, never resolved against the launch directory.
func TestMCPV0113HelpAndRelativeRoot(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		exit := run(context.Background(), []string{flag}, bytes.NewReader(nil), &stdout, &stderr)
		if exit != 0 || stderr.Len() != 0 || !strings.HasPrefix(stdout.String(), "usage: corvint-mcp --root ABSOLUTE_ROOT") {
			t.Fatalf("%s exit=%d stdout=%q stderr=%q", flag, exit, stdout.String(), stderr.String())
		}
	}
	// MCPV0-950: the plain relative root now starts unavailable, naming the
	// same requirement, instead of exiting before the handshake.
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{"--root", "."}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stdout.Len() != 0 || stderr.String() != "corvint-mcp: repository unavailable: --root must be an absolute path; serving tools/list, and every repository tool call answers invalid-root\n" {
		t.Fatalf("relative exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exit = run(context.Background(), []string{"--help", "--root", "/x"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 2 || stderr.String() != "corvint-mcp: invalid arguments\n" {
		t.Fatalf("help with extras exit=%d stderr=%q", exit, stderr.String())
	}
}
