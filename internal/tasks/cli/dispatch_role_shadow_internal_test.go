package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-290: `dispatch status` reports each shadowed role of the
// configuration file in roleShadows and as a result warning, and omits both
// when no role is shadowed.
func TestCALV0290_DispatchStatusReportsShadowedRoles(t *testing.T) {
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "dispatch.json")
	write := func(special int) {
		t.Helper()
		role := func(name string, priority int, match map[string]any) map[string]any {
			return map[string]any{"name": name, "host": "sh", "cap": 1, "priority": priority, "match": match, "prompt": "p", "idleSeconds": 30, "wallSeconds": 60}
		}
		raw, _ := json.Marshal(map[string]any{
			"profile": dispatch.ConfigProfile, "stateDir": stateDir, "workRoot": stateDir,
			"tickSeconds": 1, "globalCap": 2, "killGraceSeconds": 1,
			"hosts":     map[string]any{"sh": map[string]any{"argv": []string{"/bin/sh", "{prompt}"}}},
			"workState": map[string]any{"kind": "status-line", "path": "/work/{ticketLocal}", "key": "state"},
			"roles": []any{
				role("worker", 50, map[string]any{"states": []string{"work"}}),
				role("special", special, map[string]any{"idGlob": "T-001[0-4]", "states": []string{"work"}}),
			},
			"backoff": map[string]any{"cooldownSeconds": 0, "parkAfter": 1},
			"heal":    map[string]any{"handoff": true, "reap": true},
		})
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	status := func() *wire.Result {
		t.Helper()
		var out, errb bytes.Buffer
		// Off darwin and linux Tasks refuses the command up front (CTS-V0-025).
		fixture.SkipUnsupportedPlatform(t)
		Run(Env{Cwd: stateDir, Args: []string{"dispatch", "status", "--program", "prog", "--config", path}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeOK {
			t.Fatalf("status: %v %+v\n%s%s", err, res, out.Bytes(), errb.Bytes())
		}
		return res
	}

	write(92)
	res := status()
	rows, ok := statusField(t, res.Items[0], "roleShadows")
	if !ok || len(rows.Arr) != 1 {
		t.Fatalf("roleShadows = %+v", rows)
	}
	statusStrings(t, rows.Arr[0], map[string]string{"role": "special", "priority": "92", "shadowedBy": "worker", "shadowedByPriority": "50"})
	want := dispatch.RoleShadow{Role: "special", ShadowedBy: "worker", Priority: 92, ShadowedByPriority: 50}.Message()
	if len(res.Warnings) != 1 || res.Warnings[0] != want || !strings.Contains(want, "lower priority values win") {
		t.Fatalf("warnings = %q", res.Warnings)
	}

	write(10)
	res = status()
	if _, ok := statusField(t, res.Items[0], "roleShadows"); ok || len(res.Warnings) != 0 {
		t.Fatalf("correctly ordered roles reported a shadow: %+v", res)
	}
}
