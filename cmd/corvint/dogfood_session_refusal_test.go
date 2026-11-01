package main

import (
	"strings"
	"testing"
)

// TestLCPV0810SessionRefusalsNameSessionKey (V1-3735): `dogfood status` with
// no host identity, or with two, names --session-key; a positional-BASE
// subverb without BASE names `dogfood ACTION BASE` (DCW-V0-808).
func TestLCPV0810SessionRefusalsNameSessionKey(t *testing.T) {
	for _, name := range []string{"CODEX_THREAD_ID", "CODEX_SESSION_ID", "CLAUDECODE", "CLAUDE_CODE_SESSION_ID"} {
		t.Setenv(name, "")
	}
	code, stdout, stderr := runCLI(t, "dogfood", "status")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code":"session-identity-required"`) || !strings.Contains(stderr, "pass --session-key KEY") ||
		!strings.Contains(stderr, "CLAUDECODE=1 with CLAUDE_CODE_SESSION_ID") {
		t.Fatalf("no identity: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude")
	t.Setenv("CODEX_THREAD_ID", "codex")
	code, stdout, stderr = runCLI(t, "dogfood", "status")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code":"session-identity-ambiguous"`) || !strings.Contains(stderr, "pass --session-key KEY") {
		t.Fatalf("two identities: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, action := range []string{"seal", "check", "change"} {
		code, stdout, stderr = runCLI(t, "dogfood", action)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code":"dogfood-base-required"`) ||
			!strings.Contains(stderr, "dogfood "+action+" takes the base as its one positional argument: corvint dogfood "+action+" BASE") {
			t.Errorf("%s without BASE: exit=%d stdout=%q stderr=%q", action, code, stdout, stderr)
		}
	}
}
