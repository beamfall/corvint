// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cliapp"
)

// mainRun is main's dispatch without the exit: the command-tree help answer first, as
// cliapp.AnswerStamped gives it from the installed tree snapshot, then runContext.
func mainRun(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	if code, handled := cliapp.AnswerStamped(binaryName, build, sourceCommit, args, &stdout, &stderr); handled {
		return code, stdout.String(), stderr.String()
	}
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// AFL-V1-008: a help request for a `docs pages` verb that does not exist is refused as
// invalid-arguments naming the verbs, never an internal error.
func TestAFLV1008UnknownPagesVerbHelpIsInvalidArguments_V15623(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"docs", "pages", "bogus", "--help"},
		{"docs", "pages", "bogus", "-h"},
		{"--root", t.TempDir(), "docs", "pages", "bogus", "--help"},
		{"help", "docs", "pages", "bogus"},
	} {
		code, out, diag := mainRun(args...)
		if code != 2 || out != "" || !strings.Contains(diag, `"code": "invalid-arguments"`) || strings.Contains(diag, "internal-error") {
			t.Errorf("%v: %d %q %s", args, code, out, diag)
		}
	}
}

// AFL-V1-008: `docs pages flow` takes no positional word, so `docs pages flow maintain ...` is
// refused rather than run as flow-maintain, whose --apply writes.
func TestAFLV1008PagesFlowRefusesAPositional_V15623(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	for _, args := range [][]string{
		{"docs", "pages", "flow", "maintain", "--flows", "flows", "--page", "p.md", "--claims", "c.json", "--apply"},
		{"docs", "pages", "flow", "maintain", "--flows", "flows", "--page", "p.md", "--claims", "c.json", "--preview"},
		{"docs", "pages", "flow", "extra"},
	} {
		code, out, diag := pagesRun(append([]string{"--root", root}, args...)...)
		if code != 2 || out != "" || !strings.Contains(diag, `"code": "invalid-arguments"`) || !strings.Contains(diag, "docs pages flow takes no positional argument: "+args[3]) {
			t.Errorf("%v: %d %q %s", args, code, out, diag)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "p.md")); err == nil {
		t.Fatal("a refused invocation wrote the page")
	}
}

// AFL-V1-007, AFL-V1-008: the moved reports' help says where they run, and the page commands'
// refusals name their new spelling.
func TestAFLV1007MovedReportHelpAndNewSpellingRefusals_V15623(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"status", "impact", "gaps"} {
		code, out, diag := mainRun("flows", verb, "--help")
		if code != 0 || !strings.Contains(out, "moved to corvint") || !strings.Contains(out, "run corvint flows "+verb) {
			t.Errorf("flows %s --help: %d %q %s", verb, code, out, diag)
		}
	}
	root := docsRepository(t)
	page := filepath.Join(root, "docs", "page.md")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("# Page\n\nHuman intro.\n\n<!-- corvint:docmaintain:insertion-point -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selector := []string{"--page", "docs/page.md", "--source", "owner.md", "--package", "cache"}
	maintain := func(args ...string) (int, string, string) {
		return pagesRun(append([]string{"--root", root, "docs", "pages", "maintain"}, args...)...)
	}
	if code, _, diag := maintain(append(selector, "--enable", "--apply")...); code != 0 {
		t.Fatalf("apply %d %s", code, diag)
	}
	docsCommit(t, root)
	owner, err := os.ReadFile(filepath.Join(root, "owner.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "owner.md"), append(owner, []byte("A new requirement line.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	docsCommit(t, root)
	code, _, diag := maintain(append(selector, "--check")...)
	if code != 1 || !strings.Contains(diag, "refresh with docs pages maintain --enable --apply") {
		t.Fatalf("stale check names the old spelling: %d %s", code, diag)
	}
	for _, args := range [][]string{{"--check"}, {"--page", "docs/page.md"}} {
		code, _, diag := maintain(args...)
		if code != 2 || !strings.Contains(diag, "docs pages maintain") || strings.Contains(strings.ReplaceAll(diag, "docs pages maintain", ""), "docs maintain") {
			t.Errorf("docs pages maintain %v: %d %s", args, code, diag)
		}
	}
}
