package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0074_RunHostFlag proves --host must name the config host (absent
// config host is Codex) and is refused UNSUPPORTED before any program record,
// and that help names both hosts.
func TestCALV0074_RunHostFlag(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, host string) string {
		t.Helper()
		body := `{"profile":"taskman-codex-supervisor/0","executable":"/absent","executableSha256":"","model":"m","effort":"low","prompt":"","workRoot":"` + dir + `","wallSeconds":60`
		if host != "" {
			body += `,"host":"` + host + `"`
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body+"}"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	codex, claude := write("codex.json", ""), write("claude.json", "claude-code")
	programs := filepath.Join(repo.StateDir, "programs.json")
	for _, tc := range []struct{ verb, config, host string }{
		{"run", codex, "claude-code"},
		{"admit", claude, "codex"},
		{"drain", claude, "opencode"},
		{"run", codex, "Codex"},
	} {
		r := atm(t, root, nil, tc.verb, "--program", "prog", "--config", tc.config, "--host", tc.host)
		if r.res.Outcome == wire.OutcomeOK || !hasCode(r.res, wire.CodeUnsupported) {
			t.Fatalf("%s --host %s with %s: %s", tc.verb, tc.host, filepath.Base(tc.config), r.stdout)
		}
		if _, err := os.Stat(programs); !os.IsNotExist(err) {
			t.Fatalf("%s --host %s wrote programs.json: %v", tc.verb, tc.host, err)
		}
	}
	// A matching --host passes the flag check and reaches the store, which
	// refuses this fixture for its absent supervision policy, not its host.
	for config, host := range map[string]string{codex: "codex", claude: "claude-code"} {
		r := atm(t, root, nil, "run", "--program", "prog", "--config", config, "--host", host)
		if strings.Contains(string(r.stdout), "--host") {
			t.Fatalf("matching --host %s refused: %s", host, r.stdout)
		}
	}
	help := atm(t, root, nil, "help")
	if !strings.Contains(string(help.stdout), "Codex, Claude Code or OpenCode") || !strings.Contains(string(help.stdout), "--host codex|claude-code|opencode") {
		t.Fatalf("help does not name the Claude Code host: %s", help.stdout)
	}
}
