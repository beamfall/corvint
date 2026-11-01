package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLMGV0014ReportSession reports one Claude Code session, says NOT_OBSERVED
// for a session with no lines, and refuses a malformed key.
func TestLMGV0014ReportSession(t *testing.T) {
	key, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	state := t.TempDir()
	ledger := `{"profile":"corvint-gateway-usage/0","status":200,"claudeSessionKey":"` + key + `","inputTokens":10,"cacheReadInputTokens":30,"outputTokens":5}
{"profile":"corvint-gateway-usage/0","status":200,"claudeSessionKey":"` + other + `","inputTokens":999}
`
	if err := os.WriteFile(filepath.Join(state, "usage.jsonl"), []byte(ledger), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"report", "--state", state, "--session", key}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, "session "+key) || !strings.Contains(out, "requests 1 ") ||
		!strings.Contains(out, "input tokens 10 uncached, 30 cache-read") || strings.Contains(out, "999") {
		t.Fatalf("report:\n%s", out)
	}
	stdout.Reset()
	if code := run([]string{"report", "--state", state, "--session", strings.Repeat("c", 64)}, &stdout, &stderr); code != 0 ||
		!strings.Contains(stdout.String(), "NOT_OBSERVED") {
		t.Fatalf("exit %d, report:\n%s", code, stdout.String())
	}
	for _, bad := range [][]string{{"--session", "ABC"}, {"--session="}, {"--session", ""}, {"--session", strings.ToUpper(key)}} {
		if code := run(append([]string{"report", "--state", state}, bad...), &stdout, &stderr); code != 2 {
			t.Fatalf("%q exit %d", bad, code)
		}
	}
}
