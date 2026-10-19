package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTestsUnselectedCLI prints the selection report and exits 1 on an unselected file, 0 when
// every test-looking file is selected, and 2 on a bad argument (TSA-V0-145).
func TestTestsUnselectedCLI(t *testing.T) {
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	testsCommit(t, root, map[string]string{
		"package.json":      `{"devDependencies":{"jest":"29.7.0"}}`,
		"jest.config.js":    "module.exports = { testPathIgnorePatterns: ['/node_modules/', '/legacy/'] };\n",
		"src/a.test.js":     "test('a', () => { expect(1).toBe(1); });\n",
		"legacy/b.test.js":  "test('b', () => { expect(1).toBe(1); });\n",
		"e2e/login.spec.ts": "import { test } from '@playwright/test';\ntest('login', async () => {});\n",
	})
	code, out, errb := runCLI(t, "--root", root, "tests", "unselected")
	var report struct {
		Schema   string
		Findings []struct{ Code, Path, Reason, Pattern string }
	}
	if code != 1 || json.Unmarshal([]byte(out), &report) != nil || report.Schema != "corvint-test-file-selection/0" {
		t.Fatalf("code=%d out=%s err=%s", code, out, errb)
	}
	got := []string{}
	for _, f := range report.Findings {
		got = append(got, f.Code+" "+f.Path+" "+f.Reason+" "+f.Pattern)
	}
	want := "TEST-FILE-UNSELECTED e2e/login.spec.ts no-runner-for-framework |TEST-FILE-UNSELECTED legacy/b.test.js excluded /legacy/"
	if strings.Join(got, "|") != want {
		t.Errorf("findings = %q", got)
	}
	code, out, _ = runCLI(t, "--root", root, "tests", "validity", "--files", "legacy/b.test.js")
	if code != 1 || !strings.Contains(out, `"reason": "TEST-FILE-UNSELECTED"`) {
		t.Errorf("validity of an unselected file: code=%d out=%s", code, out)
	}
	testsCommit(t, root, map[string]string{"jest.config.js": "module.exports = {};\n", "e2e/login.spec.ts": "test('login', () => {});\n"})
	if code, out, errb = runCLI(t, "--root", root, "tests", "unselected"); code != 0 {
		t.Errorf("all selected: code=%d out=%s err=%s", code, out, errb)
	}
	if code, _, errb = runCLI(t, "--root", root, "tests", "unselected", "extra"); code != 2 || !strings.Contains(errb, "tests-invalid-arguments") {
		t.Errorf("bad argument: code=%d err=%s", code, errb)
	}
}
