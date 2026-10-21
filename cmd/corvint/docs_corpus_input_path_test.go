package main

import (
	json "encoding/json/v2"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsCorpusLocalInputPathRefusals checks that each refused local input path names its flag and
// the broken rule under the stable corpus-refused code and error shape (DCP-V1-054, V1-1226).
func TestDocsCorpusLocalInputPathRefusals(t *testing.T) {
	root, _ := playwrightCorpusRepository(t)
	discover := func(flag, value string) []string {
		args := map[string]string{"--migration": "evidence/migration.json", "--playwright-list": "evidence/list.json"}
		args[flag] = value
		out := []string{"docs", "corpus", "discover-playwright", "--config", "playwright.config.ts", "--migration", args["--migration"], "--playwright-list", args["--playwright-list"]}
		if flag == "--receipt" {
			out = append(out, "--receipt", value)
		}
		return out
	}
	for _, tc := range []struct {
		name, flag, rule string
		args             []string
	}{
		{"absolute migration", "--migration", "got an absolute path", discover("--migration", filepath.Join(root, "evidence", "migration.json"))},
		{"absolute listing", "--playwright-list", "got an absolute path", discover("--playwright-list", filepath.Join(root, "evidence", "list.json"))},
		{"parent receipt", "--receipt", "got a parent traversal", discover("--receipt", "../receipt.json")},
		{"non-canonical migration", "--migration", "got a non-canonical path", discover("--migration", "evidence/./migration.json")},
		{"inner parent listing", "--playwright-list", "got a non-canonical path", discover("--playwright-list", "evidence/../evidence/list.json")},
		{"absolute artifact", "--artifact", "got an absolute path", []string{"docs", "corpus", "info", "--artifact=" + filepath.Join(root, "corpus.json")}},
		{"absolute manifest", "--manifest", "got an absolute path", []string{"docs", "corpus", "build", "--manifest", filepath.Join(root, "corpus-input.json")}},
		{"absolute adapter input", "--input", "got an absolute path", []string{"docs", "corpus", "behavior-adapter", "--input", filepath.Join(root, "request.json")}},
		{"parent adapter previous", "--previous", "got a parent traversal", []string{"docs", "corpus", "behavior-adapter", "--input", "request.json", "--previous", "../result.json"}},
		{"absolute cem", "--cem", "got an absolute path", []string{"docs", "corpus", "cem", "--artifact", "corpus.json", "--cem", filepath.Join(root, "change.cem.json")}},
		{"root retirement", "--retirement", "got the root itself", []string{"docs", "corpus", "info", "--artifact", "corpus.json", "--retirement", "."}},
		{"absolute witness report", "--report", "got an absolute path", []string{"docs", "corpus", "witness-playwright", "--input", "evidence/request.json", "--receipt-input", "receipt", "--report", filepath.Join(root, "evidence", "report.json")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, stderr := corpusCLI(t, root, tc.args...)
			if code != 2 || out != "" {
				t.Fatalf("%d %q %s", code, out, stderr)
			}
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
				OK    bool   `json:"ok"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &refusal, json.RejectUnknownMembers(true)); err != nil {
				t.Fatalf("refusal is not the closed error object: %v %s", err, stderr)
			}
			want := tc.flag + ": local input paths must be canonical and relative to --root (" + tc.rule + ")"
			if refusal.OK || refusal.Code != "corpus-refused" || refusal.Error != want {
				t.Fatalf("refusal = %v, want code corpus-refused and %q", refusal, want)
			}
		})
	}
}

func TestDocsCorpusHelpStatesRootRelativeInputs(t *testing.T) {
	t.Parallel()
	for _, required := range []string{"--migration", "--playwright-list", "--receipt", "relative to --root", "must sit under", "no leading /, no ..", "Absolute paths are refused"} {
		if !strings.Contains(docsHelp, required) {
			t.Errorf("docs corpus help omits %q", required)
		}
	}
}

// TestDCPV1241CorpusMaintainPageRefusalNamesFlag checks that a refused `docs corpus maintain
// --page` names the flag and the broken rule before any read, under the unchanged corpus-refused
// code and error shape (DCP-V1-241, V1-1227).
func TestDCPV1241CorpusMaintainPageRefusalNamesFlag(t *testing.T) {
	root, _ := playwrightCorpusRepository(t)
	for _, tc := range []struct{ name, page, rule string }{
		{"absolute", filepath.Join(root, "docs", "page.md"), "got an absolute path"},
		{"parent", "../page.md", "got a parent traversal"},
		{"non-canonical", "docs/./page.md", "got a non-canonical path"},
		{"not markdown", "docs/page.txt", "got a page that is not Markdown"},
		{"git metadata", "docs/.GIT/page.md", "got a page inside Git metadata"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, stderr := corpusCLI(t, root, "docs", "corpus", "maintain", "--artifact", "missing-corpus.json", "--page", tc.page)
			if code != 2 || out != "" {
				t.Fatalf("%d %q %s", code, out, stderr)
			}
			var refusal struct {
				Code  string `json:"code"`
				Error string `json:"error"`
				OK    bool   `json:"ok"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &refusal, json.RejectUnknownMembers(true)); err != nil {
				t.Fatalf("refusal is not the closed error object: %v %s", err, stderr)
			}
			want := "--page: maintenance requires a Markdown page relative to --root outside Git metadata (" + tc.rule + ")"
			if refusal.OK || refusal.Code != "corpus-refused" || refusal.Error != want {
				t.Fatalf("refusal = %v, want code corpus-refused and %q", refusal, want)
			}
		})
	}
}
