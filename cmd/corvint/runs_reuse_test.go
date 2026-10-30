package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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
	out, err := exec.Command("git", append(prefix, args...)...).CombinedOutput()
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
		".corvint/evidence-reach.json": `{"profile":"corvint-evidence-reach/0","shared":["e2e/support/"]}`,
		"e2e/cart.spec.ts":             "t0\n", "src/app.ts": "v0\n",
	})
	b := reuseCommit(t, repo, map[string]string{"src/app.ts": "v1\n"})
	c := reuseCommit(t, repo, map[string]string{"e2e/support/login.ts": "l1\n"})
	e2e := filepath.Join(repo, "e2e")
	writeRunsReports(t, repo, map[string]string{
		"prior.json":     runsReuseReport("passed", a, a, e2e, ""),
		"sel-b.json":     runsReuseReport("passed", b, a, e2e, ""),
		"sel-c.json":     runsReuseReport("passed", c, a, e2e, ""),
		"sel-a.json":     runsReuseReport("passed", a, a, e2e, ""),
		"elsewhere.json": runsReuseReport("passed", b, a, "/somewhere/else", ""),
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
	t.Run("PWR-V0-035 a revision change without --fresh is MUST_RUN", func(t *testing.T) {
		_, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-b.json")
		var doc reuseDoc
		decodeRuns(t, out, &doc)
		if doc.Tests[0].Reason != "FINGERPRINT_CHANGED" || strings.Join(doc.Tests[0].Fields, ",") != "applicationRevision" {
			t.Fatalf("out=%s", out)
		}
	})
	t.Run("PWR-V0-036 --fresh admits FRESH_PASS and refuses STALE through the declared reach", func(t *testing.T) {
		_, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-b.json", "--fresh")
		var doc reuseDoc
		decodeRuns(t, out, &doc)
		if doc.Tests[0].Decision != "REUSABLE" || doc.Tests[0].Reason != "FRESH_PASS" || doc.Summary.Freshness != "ENABLED" {
			t.Fatalf("fresh: %s", out)
		}
		_, out, _ = runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-c.json", "--fresh")
		doc = reuseDoc{}
		decodeRuns(t, out, &doc)
		if doc.Tests[0].Reason != "STALE" || strings.Join(doc.Tests[0].Paths, ",") != "e2e/support/login.ts" {
			t.Fatalf("stale: %s", out)
		}
		_, out, _ = runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "elsewhere.json", "--fresh")
		doc = reuseDoc{}
		decodeRuns(t, out, &doc)
		if doc.Tests[0].Reason != "FRESHNESS_UNRESOLVED" || doc.Tests[0].Detail != "TEST_ROOT_UNRESOLVED" {
			t.Fatalf("unlocated: %s", out)
		}
		code, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "prior.json", "--selection", "sel-b.json", "--fresh", "--summary")
		doc = reuseDoc{}
		decodeRuns(t, out, &doc)
		if code != 0 || doc.Form != "summary" || doc.Tests != nil || doc.Summary.Reusable != 1 {
			t.Fatalf("summary: %s", out)
		}
	})
	t.Run("PWR-V0-036 an absolute, escaping or backslash test file is TEST_ROOT_UNRESOLVED", func(t *testing.T) {
		for i, name := range []string{"/outside/cart.spec.ts", "../cart.spec.ts", "sub/../../cart.spec.ts", `..\cart.spec.ts`, `\outside\cart.spec.ts`, `C:\outside\cart.spec.ts`} {
			rename := func(body string) string { return strings.ReplaceAll(body, `"cart.spec.ts"`, jsonString(name)) }
			prior, sel := "odd-prior-"+string(rune('a'+i))+".json", "odd-sel-"+string(rune('a'+i))+".json"
			writeRunsReports(t, repo, map[string]string{
				prior: rename(runsReuseReport("passed", a, a, e2e, "")),
				sel:   rename(runsReuseReport("passed", b, a, e2e, "")),
			})
			t.Cleanup(func() { _ = os.Remove(filepath.Join(repo, prior)); _ = os.Remove(filepath.Join(repo, sel)) })
			_, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", prior, "--selection", sel, "--fresh")
			var doc reuseDoc
			decodeRuns(t, out, &doc)
			if len(doc.Tests) != 1 || doc.Tests[0].Decision != "MUST_RUN" || doc.Tests[0].Detail != "TEST_ROOT_UNRESOLVED" {
				t.Fatalf("%s: %s", name, out)
			}
		}
	})
	t.Run("PWR-V0-036 a report from another machine resolves through its declared testRootPath", func(t *testing.T) {
		// V1-1611: config.rootDir is another machine's path, so only testRootPath can locate the file.
		withRoot := func(body, rootPath string) string {
			return strings.Replace(body, `"testRevision":`, `"testRootPath":`+jsonString(rootPath)+`,"testRevision":`, 1)
		}
		foreign := "/home/ci/build/e2e"
		writeRunsReports(t, repo, map[string]string{
			"ci-prior.json":    withRoot(runsReuseReport("passed", a, a, foreign, ""), "e2e"),
			"ci-sel-b.json":    withRoot(runsReuseReport("passed", b, a, foreign, ""), "e2e"),
			"ci-sel-c.json":    withRoot(runsReuseReport("passed", c, a, foreign, ""), "e2e"),
			"ci-sel-bad.json":  withRoot(runsReuseReport("passed", b, a, foreign, ""), "../e2e"),
			"ci-sel-none.json": runsReuseReport("passed", b, a, foreign, ""),
		})
		t.Cleanup(func() {
			for _, n := range []string{"ci-prior.json", "ci-sel-b.json", "ci-sel-c.json", "ci-sel-bad.json", "ci-sel-none.json"} {
				_ = os.Remove(filepath.Join(repo, n))
			}
		})
		want := map[string]string{"ci-sel-b.json": "FRESH_PASS", "ci-sel-c.json": "STALE", "ci-sel-bad.json": "FRESHNESS_UNRESOLVED", "ci-sel-none.json": "FRESHNESS_UNRESOLVED"}
		for sel, reason := range want {
			_, out, _ := runsCLI(t, "--root", repo, "runs", "reusable", "--prior", "ci-prior.json", "--selection", sel, "--fresh")
			var doc reuseDoc
			decodeRuns(t, out, &doc)
			if len(doc.Tests) != 1 || doc.Tests[0].Reason != reason {
				t.Fatalf("%s: want %s: %s", sel, reason, out)
			}
			if reason == "FRESHNESS_UNRESOLVED" && doc.Tests[0].Detail != "TEST_ROOT_UNRESOLVED" {
				t.Fatalf("%s: untyped unresolved: %s", sel, out)
			}
		}
	})
	t.Run("PWR-V0-039 PWR-V0-040 runs fresh reads the declared reach", func(t *testing.T) {
		var res struct {
			Verdict  string `json:"verdict"`
			Reason   string `json:"reason"`
			Deciding []struct {
				Path string `json:"path"`
				By   string `json:"by"`
			} `json:"deciding"`
		}
		code, out, _ := runsCLI(t, "--root", repo, "runs", "fresh", "--old", b, "--new", c, "--cite", "e2e/cart.spec.ts")
		decodeRuns(t, out, &res)
		if code != 0 || res.Verdict != "STALE" || len(res.Deciding) != 1 || res.Deciding[0].Path != "e2e/support/login.ts" || res.Deciding[0].By != "shared" {
			t.Fatalf("stale: %s", out)
		}
		res.Deciding = nil
		_, out, _ = runsCLI(t, "--root", repo, "runs", "fresh", "--old", a, "--new", b, "--cite", "e2e/cart.spec.ts", "--summary")
		decodeRuns(t, out, &res)
		if res.Verdict != "FRESH" || strings.Contains(out, `"deciding"`) {
			t.Fatalf("fresh: %s", out)
		}
		code, out, _ = runsCLI(t, "--root", repo, "runs", "fresh", "--old", "missing-rev", "--new", c)
		decodeRuns(t, out, &res)
		if code != 0 || res.Verdict != "UNRESOLVED" || res.Reason != "REVISION_UNRESOLVED" {
			t.Fatalf("unresolved: %s", out)
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
			{[]string{"runs", "fresh", "--old", "HEAD", "--new", "HEAD", "--cite", "../x"}, "runs-invalid-arguments"},
			{[]string{"runs", "fresh", "--old", "HEAD", "--old", "HEAD~1", "--new", "HEAD"}, "runs-invalid-arguments"},
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
		for _, want := range []string{"runs reusable", "runs redundancy", "runs fresh", "evidence-reach.json"} {
			if !strings.Contains(out, want) {
				t.Fatalf("help lacks %q", want)
			}
		}
	})
}
