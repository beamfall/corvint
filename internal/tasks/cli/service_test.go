//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/service"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// SERVICE500-001/008: the public `service` route parses closed flags,
// refuses before any user-manager call, and `service status` plus
// `dispatch status` are pure reads that add nothing when no service is
// installed. HOME is a disposable temp directory; no manager runs.
func TestSERVICE500_CLIRoutesRefusalsAndPureReads(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	refused := func(code string, args ...string) run {
		t.Helper()
		r := atm(t, home, nil, args...)
		if r.res.Outcome != wire.OutcomeError || (code != "" && (len(r.res.Codes) != 1 || r.res.Codes[0] != code)) {
			t.Fatalf("%v: want ERROR %s, got %s", args, code, r.stdout)
		}
		return r
	}
	refused("", "service")
	refused("", "service", "enable", "--program", "site")
	refused("", "service", "install", "--program", "site", "--request-id", "i-1")
	refused("", "service", "status", "--program", "site", "--program", "site")
	refused("", "service", "status", "--program", "site", "--bogus", "x")
	refused("", "service", "status", "--program", "Bad Name")
	refused("", "service", "stop", "--program", "site", "--request-id")

	state := filepath.Join(home, ".local")
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("refused service commands created state")
	}
	status := atm(t, home, nil, "service", "status", "--program", "site")
	if status.res.Outcome != wire.OutcomeOK || field(status.res.Items[0], "installed").Str != "ABSENT" || field(status.res.Items[0], "desired").Str != "ABSENT" {
		t.Fatalf("absent status: %s", status.stdout)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("service status created state")
	}
	refused(wire.CodeUnsupported, "service", "stop", "--program", "site", "--request-id", "s-1", "--drain")
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("refused drain created state")
	}

	// A relative --config resolves against the working directory; a
	// temporary production path is refused before any manager call.
	cfgDir := filepath.Join(home, "config")
	profile := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(service.ProfileName)).Set("executable", wire.String(filepath.Join(home, "bin", "corvint-tasks"))).Set("dispatchConfig", wire.String(filepath.Join(cfgDir, "dispatch.json"))).Set("workRoot", wire.String(filepath.Join(home, "project"))).Set("legacyStopFile", wire.Null()).Set("helpers", wire.Array())))
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "service.json"), profile, 0o600); err != nil {
		t.Fatal(err)
	}
	install := refused(wire.CodeMalformed, "service", "install", "--program", "site", "--config", "config/service.json", "--request-id", "i-1")
	if !strings.Contains(string(install.stdout), "temporary production path") {
		t.Fatalf("relative config was not resolved and decoded: %s", install.stdout)
	}
	if _, err := os.Stat(filepath.Join(home, "Library")); !os.IsNotExist(err) {
		t.Fatal("refused install created a unit directory")
	}
	refused(wire.CodeUnsupported, "service", "run", "--program", "site", "--manifest", filepath.Join(home, "manifest.json"))

	// dispatch status keeps its exact shape when no service binds it.
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": filepath.Join(home, "dispatch-state"), "workRoot": filepath.Join(home, "project"),
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts":   map[string]any{"sh": map[string]any{"argv": []string{"/bin/sh", "{prompt}"}}},
		"roles":   []any{map[string]any{"name": "impl", "host": "sh", "cap": 1, "match": map[string]any{}, "prompt": "work on {ticketLocal}", "idleSeconds": 30, "wallSeconds": 60}},
		"backoff": map[string]any{"parkAfter": 2},
	}
	raw, _ := json.Marshal(config)
	configPath := filepath.Join(cfgDir, "dispatch.json")
	fixture.Write(t, configPath, raw)
	ds := atm(t, home, nil, "dispatch", "status", "--program", "site", "--config", configPath)
	if ds.res.Outcome != wire.OutcomeOK {
		t.Fatalf("dispatch status: %s", ds.stdout)
	}
	if _, ok := ds.res.Items[0].Obj.Get("service"); ok {
		t.Fatalf("dispatch status reported an absent service: %s", ds.stdout)
	}
}
