// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	flowscmd "github.com/Beamfall/corvint/internal/command/flows"
)

// pagesRun runs one invocation and returns its exit, stdout and stderr.
func pagesRun(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// AFL-V1-008 scenario "Old name still works with a notice": each old spelling runs the command
// its new spelling runs, with the same exit and stdout, and its stderr is one notice naming the
// new spelling followed by the new spelling's own stderr.
func TestAFLV1008OldNamesAreAliasesWithANotice_V15623(t *testing.T) {
	t.Parallel()
	root := docsRepository(t)
	page := filepath.Join(root, "docs", "page.md")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("# Page\n\nHuman intro.\n\n<!-- corvint:docmaintain:insertion-point -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	maintain := []string{"--page", "docs/page.md", "--source", "owner.md", "--package", "cache"}
	for _, test := range []struct {
		old, renamed []string
		args         []string
		notice       string
	}{
		{[]string{"docs", "maintain"}, []string{"docs", "pages", "maintain"}, append([]string{"--enable", "--preview"}, maintain...), "docs maintain is renamed docs pages maintain"},
		{[]string{"docs", "maintain"}, []string{"docs", "pages", "maintain"}, []string{"--preview"}, "docs maintain is renamed docs pages maintain"},
		{[]string{"docs", "flows", "generate"}, []string{"docs", "pages", "generate"}, []string{"--revision", "HEAD"}, "docs flows generate is renamed docs pages generate"},
		{[]string{"docs", "flows", "check"}, []string{"docs", "pages", "check"}, []string{"--revision", strings.Repeat("a", 40), "--previous", "absent.json"}, "docs flows check is renamed docs pages check"},
		{[]string{"docs", "flows", "finalize"}, []string{"docs", "pages", "finalize"}, []string{"--previous", "absent.json"}, "docs flows finalize is renamed docs pages finalize"},
		{[]string{"flows", "docs"}, []string{"docs", "pages", "flow"}, []string{"--check", "--flows", "flows", "--page", "p.md", "--claims", "c.json"}, "flows docs is renamed docs pages flow"},
		{[]string{"flows", "docs", "maintain"}, []string{"docs", "pages", "flow-maintain"}, []string{"--flows", "flows", "--page", "p.md", "--claims", "c.json", "--preview"}, "flows docs maintain is renamed docs pages flow-maintain"},
	} {
		oldCode, oldOut, oldErr := pagesRun(append(append([]string{"--root", root}, test.old...), test.args...)...)
		newCode, newOut, newErr := pagesRun(append(append([]string{"--root", root}, test.renamed...), test.args...)...)
		want := "corvint-docs: " + test.notice + "; " + strings.Join(test.old, " ") + " is an alias until the next release (AFL-V1-008)\n"
		if oldCode != newCode || oldOut != newOut || oldErr != want+newErr {
			t.Errorf("%v vs %v:\nold %d %q %q\nnew %d %q %q", test.old, test.renamed, oldCode, oldOut, oldErr, newCode, newOut, newErr)
		}
		if strings.Contains(newErr, "AFL-V1-008") {
			t.Errorf("%v printed the alias notice: %q", test.renamed, newErr)
		}
	}
	// The preview really ran under both spellings.
	if code, out, _ := pagesRun(append([]string{"--root", root, "docs", "pages", "maintain", "--enable", "--preview"}, maintain...)...); code != 0 || out == "" {
		t.Fatalf("docs pages maintain --preview: %d %q", code, out)
	}
	for _, args := range [][]string{{"docs", "pages"}, {"docs", "pages", "bogus"}, {"docs", "pages", "--zz"}} {
		code, out, diag := pagesRun(append([]string{"--root", root}, args...)...)
		if code != 2 || out != "" || !strings.Contains(diag, "docs pages requires one of generate, check, finalize, flow, flow-maintain, maintain") {
			t.Errorf("%v: %d %q %s", args, code, out, diag)
		}
	}
}

// AFL-V1-008 scenario "Help lists one group": root help names the docs pages group and marks the
// old spellings as aliases; the group topic lists all six commands and the alias of each, and
// each command's help, under either spelling, is its topic with no notice.
func TestAFLV1008HelpListsOneGroup_V15623(t *testing.T) {
	t.Parallel()
	code, root, _ := pagesRun("--help")
	if code != 0 || !strings.Contains(root, "docs pages [VERB]") || !strings.Contains(root, "docs flows, flows docs [maintain] and docs maintain are\nits old spellings, aliases until the next release (AFL-V1-008)") {
		t.Fatalf("root help: %s", root)
	}
	code, group, _ := pagesRun("help", "docs", "pages")
	if code != 0 || group != docsPagesHelp+flowscmd.PageHelp {
		t.Fatalf("help docs pages: %d\n%s", code, group)
	}
	for _, verb := range pageVerbs() {
		if !strings.Contains(group, "corvint-docs [--root PATH] docs pages "+verb+" ") {
			t.Errorf("docs pages help lacks %s", verb)
		}
	}
	for _, alias := range []string{"docs flows generate|check|finalize  is  docs pages generate|check|finalize", "flows docs                          is  docs pages flow",
		"flows docs maintain                 is  docs pages flow-maintain", "docs maintain                       is  docs pages maintain"} {
		if !strings.Contains(group, alias) {
			t.Errorf("docs pages help does not mark the alias %q", alias)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"docs", "pages", "--help"}, docsPagesHelp + flowscmd.PageHelp},
		{[]string{"docs", "pages", "maintain", "--help"}, docsMaintainHelp},
		{[]string{"docs", "maintain", "--help"}, docsMaintainHelp},
		{[]string{"docs", "pages", "check", "--help"}, docsFlowsHelp},
		{[]string{"docs", "flows", "check", "--help"}, docsFlowsHelp},
		{[]string{"docs", "pages", "flow", "--help"}, docsPagesHelp + flowscmd.PageHelp},
		{[]string{"help", "docs", "pages", "flow-maintain"}, docsPagesHelp + flowscmd.PageHelp},
		{[]string{"help", "docs", "pages", "generate"}, docsFlowsHelp},
	} {
		if code, out, diag := pagesRun(test.args...); code != 0 || out != test.want || diag != "" {
			t.Errorf("%v: %d %q stdout matches=%v", test.args, code, diag, out == test.want)
		}
	}
	// The command tree holds the group, its six commands and the old spellings.
	tree := commandTree()
	for _, path := range [][]string{{"docs", "pages"}, {"docs", "pages", "generate"}, {"docs", "pages", "check"}, {"docs", "pages", "finalize"},
		{"docs", "pages", "flow"}, {"docs", "pages", "flow-maintain"}, {"docs", "pages", "maintain"}, {"docs", "maintain"}, {"flows", "docs", "maintain"}} {
		if node, ok := tree.Find(path); !ok || !slices.Equal(node.Path, path) {
			t.Errorf("command tree lacks %q", path)
		}
	}
}
