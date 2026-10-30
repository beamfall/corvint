package typescript

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// GitHub #717 (V1-1082, DCP-V1-046): the listed executions keep Playwright's per-project test
// identity, repository-relative location and title path under the same refusals as discovery.
func TestPlaywrightListedTestsMultiProject(t *testing.T) {
	root, listing := multiProjectListFixture(t)
	tests, err := PlaywrightListedTests(root, "playwright.config.ts", listing, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 10 {
		t.Fatalf("listed %d executions, want 10: %+v", len(tests), tests)
	}
	want := PlaywrightListedTest{ID: "d318381359a9b96acb1e-e722184229c1526d3c2d", Path: "e2e/home.spec.ts", Line: 4, Column: 7, Project: "chromium", Title: "has title", Describe: []string{"home"}}
	index := slices.IndexFunc(tests, func(test PlaywrightListedTest) bool { return test.ID == want.ID })
	if index < 0 || tests[index].Path != want.Path || tests[index].Line != want.Line || tests[index].Column != want.Column || tests[index].Project != want.Project || tests[index].Title != want.Title || !slices.Equal(tests[index].Describe, want.Describe) {
		t.Fatalf("home execution: %+v", tests)
	}
	admin := slices.IndexFunc(tests, func(test PlaywrightListedTest) bool { return test.Project == "admin" })
	if admin < 0 || tests[admin].Path != "e2e/admin/users.spec.ts" || len(tests[admin].Describe) != 0 {
		t.Fatalf("admin execution: %+v", tests)
	}
	for i := 1; i < len(tests); i++ {
		if tests[i-1].Path > tests[i].Path {
			t.Fatalf("executions are not sorted by path: %+v", tests)
		}
	}
	sharded := bytes.Replace(listing, []byte(`"shard": null`), []byte(`"shard": {"current": 1, "total": 2}`), 1)
	if bytes.Equal(sharded, listing) {
		t.Fatal("fixture has no shard field to edit")
	}
	if _, err := PlaywrightListedTests(root, "playwright.config.ts", sharded, nil); err == nil || !strings.Contains(err.Error(), "sharded") {
		t.Fatalf("sharded listing: %v", err)
	}
	duplicate := bytes.Replace(listing, []byte("d318381359a9b96acb1e-577d9cd584a754eacf04"), []byte("d318381359a9b96acb1e-e722184229c1526d3c2d"), 1)
	if _, err := PlaywrightListedTests(root, "playwright.config.ts", duplicate, nil); err == nil || !strings.Contains(err.Error(), "repeats test id") {
		t.Fatalf("duplicate id: %v", err)
	}
}
