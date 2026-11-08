// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// selectFixture is a module whose guard test lists cmd/tool's import graph
// with a proven `go list` call; cmd/tool is built from lib.
var selectFixture = map[string]string{
	"go.mod":           "module example.test/g\n",
	"lib/lib.go":       "package lib\n\nfunc Help() string { return \"help\" }\n",
	"cmd/tool/main.go": "package main\n\nimport \"example.test/g/lib\"\n\nfunc main() { println(lib.Help()) }\n",
	"guard/guard.go":   "package guard\n",
	"guard/guard_test.go": "package guard\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestClosure(t *testing.T) {\n\tout, err := exec.Command(\"go\", \"list\", \"-deps\", \"-f\", \"{{.ImportPath}} {{join .Imports \\\" \\\"}}\", \"./cmd/tool\").Output()\n\t_, _ = out, err\n}\n",
}

const guardID = "go:example.test/g/guard"

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func readerSelected(plan affected.Plan) bool {
	for _, selection := range plan.Selected {
		if selection.UnitID == guardID && selection.Witness.Kind == affected.WitnessImportGraphReader {
			return true
		}
	}
	return false
}

// AFP-V0-9011 (V1-5030): library planning from a base agrees with
// `corvint affected --base`: SelectAt compares import sets at the base
// commit, so a body-only edit selects no import-graph reader and an import
// edit does. Without a pinned base (the fault injection) it falls back to the
// unknown comparison and selects the reader, as plain affected.Select and a
// `--path` plan do.
func TestSelectAtComparesImportSets_AFP_V0_9011(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	writeFixture(t, root, selectFixture)
	testsupport.RunGit(t, git, root, "init", "--quiet")
	testsupport.RunGit(t, git, root, "add", "-A")
	testsupport.RunGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "fixture")
	commit := strings.TrimSpace(testsupport.RunGit(t, git, root, "rev-parse", "HEAD"))
	ctx := context.Background()
	dirty := []string{"lib/lib.go"}

	writeFixture(t, root, map[string]string{"lib/lib.go": "package lib\n\n// Help names the helper.\nfunc Help() string { return \"help\" }\n"})
	graph, err := affected.Build(ctx, root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if readerSelected(SelectAt(ctx, root, root, commit, graph, dirty)) {
		t.Error("SelectAt selects the import-graph reader for a body-only edit")
	}
	if !readerSelected(SelectAt(ctx, root, root, "", graph, dirty)) || !readerSelected(affected.Select(graph, dirty)) {
		t.Error("without a base the plan must keep the reader (unknown comparison)")
	}

	writeFixture(t, root, map[string]string{"lib/lib.go": "package lib\n\nimport \"strings\"\n\nfunc Help() string { return strings.ToLower(\"help\") }\n"})
	graph, err = affected.Build(ctx, root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if !readerSelected(SelectAt(ctx, root, root, commit, graph, dirty)) {
		t.Error("SelectAt drops the import-graph reader for an import edit")
	}
}

// AFP-V0-9011: SelectBetween is SelectAt over two trees; a nil base is
// unknown and keeps the reader.
func TestSelectBetweenComparesImportSets_AFP_V0_9011(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, selectFixture)
	base := fstest.MapFS{}
	for name, body := range selectFixture {
		base[name] = &fstest.MapFile{Data: []byte(body)}
	}
	writeFixture(t, root, map[string]string{"lib/lib.go": "package lib\n\n// Help names the helper.\nfunc Help() string { return \"help\" }\n"})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"lib/lib.go"}
	if readerSelected(SelectBetween(base, os.DirFS(root), graph, dirty)) {
		t.Error("SelectBetween selects the import-graph reader for a body-only edit")
	}
	if !readerSelected(SelectBetween(nil, os.DirFS(root), graph, dirty)) {
		t.Error("SelectBetween without a base drops the reader")
	}
}
