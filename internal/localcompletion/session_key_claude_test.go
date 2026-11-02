package localcompletion

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// TestLCPV0810SessionKeyAdmitsClaudeCode (V1-3735): inside Claude Code the
// default key is the native hook's Claude-domain hash; a Codex identity also
// set is ambiguous; CLAUDE_CODE_SESSION_ID without CLAUDECODE=1 is no
// identity; Codex alone keeps its domain; an explicit key always wins.
func TestLCPV0810SessionKeyAdmitsClaudeCode(t *testing.T) {
	for _, name := range []string{"CODEX_THREAD_ID", "CODEX_SESSION_ID", "CLAUDECODE", "CLAUDE_CODE_SESSION_ID"} {
		t.Setenv(name, "")
	}
	hook := sha256.Sum256([]byte("corvint-local-completion-session/claude-code/0\x00claude-session-é"))
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-session-é")
	if _, err := SessionKey(""); err == nil || err.Error() != "session-identity-required" {
		t.Fatalf("without CLAUDECODE=1: %v; want session-identity-required", err)
	}
	t.Setenv("CLAUDECODE", "1")
	if key, err := SessionKey(""); err != nil || key != hex.EncodeToString(hook[:]) {
		t.Fatalf("Claude Code key = %q, %v; want the hook's %x", key, err, hook)
	}
	explicit := strings.Repeat("a", 64)
	if key, err := SessionKey(explicit); err != nil || key != explicit {
		t.Fatalf("explicit key = %q, %v", key, err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", strings.Repeat("x", 4097))
	if _, err := SessionKey(""); err == nil || err.Error() != "invalid-session-identity" {
		t.Fatalf("oversized Claude identity: %v", err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-session-é")
	t.Setenv("CODEX_SESSION_ID", "codex-session")
	if _, err := SessionKey(""); err == nil || err.Error() != "session-identity-ambiguous" {
		t.Fatalf("both hosts: %v; want session-identity-ambiguous", err)
	}
	t.Setenv("CLAUDECODE", "")
	if key, err := SessionKey(""); err != nil || key != HashSession("codex-session") {
		t.Fatalf("Codex alone = %q, %v", key, err)
	}
}
