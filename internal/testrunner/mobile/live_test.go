//go:build darwin || linux

package mobile

import (
	"context"
	"encoding/json"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"os"
	"path/filepath"
	"testing"
)

// Opt-in qualification uses an already running operator-owned isolated server
// and emulator. This test never starts a server, installs an app or downloads.
func TestLiveAppium(t *testing.T) {
	root, endpoint, device := os.Getenv("CORVINT_MOBILE_FIXTURE"), os.Getenv("CORVINT_APPIUM_ENDPOINT"), os.Getenv("CORVINT_APPIUM_DEVICE")
	if root == "" || endpoint == "" || device == "" {
		t.Skip("explicit owned Appium fixture/server/emulator not supplied")
	}
	out, e := os.MkdirTemp("", "appium-evidence-")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("live evidence: %s", out)
	hash := func(p string) string {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return tr.Digest(b)
	}
	exe := filepath.Join(root, "launcher.cjs")
	config := filepath.Join(root, "wdio.config.mjs")
	reporter := os.Getenv("CORVINT_WDIO_JSON_REPORTER")
	node := os.Getenv("CORVINT_NODE")
	for _, c := range []struct {
		name     string
		exit     int
		complete bool
	}{{"mixed", 1, true}, {"pass", 0, true}, {"skip", 0, true}, {"zero", 0, false}, {"setup", 1, false}, {"collection", 1, false}} {
		t.Run(c.name, func(t *testing.T) {
			spec := c.name + ".cjs"
			r := tr.Request{Runner: Runner, Root: root, Executable: exe, ExecutableSha256: hash(exe), Config: config, ConfigSha256: hash(config), Reporter: reporter, ReporterSha256: hash(reporter), Project: endpoint, Target: device, Selectors: []string{spec}, InputFiles: map[string]string{spec: hash(filepath.Join(root, spec)), "launcher.cjs": hash(exe)}, Tools: map[string]tr.Tool{"node": {Executable: node, Sha256: hash(node)}}, ReportDir: filepath.Join(out, c.name), TimeoutSeconds: 75}
			v, e := Build(r)
			if e != nil {
				t.Fatal(e)
			}
			x, execErr := tr.Execute(context.Background(), r, v)
			o, parseErr := Parse(x.Input)
			o = tr.Normalize(x.Input, o)
			raw, e := json.MarshalIndent(struct {
				Request     tr.Request
				Invocation  tr.Invocation
				Execution   tr.Execution
				Observation tr.Observation
				ParseError  string
			}{r, v, x, o, errString(parseErr)}, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(out, c.name+".json"), raw, 0600); e != nil {
				t.Fatal(e)
			}
			if execErr != nil || x.Input.ExitCode != c.exit || o.Complete != c.complete || (c.complete && parseErr != nil) {
				t.Fatalf("exec=%v parse=%v exit=%d observation=%+v", execErr, parseErr, x.Input.ExitCode, o)
			}
		})
	}
}
func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
