package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunsResponseWaitCLI: TSA-V0-200, TSA-V0-204, TSA-V0-205 — the command reads a log relative to
// --root, reports a missing log as UNKNOWN with exit 0 and refuses bad arguments.
func TestRunsResponseWaitCLI(t *testing.T) {
	root := t.TempDir()
	log := `{"time":"2026-10-19T10:00:05.200Z","method":"GET","path":"/api/orders/placeholder","status":404,"durationMs":2}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "requests.jsonl"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--root", root, "runs", "response-wait", "--url-contains", "/api/orders/submit", "--method", "POST",
		"--wait-start", "2026-10-19T10:00:05Z", "--timeout-ms", "30000", "--log"}
	var report struct{ Schema, Verdict, Reason, FailureScope string }
	code, out, errb := runCLI(t, append(args, "requests.jsonl")...)
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != "corvint-runs-response-wait/0" ||
		report.Verdict != "REQUEST_NEVER_SENT" || report.FailureScope != "LOG" {
		t.Fatalf("code=%d out=%s err=%s", code, out, errb)
	}
	code, out, _ = runCLI(t, append(args, "missing.har")...)
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || report.Verdict != "UNKNOWN" || report.Reason != "request-log-unavailable" {
		t.Fatalf("missing: code=%d out=%s", code, out)
	}
	for _, bad := range [][]string{
		{"--root", root, "runs", "response-wait", "--log", "requests.jsonl", "--wait-start", "2026-10-19T10:00:05Z", "--timeout-ms", "1"},
		{"--root", root, "runs", "response-wait", "--log", "requests.jsonl", "--url-contains", "x", "--wait-start", "yesterday", "--timeout-ms", "1"},
		{"--root", root, "runs", "response-wait", "--log", "requests.jsonl", "--url-contains", "x", "--wait-start", "2026-10-19T10:00:05Z", "--timeout-ms", "0"},
	} {
		if code, _, errb := runCLI(t, bad...); code != 2 || !strings.Contains(errb, "runs-invalid-arguments") {
			t.Fatalf("%v: code=%d err=%s", bad, code, errb)
		}
	}
}

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
	// TSA-V0-363: the validity projection lists the profile's warnings with the containing test.
	code, out, errb = runCLI(t, "--root", root, "tests", "validity", "--files", "tests/pay.spec.ts", "--profile", "e2e")
	var validity struct {
		Env struct {
			Profile  string
			Warnings []struct{ Test, Rule string }
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &validity) != nil || validity.Env.Profile != "e2e" || len(validity.Env.Warnings) != 1 ||
		validity.Env.Warnings[0].Test != "tests/pay.spec.ts:1" || validity.Env.Warnings[0].Rule != "ENV-HARD-CODED-ADDRESS" {
		t.Fatalf("validity: code=%d out=%s err=%s", code, out, errb)
	}
	if code, _, errb := runCLI(t, "--root", root, "tests", "validity", "--files", "tests/pay.spec.ts", "--profile", "../x"); code != 2 || !strings.Contains(errb, "tests-invalid-arguments") {
		t.Fatalf("validity bad profile: code=%d err=%s", code, errb)
	}
}
