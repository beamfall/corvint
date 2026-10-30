package playwrightruns

import (
	"encoding/json"
	"testing"
)

const fixtureRoot = "/work/e2e"

// fxTest is one test of a synthetic json-reporter document.
type fxTest struct {
	project, file, title string
	describe             []string
	status               string // expected | unexpected | flaky | skipped
	expectedFail         bool
	results              []fxResult
}

type fxResult struct {
	status  string
	retry   int
	worker  int
	message string
	file    string // error location; "" means none
	line    int
	start   string // startTime; "" means absent
}

func pass() fxResult { return fxResult{status: "passed"} }

func fail(message, file string, line int) fxResult {
	return fxResult{status: "failed", message: message, file: file, line: line}
}

// identity is a config.metadata.corvintRunIdentity declaration.
func identity(app, config string, blobs map[string]string) map[string]any {
	return map[string]any{"applicationRevision": app, "configDigest": config, "testFileBlobs": blobs}
}

// reportJSON builds a Playwright json-reporter document shaped like @playwright/test 1.63 output:
// one top-level suite per file, nested describe suites, specs with one test per project.
func reportJSON(t *testing.T, version string, declared map[string]any, tests ...fxTest) []byte {
	t.Helper()
	metadata := map[string]any{"actualWorkers": 2}
	if declared != nil {
		metadata["corvintRunIdentity"] = declared
	}
	files := map[string]map[string]any{}
	order := []string{}
	for _, ft := range tests {
		top := files[ft.file]
		if top == nil {
			top = map[string]any{"title": ft.file, "file": ft.file, "specs": []any{}, "suites": []any{}}
			files[ft.file] = top
			order = append(order, ft.file)
		}
		results := []any{}
		for _, r := range ft.results {
			res := map[string]any{"workerIndex": r.worker, "parallelIndex": 0, "status": r.status, "duration": 12.4, "retry": r.retry, "errors": []any{}}
			if r.start != "" {
				res["startTime"] = r.start
			}
			if r.message != "" {
				e := map[string]any{"message": r.message}
				if r.file != "" {
					e["location"] = map[string]any{"file": r.file, "line": r.line, "column": 5}
				}
				res["errors"] = []any{e}
				res["error"] = e
			}
			results = append(results, res)
		}
		expected := "passed"
		annotations := []any{}
		if ft.expectedFail {
			expected = "failed"
			annotations = append(annotations, map[string]any{"type": "fail", "description": "DEF-12"})
		}
		project := ft.project
		if project == "" {
			project = "chromium"
		}
		spec := map[string]any{"title": ft.title, "file": ft.file, "line": 3, "tests": []any{map[string]any{
			"projectName": project, "projectId": project, "expectedStatus": expected, "status": ft.status, "annotations": annotations, "results": results}}}
		parent := top
		for _, d := range ft.describe {
			child := map[string]any{"title": d, "file": ft.file, "specs": []any{}, "suites": []any{}}
			parent["suites"] = append(parent["suites"].([]any), child)
			parent = child
		}
		parent["specs"] = append(parent["specs"].([]any), spec)
	}
	suites := []any{}
	for _, f := range order {
		suites = append(suites, files[f])
	}
	doc := map[string]any{
		"config": map[string]any{"version": version, "rootDir": fixtureRoot, "workers": 2, "metadata": metadata,
			"projects": []any{map[string]any{"name": "chromium", "retries": 0}}},
		"suites": suites, "errors": []any{}, "stats": map[string]any{"expected": 1},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustParse(t *testing.T, raw []byte) *Run {
	t.Helper()
	run, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return run
}
