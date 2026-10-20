package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	staticPlaywrightSpec = `import { test, expect } from '@playwright/test';

test.describe('login', () => {
  test('works', async ({ page }) => {
    await expect(page).toHaveTitle('x');
  });
  test.skip('skipped', async ({ page }) => {
    await expect(page).toHaveTitle('x');
  });
  test.only('focused', async ({ page }) => {
    await expect(page).toHaveTitle('x');
  });
  test.fixme('broken', async ({ page }) => {
    await expect(page).toHaveTitle('x');
  });
  test('no assertion', async ({ page }) => {
    await page.goto('/');
  });
});
`
	staticRSpec = `RSpec.describe User do
  context "when admin" do
    it "is admin" do
      expect(1).to eq(1)
    end
    xit "skipped" do
      expect(1).to eq(1)
    end
    fit "focused" do
      expect(1).to eq(1)
    end
    it "no expectation" do
      User.new
    end
  end
end
`
)

// staticSnapshot is the repository's git status and every file's path, size, mode and mtime.
func staticSnapshot(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "%s %d %v %d\n", path, info.Size(), info.Mode(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// After the walk, and without optional locks, so the snapshot never refreshes the index itself.
	out.WriteString(affectedGit(t, root, "--no-optional-locks", "status", "--porcelain", "--untracked-files=all", "--ignored"))
	return out.String()
}

// LPCV-V0-150..153: `corvint test-validity --static` projects committed Playwright and RSpec source
// with no retained run: skipped, fixme and focused tests are INELIGIBLE, an assertion-free test is
// named in sourceRules, execution and strength stay NOT_OBSERVED, every test is located, and the
// repository (status and every file's metadata) is unchanged.
func TestLPCVV0150StaticModeProjectsSourceAndWritesNothing(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"e2e/login.spec.ts": staticPlaywrightSpec, "spec/user_spec.rb": staticRSpec} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "add", "-A")
	affectedGit(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	affectedGit(t, root, "update-index", "--really-refresh") // settle racily-clean index entries first
	before := staticSnapshot(t, root)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "test-validity", "--static", "e2e", "spec"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if after := staticSnapshot(t, root); after != before {
		t.Fatalf("static mode changed the repository:\n%s\n->\n%s", before, after)
	}
	var document struct {
		Source, Kind string
		Static       struct{ Files []string }
		Tests        []struct {
			ID         string
			Projection map[string]struct{ State, Reason string }
			Location   struct {
				File      string
				Line      int
				TitlePath []string
			}
			SourceRules struct {
				Expectations int
				Findings     []struct{ Rule string }
			}
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Source != "corvint-static-test-source" || document.Kind != "static" || len(document.Tests) != 9 || strings.Join(document.Static.Files, ",") != "e2e/login.spec.ts,spec/user_spec.rb" {
		t.Fatalf("document=%s", stdout.String())
	}
	want := map[string]string{
		"e2e/login.spec.ts:4": "ELIGIBLE", "e2e/login.spec.ts:7": "INELIGIBLE JS-SKIPPED", "e2e/login.spec.ts:10": "INELIGIBLE JS-FOCUSED",
		"e2e/login.spec.ts:13": "INELIGIBLE JS-SKIPPED", "e2e/login.spec.ts:16": "ELIGIBLE", "spec/user_spec.rb:3": "ELIGIBLE",
		"spec/user_spec.rb:6": "INELIGIBLE RSPEC-SKIPPED", "spec/user_spec.rb:9": "INELIGIBLE RSPEC-FOCUSED", "spec/user_spec.rb:12": "ELIGIBLE",
	}
	for _, test := range document.Tests {
		hygiene := test.Projection["hygiene"]
		got := hygiene.State
		if hygiene.State == "INELIGIBLE" {
			got += " " + hygiene.Reason
		}
		if want[test.ID] != got || test.Projection["execution"].State != "NOT_OBSERVED" || test.Projection["strength"].State != "NOT_OBSERVED" ||
			test.Location.File+":"+fmt.Sprint(test.Location.Line) != test.ID || len(test.Location.TitlePath) < 2 {
			t.Fatalf("test %s: hygiene %q want %q, %+v", test.ID, got, want[test.ID], test)
		}
	}
	if free := document.Tests[4].SourceRules; free.Expectations != 0 || len(free.Findings) != 1 || free.Findings[0].Rule != "JS-NO-ASSERTION" {
		t.Fatalf("assertion-free js=%+v", free)
	}
	if free := document.Tests[8].SourceRules; free.Expectations != 0 || strings.Join(document.Tests[8].Location.TitlePath, "/") != "User/when admin/no expectation" {
		t.Fatalf("assertion-free rspec=%+v", document.Tests[8])
	}

	code = run([]string{"--root", root, "test-validity", "--static", "lib"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "tests-source-unavailable") {
		t.Fatalf("missing path exit=%d stderr=%s", code, stderr.String())
	}
}

// LPCV-V0-153: --static excludes --receipt, --discover and --route-stubs, and a PATH operand needs
// --static; each misuse exits 2 with no stdout.
func TestLPCVV0153StaticOptionsAreExclusive(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"--static", "--discover"},
		{"--static", "--receipt", "x.json"},
		{"--static", "--route-stubs"},
		{"--static", "--static"},
		{"--static=e2e"},
		{"e2e"},
		{"--discover", "e2e"},
	} {
		code, stdout, stderr := runTestValidityCLI(t, arguments...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
		}
	}
}
