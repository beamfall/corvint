//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCALV0074_CapsuleHost proves the capsule names its host canonically and
// that the lane leader decodes a zero exit in that host's vocabulary.
func TestCALV0074_CapsuleHost(t *testing.T) {
	exe, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	binary, e := ReadBounded(exe, 256<<20)
	if e != nil {
		t.Fatal(e)
	}
	base := Capsule{Profile: "taskman-codex-supervisor/0", Effect: strings.Repeat("b", 64), Executable: exe, ExecutableSHA256: Digest(binary), Directory: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"}}
	for _, host := range []string{HostCodex, "opencode", "CLAUDE-CODE"} {
		c := base
		c.Host = host
		if e := ValidateCapsule(c); e == nil {
			t.Fatalf("capsule host %q admitted", host)
		}
	}
	codex, _ := json.Marshal(base)
	if strings.Contains(string(codex), `"host"`) {
		t.Fatalf("codex capsule bytes carry host: %s", codex)
	}
	result := `{"type":"result","subtype":"success","is_error":false,"session_id":"claude-session","result":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":{"input_tokens":4,"output_tokens":2}}`
	for _, host := range []string{"", HostClaudeCode} {
		c := base
		c.Host = host
		c.Effect = Digest([]byte(host))
		c.Argv = []string{"-c", "printf '%s\\n' '" + result + "'"}
		if e := ValidateCapsule(c); e != nil {
			t.Fatalf("capsule host %q refused: %v", host, e)
		}
		dir := t.TempDir()
		out, e := Run(context.Background(), os.Args[0], dir, c, func(string, Boot, *Outcome) error { return nil })
		if e != nil && host == HostClaudeCode {
			t.Fatalf("claude-code run: %v", e)
		}
		if host == HostClaudeCode {
			if out.Class != "EXIT_ZERO" || out.SessionID != "claude-session" || out.Result.Kind != "BUILT" || !out.Clean {
				t.Fatalf("claude-code outcome %+v", out)
			}
		} else if out.Class != "INVALID_RESULT" {
			t.Fatalf("codex vocabulary admitted a Claude Code result: %+v", out)
		}
	}
	// A duplicate is_error (true then false) is not a successful result.
	duplicate := `{"type":"result","subtype":"success","is_error":true,"is_error":false,"session_id":"claude-session","result":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":{"input_tokens":4,"output_tokens":2}}`
	c := base
	c.Host = HostClaudeCode
	c.Effect = Digest([]byte("duplicate"))
	c.Argv = []string{"-c", "printf '%s\\n' '" + duplicate + "'"}
	out, _ := Run(context.Background(), os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	if out.Class != "INVALID_RESULT" || out.Result.Kind != "" {
		t.Fatalf("duplicate is_error outcome %+v", out)
	}
}
