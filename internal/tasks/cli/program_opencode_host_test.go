package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

func TestCALV0076_ConfigHostFlag(t *testing.T) {
	for _, tc := range []struct {
		flag, host string
		want       bool
	}{
		{"codex", "", true},
		{"opencode", supervisor.HostOpenCode, true},
		{"claude-code", supervisor.HostClaudeCode, true},
		{"opencode", "", false},
		{"opencode", supervisor.HostClaudeCode, false},
		{"codex", supervisor.HostOpenCode, false},
		{"gemini-cli", "gemini-cli", false},
	} {
		if got := configHost(tc.flag, tc.host); got != tc.want {
			t.Errorf("--host %s with config host %q: %v", tc.flag, tc.host, got)
		}
	}
}
