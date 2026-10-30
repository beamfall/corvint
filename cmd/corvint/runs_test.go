package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	return `{"config":{"version":"1.64.0","rootDir":"/w","workers":1,"metadata":{` + declared + `}},"errors":[],"stats":{},` +
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
		if code := run([]string{"--help"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "\n    runs  ") || !strings.Contains(out.String(), " (PWR-V0)\n") {
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

func TestRunsCompareCLI(t *testing.T) {
	dir := t.TempDir()
	writeRunsReports(t, dir, map[string]string{"base1.json": runsReport("passed", runsDeclared), "base2.json": runsReport("passed", runsDeclared),
		"same.json": runsReport("passed", runsDeclared), "fail.json": runsReport("failed", runsDeclared)})
	compare := func(t *testing.T, args ...string) (int, map[string]any, string) {
		t.Helper()
		var out, errb bytes.Buffer
		code := run(append([]string{"--root", dir, "runs", "compare"}, args...), strings.NewReader(""), &out, &errb)
		doc := map[string]any{}
		if out.Len() > 0 {
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
		}
		return code, doc, errb.String()
	}
	t.Run("PWR-V0-010 help and verdicts end to end", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := run([]string{"runs", "--help"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "runs compare --baseline") || !strings.Contains(out.String(), "INCOMPARABLE") {
			t.Fatalf("help: code=%d", code)
		}
		before, _ := os.ReadDir(dir)
		for _, c := range []struct {
			args    []string
			verdict string
		}{
			{[]string{"--baseline", "base1.json,base2.json", "--candidate", "same.json"}, "PARITY"},
			{[]string{"--baseline", "base1.json", "--candidate", "fail.json"}, "DIVERGED"},
			{[]string{"--baseline", "base1.json", "--candidate", "absent.json"}, "INCOMPARABLE"},
		} {
			code, doc, diag := compare(t, c.args...)
			if code != 0 || diag != "" || doc["profile"] != "corvint-runs-compare/0" || doc["verdict"] != c.verdict {
				t.Fatalf("%q: code=%d doc=%v diag=%q", c.args, code, doc, diag)
			}
		}
		_, doc, _ := compare(t, "--baseline", "base1.json", "--candidate", "absent.json")
		problems, _ := doc["problems"].([]any)
		if len(problems) != 1 || problems[0].(map[string]any)["reason"] != "REPORT_UNREADABLE" || problems[0].(map[string]any)["side"] != "candidate" {
			t.Fatalf("problems = %v", doc["problems"])
		}
		after, _ := os.ReadDir(dir)
		if len(after) != len(before) {
			t.Fatalf("compare wrote into the report directory")
		}
	})
	t.Run("PWR-V0-010 refusals", func(t *testing.T) {
		for _, args := range [][]string{{}, {"--baseline", "base1.json"}, {"--candidate", "same.json"}, {"--baseline", "base1.json", "--candidate", "same.json", "extra"},
			{"--baseline", ",", "--candidate", "same.json"}, {"--baseline", "base1.json", "--candidate", "same.json", "--token=do-not-echo"}} {
			code, doc, diag := compare(t, args...)
			if code != 2 || len(doc) != 0 || !strings.Contains(diag, `"code":"runs-invalid-arguments"`) || strings.Contains(diag, "do-not-echo") {
				t.Errorf("%q: code=%d doc=%v diag=%q", args, code, doc, diag)
			}
		}
	})
}

// runsShardReport adds config.shard to a runsReport document; an empty shard has no suites.
func runsShardReport(body string, current, total int, empty bool) string {
	body = strings.Replace(body, `"workers":1,`, `"workers":1,"shard":{"current":`+strconv.Itoa(current)+`,"total":`+strconv.Itoa(total)+`},`, 1)
	if empty {
		body = body[:strings.Index(body, `"suites":[{"title":"cart.spec.ts"`)] + `"suites":[]}`
	}
	return body
}

// runsProjectReport is runsReport declaring its one configured project, as runsRepeatReport does.
func runsProjectReport(status string) string {
	return strings.Replace(runsReport(status, runsDeclared), `"workers":1,`, `"workers":1,"projects":[{"name":"chromium","repeatEach":1}],`, 1)
}

// runsRepeatReport is runsReport run with --repeat-each 2: one spec entry per repeat, each at the
// same reported location.
func runsRepeatReport(first, second string) string {
	spec := func(status string) string {
		one := runsReport(status, runsDeclared)
		start := strings.Index(one, `{"title":"adds item"`)
		return strings.Replace(one[start:len(one)-len(`]}]}]}`)], `"file":"cart.spec.ts",`, `"file":"cart.spec.ts","line":3,"column":3,`, 1)
	}
	head := runsReport(first, runsDeclared)
	head = strings.Replace(head, `"workers":1,`, `"workers":1,"projects":[{"name":"chromium","repeatEach":2}],`, 1)
	return head[:strings.Index(head, `{"title":"adds item"`)] + spec(first) + "," + spec(second) + `]}]}]}`
}

func TestRunsRepeatAndShardCLI(t *testing.T) {
	dir := t.TempDir()
	writeRunsReports(t, dir, map[string]string{
		"whole.json":  runsReport("passed", runsDeclared),
		"s1.json":     runsShardReport(runsReport("passed", runsDeclared), 1, 2, false),
		"s2.json":     runsShardReport(runsReport("passed", runsDeclared), 2, 2, true),
		"repeat.json": runsRepeatReport("passed", "failed"),
		"failed.json": runsProjectReport("failed"),
		"passed.json": runsProjectReport("passed"),
	})
	cli := func(t *testing.T, args ...string) map[string]any {
		t.Helper()
		var out, errb bytes.Buffer
		if code := run(append([]string{"--root", dir, "runs"}, args...), strings.NewReader(""), &out, &errb); code != 0 || errb.Len() != 0 {
			t.Fatalf("%q: code=%d err=%q", args, code, errb.String())
		}
		doc := map[string]any{}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	only := func(t *testing.T, doc map[string]any) map[string]any {
		t.Helper()
		tests, _ := doc["tests"].([]any)
		if len(tests) != 1 {
			t.Fatalf("tests = %v", doc["tests"])
		}
		return tests[0].(map[string]any)
	}
	t.Run("PWR-V0-023 help names repeats, shard sets and refusals", func(t *testing.T) {
		var out, errb bytes.Buffer
		run([]string{"runs", "--help"}, strings.NewReader(""), &out, &errb)
		for _, want := range []string{"--repeat-each", "repeats[]", "shardSets[]", "SHARD_INPUTS_NOT_DECLARED", "SHARD_SET_INCOMPLETE", "SHARD_DUPLICATE", "merge-reports"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("help lacks %q", want)
			}
		}
	})
	t.Run("PWR-V0-011 PWR-V0-014 PWR-V0-023 one repeat-each report end to end", func(t *testing.T) {
		e := only(t, cli(t, "stability", "--runs", "repeat.json", "--min-runs", "2"))
		if e["verdict"] != "INTERMITTENT" || fmt.Sprint(e["repeats"]) != "[0 1]" || fmt.Sprint(e["runs"]) != "[1 1]" {
			t.Fatalf("entry = %v", e)
		}
		doc := cli(t, "compare", "--baseline", "passed.json,failed.json", "--candidate", "repeat.json")
		cand := only(t, doc)["candidate"].(map[string]any)
		if doc["verdict"] != "PARITY" || cand["stability"] != "1/2" {
			t.Fatalf("compare = %v", doc)
		}
	})
	t.Run("PWR-V0-017 PWR-V0-019 PWR-V0-020 PWR-V0-023 shard sets end to end", func(t *testing.T) {
		before, _ := os.ReadDir(dir)
		e := only(t, cli(t, "stability", "--runs", "s2.json,s1.json", "--runs", "whole.json", "--min-runs", "2"))
		if e["verdict"] != "DETERMINISTIC" || fmt.Sprint(e["runs"]) != "[1 3]" {
			t.Fatalf("complete set: %v", e)
		}
		doc := cli(t, "stability", "--runs", "s1.json", "--min-runs", "2")
		set := doc["shardSets"].([]any)[0].(map[string]any)
		if e := only(t, doc); e["verdict"] != "INSUFFICIENT" || e["reason"] != "SHARD_SET_INCOMPLETE" || fmt.Sprint(set["missing"]) != "[2]" {
			t.Fatalf("incomplete set: %v %v", e, set)
		}
		doc = cli(t, "compare", "--baseline", "whole.json", "--candidate", "s1.json,s1.json,s2.json")
		if doc["verdict"] != "INCOMPARABLE" || fmt.Sprint(doc["reasons"]) != "[SHARD_DUPLICATE]" {
			t.Fatalf("duplicate set: %v", doc)
		}
		if doc = cli(t, "compare", "--baseline", "whole.json", "--candidate", "s1.json,s2.json"); doc["verdict"] != "PARITY" {
			t.Fatalf("complete set compare: %v", doc)
		}
		if after, _ := os.ReadDir(dir); len(after) != len(before) {
			t.Fatalf("runs wrote into the report directory")
		}
	})
}
