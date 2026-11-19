package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRunsResponseWaitCLI: TSA-V0-200, TSA-V0-204, TSA-V0-205 — the command reads a log relative to
// --root, reports a missing log as UNKNOWN with exit 0 and refuses bad arguments.

// TestTestsRoleLocatorsAndEnvCLI: TSA-V0-206, TSA-V0-210 — role-locators exits 1 on a finding and
// env prints the profile's contract; an invalid profile name is refused.
func TestTestsRoleLocatorsAndEnvCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{
		"tests/banner.spec.tsx":    "import ErrorBanner from '../src/ErrorBanner';\ntest('t', async ({ mount }) => { const c = await mount(<ErrorBanner />); await c.getByRole('alert').click(); });\n",
		"src/ErrorBanner.tsx":      "export default () => <div className=\"error\">boom</div>;\n",
		"scripts/bootstrap-e2e.sh": "export STUB_URL=http://localhost:4110\nPORT=4110 node stubs/pay/server.js &\n",
		"stubs/pay/server.js":      "app.post('/__control/faults', (req, res) => res.end());\n",
		"tests/pay.spec.ts":        "test('pays', async () => {\n  await fetch('http://localhost:4110/x');\n});\n",
	})
	code, out, errb := runCLI(t, "--root", root, "tests", "role-locators")
	if code != 1 || !strings.Contains(out, "ROLE-LOCATOR-UNRENDERED") || !strings.Contains(out, "src/ErrorBanner.tsx") {
		t.Fatalf("role-locators: code=%d out=%s err=%s", code, out, errb)
	}
	code, out, errb = runCLI(t, "--root", root, "tests", "env", "--profile", "e2e")
	var report struct {
		Schema   string
		StandIns []struct {
			URL    string
			Routes []struct{ Path, Kind string }
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != "corvint-tests-env/0" || len(report.StandIns) != 1 ||
		report.StandIns[0].URL != "http://localhost:4110" || report.StandIns[0].Routes[0].Kind != "FAULT_INJECTION" {
		t.Fatalf("env: code=%d out=%s err=%s", code, out, errb)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "env", "--profile", "../x"); code != 2 || !strings.Contains(errb, "tests-invalid-arguments") {
		t.Fatalf("bad profile: code=%d err=%s", code, errb)
	}
}
