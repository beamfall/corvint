package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0074_PolicyHost(t *testing.T) {
	p, e := supervisionPolicy(t, `"host":"claude-code",`)
	if e != nil {
		t.Fatal(e)
	}
	if p.Supervision.Host != intent.SupervisedHostClaudeCode || p.Supervision.SupervisedHost() != "claude-code" {
		t.Fatalf("host %q", p.Supervision.Host)
	}
	if p, e = supervisionPolicy(t, ""); e != nil || p.Supervision.SupervisedHost() != "" {
		t.Fatalf("absent host is not Codex: %+v %v", p, e)
	}
	if (*intent.SupervisionPolicy)(nil).SupervisedHost() != "" {
		t.Fatal("nil supervision host is not Codex")
	}
	// Codex is only ever the absent host, so a Codex policy has one encoding.
	for _, host := range []string{`"codex"`, `"gemini-cli"`, `"Claude-Code"`, `""`, `null`, `{}`} {
		_, e := supervisionPolicy(t, `"host":`+host+`,`)
		if e == nil {
			t.Fatalf("host %s admitted", host)
		}
		if host[0] == '"' && wire.CodeOf(e) != wire.CodeUnsupported {
			t.Fatalf("host %s refused %s, want %s: %v", host, wire.CodeOf(e), wire.CodeUnsupported, e)
		}
	}
}
