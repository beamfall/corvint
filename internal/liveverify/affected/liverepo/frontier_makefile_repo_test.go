// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127 builds the
// driver graph of this repository's root Makefile. Its WINDOWS_VET_GOCACHE
// is recursive, so the `$(shell stat ...)` probe runs only where windows-vet
// expands it, and windows-vet alone keeps that claim. The same file with
// that one assignment made immediate, as it was at 538e823f4, runs the probe
// while make reads the file, on a directory the environment supplies, so
// every target of the file is a computed site and claims every dirty path,
// including each merge's sealed CEM.
func TestFrontierSiteRootMakefileRunsNoReadTimeCommand_AFP_V0_3127(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repositoryRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	const recursive, immediate = "\nWINDOWS_VET_GOCACHE = $(if ", "\nWINDOWS_VET_GOCACHE := $(if "
	if strings.Count(string(body), recursive) != 1 {
		t.Fatalf("the root Makefile no longer assigns WINDOWS_VET_GOCACHE recursively once; an immediate `$(shell ...)` makes every root target a computed site (AFP-V0-3127)")
	}
	// AFP-V0-9051: script operands must exist as plain files before a
	// command can certify. Their contents are irrelevant to this driver-only
	// comparison; the fixture does not execute them.
	files := map[string]string{}
	for _, word := range strings.Fields(string(body)) {
		word = strings.TrimLeft(word, "@+-") // Make recipe prefixes are not part of the path.
		if strings.HasSuffix(word, ".sh") && affected.ValidRelativePath(word) {
			files[word] = "#!/bin/sh\n"
		}
	}
	computed := func(makefile string) (sites []string, targets int) {
		files["Makefile"] = makefile
		graph := makefileGraph(t, files)
		for _, id := range graph.UnitIDs() {
			if !strings.HasPrefix(id, "driver:make:Makefile#") {
				continue
			}
			targets++
			if computedSite(t, graph, id) {
				sites = append(sites, id)
			}
		}
		return sites, targets
	}
	asIs, targets := computed(string(body))
	reverted, revertedTargets := computed(strings.Replace(string(body), recursive, immediate, 1))
	t.Logf("root Makefile computed sites: %d of %d recursive, %d of %d immediate", len(asIs), targets, len(reverted), revertedTargets)
	if len(reverted) != revertedTargets {
		t.Errorf("with an immediate WINDOWS_VET_GOCACHE %d of %d root targets are computed sites; want every one", len(reverted), revertedTargets)
	}
	if !slices.Contains(asIs, "driver:make:Makefile#windows-vet") {
		t.Errorf("windows-vet expands the stat probe on an environment directory, but it is not a computed site")
	}
	if dropped := revertedTargets - len(asIs); dropped < targets/4 {
		t.Errorf("a recursive WINDOWS_VET_GOCACHE drops only %d of %d root computed sites; want at least a quarter", dropped, targets)
	}
}

// makefileGraph builds a repository holding one Go package and the files
// given, with the driver adapter attached.
func makefileGraph(t *testing.T, head map[string]string) *affected.Graph {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27.0\n",
		"core/core.go":      "package core\n\nfunc Value() int { return 1 }\n",
		"core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
	}
	for name, body := range head {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func computedSite(t *testing.T, graph *affected.Graph, id string) bool {
	t.Helper()
	unit, found := graph.Unit(id)
	if !found {
		t.Fatalf("%s is not a unit", id)
	}
	return slices.Contains(unit.Frontier, affected.FrontierDriverComputedInvocation)
}
