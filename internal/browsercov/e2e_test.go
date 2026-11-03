// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// The end-to-end tests run real browsers. CORVINT_BROWSERCOV_DEPS names a
// directory whose node_modules holds @playwright/test and cypress (with
// their browsers installed); CORVINT_BROWSERCOV_CHROMIUM optionally names
// the Chromium binary Cypress drives (default: Playwright's).
func e2eDeps(t *testing.T) string {
	deps := os.Getenv("CORVINT_BROWSERCOV_DEPS")
	if deps == "" {
		t.Skip("CORVINT_BROWSERCOV_DEPS is not set")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	return deps
}

// fixture copies testdata/webapp into a fresh Git repository whose
// node_modules links to the dependencies.
func fixture(t *testing.T, deps string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir("testdata/webapp", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/webapp", p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(root, rel), 0o755)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), body, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(deps, "node_modules"), filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func collectIndex(t *testing.T, o Options) *linecov.Index {
	t.Helper()
	o.Work = t.TempDir()
	o.Timeout = 3 * time.Minute
	o.Log = t.Logf
	res, runner, err := Collect(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	var recs []linecov.Record
	for _, r := range res {
		if r.Record == nil {
			t.Fatalf("no record for %s %s: %s", r.Package, r.Name, r.Reason)
		}
		recs = append(recs, *r.Record)
	}
	oids, err := linecov.OIDs(context.Background(), o.Root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("browser", "HEAD", runner, oids, recs)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("runner %s: %d tests, %d files", runner, len(ix.Tests), len(ix.Files))
	return ix
}

// testsAt names the tests that executed any block starting on line of path.
func testsAt(ix *linecov.Index, path string, line int32) string {
	seen := map[string]bool{}
	for _, f := range ix.Files {
		if f.Path != path {
			continue
		}
		for _, b := range f.Blocks {
			if b.Start <= line && line <= b.End {
				for _, id := range b.Tests {
					if n := ix.Tests[id].Name; n != linecov.InOrder {
						seen[n] = true
					}
				}
			}
		}
	}
	var out []string
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// checkFixture proves each test maps to the client lines and the server
// lines it executed, and to no line only the other test executed
// (LPCV-V0-970, LPCV-V0-972, LPCV-V0-974).
func checkFixture(t *testing.T, ix *linecov.Index, adds, signs string) {
	t.Helper()
	both := adds + "," + signs
	if adds > signs {
		both = signs + "," + adds
	}
	for _, c := range []struct {
		path string
		line int32
		want string
	}{
		{"public/app.js", 2, both},   // show: fetch
		{"public/app.js", 7, adds},   // onAdd
		{"public/app.js", 11, signs}, // onSign
		{"server/math.js", 2, adds},  // add
		{"server/math.js", 6, signs}, // sign: if
		{"server/math.js", 7, ""},    // sign: negative branch, never taken
		{"server/math.js", 9, signs}, // sign: positive
		{"server.js", 13, adds},      // /api/add handler
		{"server.js", 19, signs},     // /api/sign handler
		{"server.js", 25, both},      // static files
		{"server.js", 27, ""},        // 404, never taken
	} {
		if got := testsAt(ix, c.path, c.line); got != c.want {
			t.Errorf("%s:%d executed by %q, want %q", c.path, c.line, got, c.want)
		}
	}
}

func TestE2EPlaywright(t *testing.T) {
	deps := e2eDeps(t)
	root := fixture(t, deps)
	ix := collectIndex(t, Options{Root: root, Runner: "playwright"})
	checkFixture(t, ix, "[chromium] adds", "[chromium] signs")
}

func TestE2ECypress(t *testing.T) {
	deps := e2eDeps(t)
	browser := os.Getenv("CORVINT_BROWSERCOV_CHROMIUM")
	if browser == "" {
		home, _ := os.UserHomeDir()
		matches, _ := filepath.Glob(filepath.Join(home, ".cache/ms-playwright/chromium-*/chrome-linux*/chrome"))
		if len(matches) == 0 {
			t.Skip("no Chromium binary: set CORVINT_BROWSERCOV_CHROMIUM")
		}
		sort.Strings(matches)
		browser = matches[len(matches)-1]
	}
	root := fixture(t, deps)
	ix := collectIndex(t, Options{Root: root, Runner: "cypress", Browser: browser,
		Server: "node server.js", ServerURL: "http://[::1]:3123"})
	checkFixture(t, ix, "app > adds", "app > signs")
	if _, err := os.Stat(filepath.Join(root, cyWrapper)); !os.IsNotExist(err) {
		t.Errorf("generated %s was not removed: %v", cyWrapper, err)
	}
}
