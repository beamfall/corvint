package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-176: `dispatch status` decodes a fragment configuration through the
// same DecodeConfig path, and reports a broken reference as an INVALID file
// whose reason names the role and the fragment.
func TestCALV0176_DispatchStatusReadsFragmentConfig(t *testing.T) {
	stateDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "dispatch.json")
	config := func(ref string) string {
		return `{"profile":"taskman-dispatch/0","stateDir":` + jsonQuote(stateDir) + `,"workRoot":` + jsonQuote(stateDir) +
			`,"tickSeconds":1,"globalCap":1,"killGraceSeconds":1,"hosts":{"h":{"argv":["/bin/echo","{prompt}"]}}` +
			`,"prompts":{"rules":"Shared rules.\n"}` +
			`,"roles":[{"name":"impl","host":"h","cap":1,"match":{},"prompt":[{"fragment":"` + ref + `"},"work {ticketLocal}"],"idleSeconds":30,"wallSeconds":60}]` +
			`,"backoff":{"cooldownSeconds":0,"parkAfter":1},"heal":{"handoff":true,"reap":true}}`
	}
	status := func() *wire.Result {
		t.Helper()
		var out, errb bytes.Buffer
		Run(Env{Cwd: stateDir, Args: []string{"dispatch", "status", "--program", "prog", "--config", path}, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeOK {
			t.Fatalf("status: %v %s %s", err, out.Bytes(), errb.Bytes())
		}
		return res
	}
	if err := os.WriteFile(path, []byte(config("rules")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := statusField(t, status().Items[0], "configFile"); ok {
		t.Fatal("valid fragment config reported as a configFile problem")
	}
	if err := os.WriteFile(path, []byte(config("missing")), 0o600); err != nil {
		t.Fatal(err)
	}
	file, ok := statusField(t, status().Items[0], "configFile")
	if !ok || pressureField(t, file, "state").Str != "INVALID" || !strings.Contains(pressureField(t, file, "reason").Str, `role impl prompt references unknown fragment "missing"`) {
		t.Fatalf("configFile %+v", file)
	}
}
