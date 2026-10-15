//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-225 (#900): a relative --config resolves against the command's
// working directory and `..` is resolved lexically, in all five spellings.
func TestCALV0225_DispatchConfigPathForms(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(base, "d")
	for _, dir := range []string{filepath.Join(cwd, "cfg"), filepath.Join(cwd, "x")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": t.TempDir(), "workRoot": cwd,
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts":   map[string]any{"sh": map[string]any{"argv": []string{"/bin/sh", "-c", "exit 0"}}},
		"roles":   []any{map[string]any{"name": "impl", "host": "sh", "cap": 1, "match": map[string]any{"labels": []string{"never-launch"}}, "prompt": "work", "idleSeconds": 30, "wallSeconds": 60}},
		"backoff": map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":    map[string]any{"handoff": true, "reap": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(cwd, "cfg", "x.json"), raw)
	forms := []string{
		"cfg/x.json",
		"./cfg/x.json",
		"../d/cfg/x.json",
		filepath.Join(cwd, "x") + "/../cfg/x.json",
		filepath.Join(cwd, "cfg", "x.json"),
	}
	for _, form := range forms {
		r := atm(t, cwd, nil, "dispatch", "status", "--program", "prog", "--config", form)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s: %s", form, r.stdout)
		}
	}
	// The same rule serves the program verbs' --config.
	supervised := []byte(`{"profile":"taskman-codex-supervisor/99"}`)
	fixture.Write(t, filepath.Join(cwd, "cfg", "p.json"), supervised)
	for _, form := range []string{"cfg/p.json", "../d/cfg/p.json"} {
		r := atm(t, cwd, nil, "admit", "--program", "prog", "--config", form)
		if !hasCode(r.res, wire.CodeUnsupportedVersion) {
			t.Fatalf("%s: program config was not read: %s", form, r.stdout)
		}
	}
	// A missing relative file is reported as missing, never as a
	// filesystem-support refusal.
	r := atm(t, cwd, nil, "dispatch", "status", "--program", "prog", "--config", "cfg/absent.json")
	if r.res.Outcome == wire.OutcomeOK || hasCode(r.res, wire.CodeUnsupportedFilesystem) {
		t.Fatalf("absent relative config: %s", r.stdout)
	}
}
