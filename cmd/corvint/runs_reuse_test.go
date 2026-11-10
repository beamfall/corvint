package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runsReuseReport is runsReport with a test-suite revision, a start time and a rootDir.
func runsReuseReport(status, app, testRev, rootDir, start string) string {
	declared := `"corvintRunIdentity":{"applicationRevision":"` + app + `","testRevision":"` + testRev + `","configDigest":"cfg-1","testFileBlobs":{"cart.spec.ts":"blob-1"}}`
	body := runsReport(status, declared)
	if start != "" {
		body = strings.Replace(body, `"stats":{}`, `"stats":{"startTime":"`+start+`"}`, 1)
	}
	return strings.Replace(body, `"rootDir":"/w"`, `"rootDir":`+jsonString(rootDir), 1)
}

func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func runsCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func reuseGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	prefix := []string{"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-C", root}
	out, err := fixtureGitCommand(append(prefix, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func reuseCommit(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		reuseGit(t, root, "add", "--", name)
	}
	reuseGit(t, root, "commit", "-qm", "change")
	return reuseGit(t, root, "rev-parse", "HEAD")
}

type reuseDoc struct {
	Profile string `json:"profile"`
	Form    string `json:"form"`
	Tests   []struct {
		Decision string   `json:"decision"`
		Reason   string   `json:"reason"`
		Detail   string   `json:"detail"`
		Fields   []string `json:"fields"`
		Paths    []string `json:"paths"`
	} `json:"tests"`
	Summary struct {
		Selected  int    `json:"selected"`
		Reusable  int    `json:"reusable"`
		MustRun   int    `json:"mustRun"`
		Freshness string `json:"freshness"`
	} `json:"summary"`
}

func decodeRuns(t *testing.T, raw string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), into); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
}

func TestRunsReusableCLI(t *testing.T) {
	repo := t.TempDir()
	reuseGit(t, repo, "init", "-q")
	reuseGit(t, repo, "config", "user.email", "corvint@example.test")
	reuseGit(t, repo, "config", "user.name", "Corvint Test")
	a := reuseCommit(t, repo, map[string]string{
		"e2e/cart.spec.ts": "t0\n", "src/app.ts": "v0\n",
	})
	b := reuseCommit(t, repo, map[string]string{"src/app.ts": "v1\n"})
	e2e := filepath.Join(repo, "e2e")
	writeRunsReports(t, repo, map[string]string{
		"prior.json": runsReuseReport("passed", a, a, e2e, ""),
		"sel-b.json": runsReuseReport("passed", b, a, e2e, ""),
		"sel-a.json": runsReuseReport("passed", a, a, e2e, ""),
	})
	before := reuseGit(t, repo, "status", "--porcelain")

	t.Run("PWR-V0-034 PWR-V0-040 an exact qualified pass is REUSABLE", func(t *testing.T) {
		code, out, errs := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-a.json")
		var doc reuseDoc
		decodeRuns(t, out, &doc)
		if code != 0 || errs != "" || doc.Profile != "corvint-runs-reusable/0" || doc.Form != "full" || len(doc.Tests) != 1 ||
			doc.Tests[0].Decision != "REUSABLE" || doc.Tests[0].Reason != "EXACT_PASS" || doc.Summary.Freshness != "DISABLED" {
			t.Fatalf("code=%d out=%s err=%s", code, out, errs)
		}
		if strings.Count(out, `"servedDigest":"UNKNOWN"`) != 2 || strings.Contains(out, `"servedDigest":""`) {
			t.Fatalf("PWR-V0-057 servedDigest: %s", out)
		}
	})
	t.Run("PWR-V0-035 a revision change is MUST_RUN", func(t *testing.T) {
		_, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-b.json")
		var doc reuseDoc
		decodeRuns(t, out, &doc)
		if doc.Tests[0].Reason != "FINGERPRINT_CHANGED" || strings.Join(doc.Tests[0].Fields, ",") != "applicationRevision" {
			t.Fatalf("out=%s", out)
		}
	})
	t.Run("PWR-V0-040 the reads leave the repository unchanged", func(t *testing.T) {
		if after := reuseGit(t, repo, "status", "--porcelain"); after != before {
			t.Fatalf("status changed: %q -> %q", before, after)
		}
	})
}

func TestRunsRedundancyCLI(t *testing.T) {
	dir := t.TempDir()
	writeRunsReports(t, dir, map[string]string{
		"r1.json": runsReuseReport("passed", "app-1", "tests-1", "/w", "2026-10-01T10:00:00Z"),
		"r2.json": runsReuseReport("passed", "app-1", "tests-1", "/w", "2026-10-01T11:00:00Z"),
		"r3.json": runsReuseReport("passed", "app-2", "tests-1", "/w", "2026-10-01T12:00:00Z"),
	})
	t.Run("PWR-V0-037 PWR-V0-040 exact and same-file-content re-runs", func(t *testing.T) {
		var doc struct {
			Profile string `json:"profile"`
			Form    string `json:"form"`
			Runs    []struct {
				Role string `json:"role"`
			} `json:"runs"`
			Total struct {
				Runs                      int     `json:"runs"`
				TotalMS                   int64   `json:"totalMs"`
				ExactRerunTests           int     `json:"exactRerunTests"`
				SameFileContentRerunTests int     `json:"sameFileContentRerunTests"`
				ExactRerunShare           float64 `json:"exactRerunShare"`
			} `json:"total"`
		}
		code, out, errs := runsCLI(t, "--root", dir, "runs", "redundancy", "--runs", "r1.json,r2.json", "--runs", "r3.json", "--since", "2026-10-01T10:30:00Z")
		decodeRuns(t, out, &doc)
		if code != 0 || errs != "" || doc.Profile != "corvint-runs-redundancy/0" || len(doc.Runs) != 3 || doc.Runs[0].Role != "HISTORY" ||
			doc.Total.Runs != 2 || doc.Total.TotalMS != 20 || doc.Total.ExactRerunTests != 1 || doc.Total.SameFileContentRerunTests != 1 || doc.Total.ExactRerunShare != 0.5 {
			t.Fatalf("code=%d out=%s err=%s", code, out, errs)
		}
		_, out, _ = runsCLI(t, "--root", dir, "runs", "redundancy", "--runs", "r1.json,r2.json,r3.json", "--summary")
		doc.Runs = nil
		decodeRuns(t, out, &doc)
		if doc.Form != "summary" || doc.Runs != nil || doc.Total.Runs != 3 {
			t.Fatalf("summary: %s", out)
		}
	})
	t.Run("PWR-V0-040 refusals", func(t *testing.T) {
		writeRunsReports(t, dir, map[string]string{"bad.json": "{"})
		for _, test := range []struct {
			args []string
			code string
		}{
			{[]string{"runs", "reusable"}, "runs-invalid-arguments"},
			{[]string{"runs", "reusable", "--selection", "a.json,b.json"}, "runs-invalid-arguments"},
			{[]string{"runs", "reusable", "--selection", "a.json", "extra"}, "runs-invalid-arguments"},
			{[]string{"--root", dir, "runs", "reusable", "--selection", "r1.json", "--prior", "missing.json"}, "runs-report-unavailable"},
			{[]string{"--root", dir, "runs", "reusable", "--selection", "bad.json"}, "runs-report-unavailable"},
			{[]string{"runs", "redundancy"}, "runs-invalid-arguments"},
			{[]string{"runs", "redundancy", "--runs", "r1.json", "--since", "yesterday"}, "runs-invalid-arguments"},
			{[]string{"runs", "redundancy", "--runs", "r1.json", "--since", "2026-10-01T00:00:00Z", "--since", "2026-10-02T00:00:00Z"}, "runs-invalid-arguments"},
			{[]string{"--root", dir, "runs", "redundancy", "--runs", "r1.json,bad.json"}, "runs-report-unavailable"},
			{[]string{"runs", "fresh", "--old", "HEAD"}, "runs-invalid-arguments"},
			{[]string{"runs", "identity", "--config", "playwright.config.ts"}, "runs-invalid-arguments"},
		} {
			code, out, errs := runsCLI(t, test.args...)
			var diag struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal([]byte(errs), &diag)
			if code != 2 || out != "" || diag.Code != test.code {
				t.Errorf("%q: code=%d out=%q err=%q", test.args, code, out, errs)
			}
		}
	})
	t.Run("PWR-V0-040 help names the new reads", func(t *testing.T) {
		_, out, _ := runsCLI(t, "runs", "--help")
		for _, want := range []string{"runs reusable", "runs redundancy"} {
			if !strings.Contains(out, want) {
				t.Fatalf("help lacks %q", want)
			}
		}
	})
}
