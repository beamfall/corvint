//go:build darwin || linux

package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestV10799_RefusedCodexStreamJournalsNoSession: when a later line of a
// Codex stream repeats or case-aliases a member, or its JSON handoff does,
// Run classes the turn INVALID_RESULT and journals no session, so a refused
// stream never offers a session for resumption (V1-0799).
func TestV10799_RefusedCodexStreamJournalsNoSession(t *testing.T) {
	exe, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	binary, e := ReadBounded(exe, 256<<20)
	if e != nil {
		t.Fatal(e)
	}
	for name, lines := range map[string][]string{
		"repeated type":    {codexStarted, `{"type":"turn.started","type":"turn.started"}`, codexMessage, codexDone},
		"repeated handoff": {codexStarted, `{"type":"item.completed","item":{"type":"agent_message","text":"{\"kind\":\"WAIT\",\"kind\":\"BUILT\",\"summary\":\"s\",\"nextAction\":\"n\"}"}}`, codexDone},
	} {
		c := Capsule{Profile: "taskman-codex-supervisor/0", Effect: Digest([]byte(name)), Executable: exe, ExecutableSHA256: Digest(binary),
			Directory: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"}, Argv: []string{"-c", "printf '%s\\n' '" + strings.Join(lines, "' '") + "'"}}
		var journaled []string
		out, _ := Run(context.Background(), os.Args[0], t.TempDir(), c, func(state string, _ Boot, o *Outcome) error {
			if state == "FINISHED" || state == "BLOCKED_RECOVERY" {
				journaled = append(journaled, o.SessionID)
			}
			return nil
		})
		if out.Class != "INVALID_RESULT" || out.SessionID != "" || out.Result.Kind != "" {
			t.Errorf("%s: outcome class %s session %q kind %q", name, out.Class, out.SessionID, out.Result.Kind)
		}
		for _, s := range journaled {
			if s != "" {
				t.Errorf("%s: journaled session %q", name, s)
			}
		}
	}
}
