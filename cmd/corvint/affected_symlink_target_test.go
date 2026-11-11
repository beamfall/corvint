// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// AFP-V0-9965: `corvint affected` passes the planner the tree's links, so an
// edit below a tracked link's target, which `cat link/a.sh` reads through the
// link, makes the plan UNKNOWN with and without --base; an edit no link
// reaches does not.
func TestAffectedSymlinkTargetCLI_AFPV09965(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege Windows test hosts do not grant")
	}
	root := t.TempDir()
	for relative, body := range map[string]string{
		"real/a.sh":      "true\n",
		"other/b.sh":     "true\n",
		"reader_test.sh": "cat link/a.sh\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint-test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	frontier := func(edit string, options ...string) (bool, string) {
		t.Helper()
		affectedGit(t, root, "checkout", "-q", "--", ".")
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(edit)), []byte("false\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		receipt, _, stderr, code := runAffectedCLI(t, root, options...)
		if code != 0 {
			t.Fatalf("affected %v: exit %d: %s", options, code, stderr)
		}
		plan, _ := receipt["plan"].(map[string]any)
		unknown, _ := plan["unknown"].([]any)
		for _, raw := range unknown {
			entry, _ := raw.(map[string]any)
			if entry["reason"] == "LANGUAGE_FRONTIER" && entry["detail"] == "shell:symlink-target:"+edit {
				return true, plan["scope"].(string)
			}
		}
		scope, _ := plan["scope"].(string)
		return false, scope
	}
	for _, options := range [][]string{{"--base", base}, nil} {
		if found, scope := frontier("real/a.sh", options...); !found || scope != "UNKNOWN" {
			t.Errorf("edit below the link's target %v: frontier %v scope %s", options, found, scope)
		}
		if found, _ := frontier("other/b.sh", options...); found {
			t.Errorf("edit no link reaches %v: frontier raised", options)
		}
	}
}
