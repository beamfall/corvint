package affected_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryShellForeignCallersComplete_TSP_V0_005 verifies this
// repository's .corvint/shell-foreign-callers.json against the tree, so the
// declaration is checked rather than trusted. Every file in the source
// universe that names a declared script's file name (a path literal, the bare
// file name, a Make recipe) must be listed as a caller or a mention, and every
// listed file must still name it, whatever its suffix: only prose and data (.md, .json, .tsv) are
// exempt. The shell plugin must then accept the declaration and drop the
// foreign-invocation frontier from the script.
func TestRepositoryShellForeignCallersComplete_TSP_V0_005(t *testing.T) {
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ShellForeignCallersPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Scripts map[string]struct {
			Callers  []string `json:"callers"`
			Mentions []string `json:"mentions"`
		} `json:"scripts"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	if len(declared.Scripts) == 0 {
		t.Fatalf("no script is declared: %s", body)
	}

	source := affected.DiskSourceContext(context.Background(), root)
	files, err := source.Files(func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	// The declaration covers tracked files: an ignored build product in the
	// checkout, such as the ./corvint binary `go build ./cmd/corvint` writes,
	// embeds every script name and is no caller (V1-1819).
	listing, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]bool{}
	for _, name := range strings.Split(string(listing), "\x00") {
		tracked[name] = true
	}
	files = slices.DeleteFunc(files, func(name string) bool { return !tracked[name] })
	exempt := func(name string) bool {
		return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".tsv")
	}
	for script, entry := range declared.Scripts {
		var naming []string
		for _, name := range files {
			if name == script || exempt(name) {
				continue
			}
			text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if bytes.Contains(text, []byte(path.Base(script))) {
				naming = append(naming, name)
			}
		}
		slices.Sort(naming)
		listed := append(slices.Clone(entry.Callers), entry.Mentions...)
		for _, name := range naming {
			if !slices.Contains(listed, name) {
				t.Errorf("%s names %s but is not a declared caller or mention in %s", name, script, affected.ShellForeignCallersPath)
			}
		}
		for _, name := range listed {
			if !slices.Contains(naming, name) {
				t.Errorf("declared caller %s of %s no longer names it", name, script)
			}
		}
	}

	result, err := shell.New().UnitsSource(affected.DiskSourceContext(context.Background(), root))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(result.Frontier, shell.FrontierForeignCallersInvalid) {
		t.Fatalf("the shell plugin refused %s", affected.ShellForeignCallersPath)
	}
	for _, unit := range result.Units {
		if _, ok := declared.Scripts[strings.TrimPrefix(unit.ID, "shell:")]; ok {
			if !slices.Equal(unit.Callers, declared.Scripts[strings.TrimPrefix(unit.ID, "shell:")].Callers) || slices.Contains(unit.Frontier, shell.FrontierForeignInvocation) {
				t.Errorf("%s: callers %v, frontier %v; want declared callers and no foreign frontier", unit.ID, unit.Callers, unit.Frontier)
			}
		}
	}
}
