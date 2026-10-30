package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunsIdentityCLI(t *testing.T) {
	t.Run("PWR-V0-046 help names the producer", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := run([]string{"runs", "--help"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "runs identity --config FILE") || !strings.Contains(out.String(), "runs-identity-unavailable") {
			t.Fatalf("code=%d out=%q err=%q", code, out.String(), errb.String())
		}
	})

	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}
	reuseGit(t, repo, "init", "-q")
	head := reuseCommit(t, repo, map[string]string{"playwright.config.ts": "export default {}\n", "cart.spec.ts": "test('adds item', () => {})\n", "pages/cart.ts": "export {}\n"})

	t.Run("PWR-V0-046 PWR-V0-051 the declaration pools a report set", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := run([]string{"--root", repo, "runs", "identity", "--config", "playwright.config.ts"}, strings.NewReader(""), &out, &errb); code != 0 || errb.Len() != 0 {
			t.Fatalf("identity: code=%d out=%q err=%q", code, out.String(), errb.String())
		}
		var doc struct {
			Profile  string          `json:"profile"`
			Identity json.RawMessage `json:"corvintRunIdentity"`
			Limits   []string        `json:"limits"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		var id struct {
			ApplicationRevision string            `json:"applicationRevision"`
			TestFileBlobs       map[string]string `json:"testFileBlobs"`
			TestRootPath        string            `json:"testRootPath"`
		}
		if err := json.Unmarshal(doc.Identity, &id); err != nil {
			t.Fatal(err)
		}
		if doc.Profile != "corvint-run-identity/0" || len(doc.Limits) != 0 || id.ApplicationRevision != head || id.TestRootPath != "." || len(id.TestFileBlobs) != 1 || id.TestFileBlobs["cart.spec.ts"] == "" {
			t.Fatalf("document = %s", out.String())
		}
		dir := t.TempDir()
		declared := `"corvintRunIdentity":` + string(doc.Identity)
		writeRunsReports(t, dir, map[string]string{"a.json": runsReport("passed", declared), "b.json": runsReport("passed", declared), "c.json": runsReport("passed", declared)})
		out.Reset()
		errb.Reset()
		if code := run([]string{"--root", dir, "runs", "stability", "--runs", "a.json,b.json,c.json"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), `"verdict":"DETERMINISTIC"`) || strings.Contains(out.String(), "identityNotDeclaredRuns") {
			t.Fatalf("stability: code=%d out=%q err=%q", code, out.String(), errb.String())
		}
	})

	t.Run("PWR-V0-050 refusals", func(t *testing.T) {
		for _, args := range [][]string{
			{"--root", repo, "runs", "identity", "--config", "missing.ts"},
			{"--root", repo, "runs", "identity", "--config", "playwright.config.ts", "--test-match", "("},
			{"--root", repo, "runs", "identity", "--config", "playwright.config.ts", "--app-root", t.TempDir()},
		} {
			var out, errb bytes.Buffer
			code := run(args, strings.NewReader(""), &out, &errb)
			var diag struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(errb.Bytes(), &diag)
			if code != 2 || out.Len() != 0 || diag.Code != "runs-identity-unavailable" {
				t.Errorf("%q: code=%d out=%q err=%q", args, code, out.String(), errb.String())
			}
		}
		var out, errb bytes.Buffer
		if code := run([]string{"--root", repo, "runs", "identity"}, strings.NewReader(""), &out, &errb); code != 2 || !strings.Contains(errb.String(), "runs-invalid-arguments") {
			t.Errorf("missing --config: code=%d err=%q", code, errb.String())
		}
	})
}

// TestRunsIdentitySetupProjectPools is GitHub #1866 end to end: a setup project whose testDir sits
// beside the top-level one (testdata/runs-identity-setup-project mirrors the issue's config) is
// reported as ../setup/global.setup.ts, and with --project-test-dir its tests pool in stability.
func TestRunsIdentitySetupProjectPools(t *testing.T) {
	fixture := filepath.Join("testdata", "runs-identity-setup-project")
	files := map[string]string{}
	if err := filepath.WalkDir(filepath.Join(fixture, "repo"), func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(name)
		rel, _ := filepath.Rel(filepath.Join(fixture, "repo"), name)
		files[filepath.ToSlash(rel)] = string(body)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	template, err := os.ReadFile(filepath.Join(fixture, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}
	reuseGit(t, repo, "init", "-q")
	reuseCommit(t, repo, files)

	type entry struct {
		File          string   `json:"file"`
		Project       string   `json:"project"`
		Verdict       string   `json:"verdict"`
		Reason        string   `json:"reason"`
		MissingInputs []string `json:"missingInputs"`
	}
	stability := func(t *testing.T, extra ...string) (map[string]entry, map[string]string) {
		t.Helper()
		var out, errb bytes.Buffer
		args := append([]string{"--root", repo, "runs", "identity", "--config", "playwright.config.ts", "--test-dir", "tests"}, extra...)
		if code := run(args, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("identity: code=%d err=%q", code, errb.String())
		}
		var doc struct {
			Identity json.RawMessage `json:"corvintRunIdentity"`
		}
		var id struct {
			TestFileBlobs map[string]string `json:"testFileBlobs"`
		}
		if json.Unmarshal(out.Bytes(), &doc) != nil || json.Unmarshal(doc.Identity, &id) != nil {
			t.Fatalf("identity document = %s", out.String())
		}
		report := strings.ReplaceAll(string(template), `"__IDENTITY__"`, string(doc.Identity))
		dir := t.TempDir()
		writeRunsReports(t, dir, map[string]string{"a.json": report, "b.json": report, "c.json": report})
		out.Reset()
		errb.Reset()
		if code := run([]string{"--root", dir, "runs", "stability", "--runs", "a.json,b.json,c.json"}, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("stability: code=%d err=%q", code, errb.String())
		}
		var got struct {
			Tests []entry `json:"tests"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		byProject := map[string]entry{}
		for _, e := range got.Tests {
			byProject[e.Project] = e
		}
		return byProject, id.TestFileBlobs
	}

	t.Run("PWR-V0-053 without the project testDir the setup test names its missing blob", func(t *testing.T) {
		tests, _ := stability(t)
		if e := tests["setup"]; e.Verdict != "INSUFFICIENT" || e.Reason != "IDENTITY_INPUT_MISSING" || !reflect.DeepEqual(e.MissingInputs, []string{"testFileBlobs[../setup/global.setup.ts]"}) {
			t.Fatalf("setup entry = %+v", e)
		}
		if e := tests["chromium"]; e.Verdict != "DETERMINISTIC" || e.MissingInputs != nil {
			t.Fatalf("chromium entry = %+v", e)
		}
	})
	t.Run("PWR-V0-052 with --project-test-dir the setup test pools", func(t *testing.T) {
		tests, blobs := stability(t, "--project-test-dir", `setup:.*\.setup\.ts`)
		if len(blobs) != 2 || blobs["../setup/global.setup.ts"] != reuseGit(t, repo, "rev-parse", "HEAD:setup/global.setup.ts") {
			t.Fatalf("testFileBlobs = %v", blobs)
		}
		if e := tests["setup"]; e.Verdict != "DETERMINISTIC" || e.File != "../setup/global.setup.ts" {
			t.Fatalf("setup entry = %+v", e)
		}
		if e := tests["chromium"]; e.Verdict != "DETERMINISTIC" {
			t.Fatalf("chromium entry = %+v", e)
		}
	})
	t.Run("PWR-V0-052 PWR-V0-053 argument and refusal", func(t *testing.T) {
		for _, bad := range []string{"", ":x", "setup:"} {
			var out, errb bytes.Buffer
			if code := run([]string{"--root", repo, "runs", "identity", "--config", "playwright.config.ts", "--project-test-dir", bad}, strings.NewReader(""), &out, &errb); code != 2 || !strings.Contains(errb.String(), "runs-invalid-arguments") {
				t.Errorf("%q: code=%d err=%q", bad, code, errb.String())
			}
		}
		var out, errb bytes.Buffer
		code := run([]string{"--root", repo, "runs", "identity", "--config", "playwright.config.ts", "--test-dir", "tests", "--project-test-dir", t.TempDir()}, strings.NewReader(""), &out, &errb)
		if code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "PROJECT_TEST_DIR_OUTSIDE_REPOSITORY") {
			t.Fatalf("outside: code=%d out=%q err=%q", code, out.String(), errb.String())
		}
	})
}

func TestSplitProjectTestDir(t *testing.T) {
	for in, want := range map[string][3]string{
		"setup":              {"setup", "", "true"},
		`setup:\.setup\.ts$`: {"setup", `\.setup\.ts$`, "true"},
		`C:\w\setup:(?:a|b)`: {`C:\w\setup`, "(?:a|b)", "true"},
		`C:/w/setup`:         {"C:/w/setup", "", "true"},
		"":                   {"", "", "false"},
		"setup:":             {"setup", "", "false"},
	} {
		dir, match, ok := splitProjectTestDir(in)
		if dir != want[0] || match != want[1] || (ok != (want[2] == "true")) {
			t.Errorf("%q = %q %q %v", in, dir, match, ok)
		}
	}
}

// TestRunsIdentityServedDigest is GitHub #1873 end to end: the served build is declared with
// --artifact-digest, and a candidate application revision that served the baseline's build compares
// UNDEPLOYED instead of PARITY.
func TestRunsIdentityServedDigest(t *testing.T) {
	tests, app := t.TempDir(), t.TempDir()
	for _, dir := range []*string{&tests, &app} {
		if resolved, err := filepath.EvalSymlinks(*dir); err == nil {
			*dir = resolved
		}
		reuseGit(t, *dir, "init", "-q")
	}
	reuseCommit(t, tests, map[string]string{"playwright.config.ts": "export default {}\n", "cart.spec.ts": "test('adds item', () => {})\n"})
	reuseCommit(t, app, map[string]string{"model.ts": "export const total = 1\n"})
	identity := func(t *testing.T, extra ...string) (json.RawMessage, string) {
		t.Helper()
		var out, errb bytes.Buffer
		args := append([]string{"--root", tests, "runs", "identity", "--config", "playwright.config.ts", "--app-root", app}, extra...)
		if code := run(args, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("identity: code=%d err=%q", code, errb.String())
		}
		var doc struct {
			Identity     json.RawMessage `json:"corvintRunIdentity"`
			ServedDigest string          `json:"servedDigest"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		return doc.Identity, doc.ServedDigest
	}
	t.Run("PWR-V0-054 without --artifact-digest the served digest is unknown", func(t *testing.T) {
		id, state := identity(t)
		if state != "UNKNOWN" || strings.Contains(string(id), "servedArtifactDigest") {
			t.Fatalf("state=%q identity=%s", state, id)
		}
	})
	t.Run("PWR-V0-054 invalid or repeated --artifact-digest refuses", func(t *testing.T) {
		for _, extra := range [][]string{{"--artifact-digest", ""}, {"--artifact-digest", "sha256 a"}, {"--artifact-digest", strings.Repeat("a", 257)},
			{"--artifact-digest", "sha256:a", "--artifact-digest", "sha256:b"}} {
			var out, errb bytes.Buffer
			args := append([]string{"--root", tests, "runs", "identity", "--config", "playwright.config.ts"}, extra...)
			if code := run(args, strings.NewReader(""), &out, &errb); code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "runs-invalid-arguments") {
				t.Errorf("%q: code=%d out=%q err=%q", extra, code, out.String(), errb.String())
			}
		}
	})
	baseID, state := identity(t, "--artifact-digest", "sha256:baseline-bundle")
	if state != "DECLARED" || !strings.Contains(string(baseID), `"servedArtifactDigest":"sha256:baseline-bundle"`) {
		t.Fatalf("state=%q identity=%s", state, baseID)
	}
	reuseCommit(t, app, map[string]string{"model.ts": "export const total = 2\n"})
	compare := func(t *testing.T, candidateDigest string) map[string]any {
		t.Helper()
		candID, _ := identity(t, "--artifact-digest", candidateDigest)
		dir := t.TempDir()
		writeRunsReports(t, dir, map[string]string{"base.json": runsReport("passed", `"corvintRunIdentity":`+string(baseID)),
			"cand.json": runsReport("passed", `"corvintRunIdentity":`+string(candID))})
		var out, errb bytes.Buffer
		if code := run([]string{"--root", dir, "runs", "compare", "--baseline", "base.json", "--candidate", "cand.json"}, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("compare: code=%d err=%q", code, errb.String())
		}
		got := map[string]any{}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Run("PWR-V0-056 a candidate revision serving the baseline build is UNDEPLOYED", func(t *testing.T) {
		got := compare(t, "sha256:baseline-bundle")
		if got["verdict"] != "UNDEPLOYED" || !reflect.DeepEqual(got["reasons"], []any{"SERVED_DIGEST_UNCHANGED"}) {
			t.Fatalf("compare = %v", got)
		}
	})
	t.Run("PWR-V0-056 a deployed candidate build is PARITY", func(t *testing.T) {
		if got := compare(t, "sha256:candidate-bundle"); got["verdict"] != "PARITY" || !reflect.DeepEqual(got["unknowns"], []any{}) {
			t.Fatalf("compare = %v", got)
		}
	})
}
