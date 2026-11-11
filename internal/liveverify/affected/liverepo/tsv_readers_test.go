// SPDX-License-Identifier: Apache-2.0
package liverepo_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// AFP-V0-3330, AFP-V0-3331: an audited local read declaration replaces the
// implicit sibling-directory scope; an unaudited enumerator stays selected.
func TestShellDataReaders_AFPV03330(t *testing.T) {
	root := affprecFiles(t, map[string]string{
		"script/check-ci-least-privilege_test.sh": "cat script/exceptions.tsv\n",
		"script/build_test.sh":                    "go build ./tool\ncat script/helper.sh\n",
		"script/enumerate_test.sh":                "go build ./tool\nfind script/ -type f\n",
		"script/helper.sh":                        "echo helper\n",
		"script/exceptions.tsv":                   "# data\n",
		"tool/main.go":                            "package main\n",
		affected.ShellReadScopesPath: `{"profile":"corvint-shell-read-scopes/0","tests":{
   "script/build_test.sh":{"reads":["tool/"],"localReads":["script/helper.sh"],"reason":"only helper is read locally"},
   "script/enumerate_test.sh":{"reads":["tool/"],"reason":"local enumeration is not narrowed"}
  }}`,
	})
	g := affprecGraph(t, root, shell.New())
	ids := affprecIDs(affected.Select(g, []string{"script/exceptions.tsv"}))
	for _, id := range []string{"shell:script/check-ci-least-privilege_test.sh", "shell:script/enumerate_test.sh"} {
		if !slices.Contains(ids, id) {
			t.Errorf("missing real or unbounded reader %s: %v", id, ids)
		}
	}
	if slices.Contains(ids, "shell:script/build_test.sh") {
		t.Errorf("implicit directory selected bounded reader: %v", ids)
	}
	for _, dirty := range []string{"script/helper.sh", "script/build_test.sh", "tool/main.go"} {
		if ids := affprecIDs(affected.Select(g, []string{dirty})); !slices.Contains(ids, "shell:script/build_test.sh") {
			t.Errorf("%s lost reader: %v", dirty, ids)
		}
	}
	if p := affected.Select(g, []string{affected.ShellReadScopesPath}); p.Scope != affected.ScopeUnknown {
		t.Fatalf("changed declaration lost uncertainty: %+v", p)
	}
}

// AFP-V0-3332: only bare utilities get output-operand filtering. Qualified
// commands, recursive source copies and ambiguous -t copies stay conservative.
func TestShellCopyDestinationReaders_AFPV03332(t *testing.T) {
	root := affprecFiles(t, map[string]string{
		"script/bare_test.sh":   "cp -f helper.sh \"$DEST/script/\"\n",
		"script/output_test.sh": "/bin/cp -f helper.sh \"$DEST/script/\"\n",
		"script/input_test.sh":  "/bin/cp -R \"$ROOT/script/\" dest/\n",
		"script/target_test.sh": "/bin/cp -t dest/ \"$ROOT/script/\"\n",
	})
	ids := affprecIDs(affected.Select(affprecGraph(t, root, shell.New()), []string{"script/exceptions.tsv"}))
	if slices.Contains(ids, "shell:script/bare_test.sh") {
		t.Errorf("copy destination selected: %v", ids)
	}
	for _, id := range []string{"shell:script/output_test.sh", "shell:script/input_test.sh", "shell:script/target_test.sh"} {
		if !slices.Contains(ids, id) {
			t.Errorf("lost copy source %s: %v", id, ids)
		}
	}
}

// AFP-V0-3330: exercise the real declarations and scripts, retaining every
// audited local file and excluding unrelated sibling data. The cause audit is
// recorded in docs/build-log/2026-11-08-tsv-readers.md.
func TestRepositoryShellDataReaders_AFPV03330(t *testing.T) {
	g := affprecGraph(t, repositoryRoot(t), languages.All()...)
	var declaration struct {
		Tests map[string]struct {
			LocalReads *[]string `json:"localReads"`
		} `json:"tests"`
	}
	if err := json.Unmarshal([]byte(affprecRead(t, affected.ShellReadScopesPath)), &declaration); err != nil {
		t.Fatal(err)
	}
	ids := affprecIDs(affected.Select(g, []string{"script/ci-least-privilege-exceptions.tsv"}))
	if !slices.Contains(ids, "shell:script/check-ci-least-privilege_test.sh") {
		t.Fatalf("lost actual TSV reader: %v", ids)
	}
	count := 0
	for test, entry := range declaration.Tests {
		if entry.LocalReads == nil {
			continue
		}
		count++
		if slices.Contains(ids, "shell:"+test) {
			t.Errorf("%s selected only through sibling data", test)
		}
		for _, read := range append(slices.Clone(*entry.LocalReads), test) {
			if strings.HasSuffix(read, "/") {
				continue
			}
			if selected := affprecIDs(affected.Select(g, []string{read})); !slices.Contains(selected, "shell:"+test) {
				t.Errorf("%s lost audited reader %s", read, test)
			}
		}
	}
	if count != 11 {
		t.Fatalf("audited %d narrowed entries, want 11", count)
	}
}

// AFP-V0-3333: wildcard matches may feed derived reads, so even complete
// globs keep the conservative directory relation. Non-path case patterns do not.
func TestShellGlobDataReaders_AFPV03333(t *testing.T) {
	root := affprecFiles(t, map[string]string{
		"script/glob_test.sh":     "for f in \"$ROOT\"/script/*_test.sh; do cat \"$f\"; done\n",
		"script/all_test.sh":      "for f in \"$ROOT\"/script/*; do cat \"$f\"; done\n",
		"script/dynamic_test.sh":  "cat \"$ROOT/script/$NAME\"\n",
		"script/globstar_test.sh": "cat \"$ROOT\"/script/**/*.sh\n",
		"script/case_test.sh":     `case "$name" in */*) echo path;; esac` + "\n",
		"script/climb_test.sh":    "cat ../script/*.sh\n",
	})
	g := affprecGraph(t, root, shell.New())
	ids := affprecIDs(affected.Select(g, []string{"script/exceptions.tsv"}))
	if slices.Contains(ids, "shell:script/case_test.sh") {
		t.Errorf("non-path case pattern selected unrelated data: %v", ids)
	}
	for _, name := range []string{"glob", "all", "dynamic", "globstar", "climb"} {
		if !slices.Contains(ids, "shell:script/"+name+"_test.sh") {
			t.Errorf("lost conservative %s reader: %v", name, ids)
		}
	}
	if ids := affprecIDs(affected.Select(g, []string{"script/new_test.sh"})); !slices.Contains(ids, "shell:script/glob_test.sh") {
		t.Errorf("lost matching glob: %v", ids)
	}
}
