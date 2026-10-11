package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runsReport is a minimal Playwright json-reporter document with one test whose single result has
// the given status, declaring the run identity inputs.
func runsReport(status, declared string) string {
	testStatus := "expected"
	if status != "passed" {
		testStatus = "unexpected"
	}
	errs := `[]`
	if status == "failed" {
		errs = `[{"message":"Error: expect(locator).toBeVisible() failed","location":{"file":"/w/cart.spec.ts","line":7,"column":3}}]`
	}
	return `{"config":{"version":"1.63.0","rootDir":"/w","workers":1,"metadata":{` + declared + `}},"errors":[],"stats":{},` +
		`"suites":[{"title":"cart.spec.ts","file":"cart.spec.ts","specs":[],"suites":[{"title":"cart","file":"cart.spec.ts","suites":[],` +
		`"specs":[{"title":"adds item","file":"cart.spec.ts","tests":[{"projectName":"chromium","expectedStatus":"passed","status":"` + testStatus +
		`","annotations":[],"results":[{"workerIndex":0,"retry":0,"status":"` + status + `","duration":10,"errors":` + errs + `}]}]}]}]}]}`
}

const runsDeclared = `"corvintRunIdentity":{"applicationRevision":"app-1","configDigest":"cfg-1","testFileBlobs":{"cart.spec.ts":"blob-1"}}`

func writeRunsReports(t *testing.T, dir string, reports map[string]string) {
	t.Helper()
	for name, body := range reports {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunsStabilityCLI(t *testing.T) {
	t.Run("PWR-V0-005 help and dispatch", func(t *testing.T) {
		for _, args := range [][]string{{"runs", "--help"}, {"help", "runs"}} {
			var out, errb bytes.Buffer
			if code := run(args, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "runs stability") || !strings.Contains(out.String(), "EXPECTED_FAIL_NOT_DETERMINISTIC") {
				t.Fatalf("%q: code=%d out=%q err=%q", args, code, out.String(), errb.String())
			}
		}
		var out, errb bytes.Buffer
		if code := run([]string{"--help"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "runs (PWR-V0)") {
			t.Fatalf("root help: code=%d", code)
		}
	})
	t.Run("PWR-V0-003 PWR-V0-005 intermittent run set end to end", func(t *testing.T) {
		dir := t.TempDir()
		writeRunsReports(t, dir, map[string]string{"a.json": runsReport("failed", runsDeclared), "b.json": runsReport("passed", runsDeclared), "c.json": runsReport("failed", runsDeclared)})
		before, _ := os.ReadDir(dir)
		var out, errb bytes.Buffer
		code := run([]string{"--root", dir, "runs", "stability", "--runs", "a.json,b.json", "--runs", "c.json"}, strings.NewReader(""), &out, &errb)
		if code != 0 || errb.Len() != 0 {
			t.Fatalf("code=%d err=%q", code, errb.String())
		}
		var doc struct {
			Profile string `json:"profile"`
			MinRuns int    `json:"minRuns"`
			Tests   []struct {
				Project     string   `json:"project"`
				TitlePath   []string `json:"titlePath"`
				Verdict     string   `json:"verdict"`
				Outcomes    []string `json:"outcomes"`
				FirstErrors []string `json:"firstErrors"`
			} `json:"tests"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Profile != "corvint-runs-stability/0" || doc.MinRuns != 3 || len(doc.Tests) != 1 || doc.Tests[0].Verdict != "INTERMITTENT" ||
			strings.Join(doc.Tests[0].TitlePath, "/") != "cart/adds item" || strings.Join(doc.Tests[0].Outcomes, ",") != "failed,passed,failed" || len(doc.Tests[0].FirstErrors) != 1 {
			t.Fatalf("doc = %s", out.String())
		}
		after, _ := os.ReadDir(dir)
		if len(after) != len(before) {
			t.Fatalf("stability wrote into the report directory")
		}
		out.Reset()
		if code := run([]string{"--root", dir, "runs", "stability", "--runs", "b.json", "--min-runs", "2"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), `"verdict":"INSUFFICIENT","reason":"BELOW_THRESHOLD"`) {
			t.Fatalf("single run: code=%d out=%s", code, out.String())
		}
	})
	t.Run("PWR-V0-005 refusals", func(t *testing.T) {
		dir := t.TempDir()
		writeRunsReports(t, dir, map[string]string{"ok.json": runsReport("passed", runsDeclared), "bad.json": "{"})
		for _, test := range []struct {
			args []string
			code string
		}{
			{[]string{"runs"}, "runs-invalid-arguments"},
			{[]string{"runs", "unknown"}, "runs-invalid-arguments"},
			{[]string{"runs", "stability"}, "runs-invalid-arguments"},
			{[]string{"runs", "stability", "--runs", "a,,b"}, "runs-invalid-arguments"},
			{[]string{"runs", "stability", "--runs", "x.json", "--min-runs", "1"}, "runs-invalid-arguments"},
			{[]string{"runs", "stability", "--runs", "x.json", "--secret=do-not-echo"}, "runs-invalid-arguments"},
			{[]string{"--root", dir, "runs", "stability", "--runs", "ok.json,missing.json"}, "runs-report-unavailable"},
			{[]string{"--root", dir, "runs", "stability", "--runs", "bad.json"}, "runs-report-unavailable"},
		} {
			var out, errb bytes.Buffer
			code := run(test.args, strings.NewReader(""), &out, &errb)
			var diag struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(errb.Bytes(), &diag)
			if code != 2 || out.Len() != 0 || diag.Code != test.code || strings.Contains(errb.String(), "do-not-echo") {
				t.Errorf("%q: code=%d out=%q err=%q", test.args, code, out.String(), errb.String())
			}
		}
	})
}
