//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AZU-V0-9920: use the real Go discovery probe, including its dot-directory
// exclusion, rather than a fake listing that already agrees with the graph.
func TestHiddenHelperUniverse_AZU_V0_9920(t *testing.T) {
	for _, mode := range []string{"exact", "root", "unplanned", "unlisted"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                   "module example.test/m\n\ngo 1.27.3\n",
				"ordinary/ordinary.go":     "package ordinary\n",
				".github/helper/helper.go": "package helper\n",
			}
			if mode == "unplanned" {
				files[".github/extra/extra.go"] = "package extra\n"
			}
			if mode == "root" {
				files[".github/root.go"] = "package helperroot\n"
			}
			for name, body := range files {
				full := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			runner := Runner{
				Root: root, Environ: append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local"),
				Universe: []string{"example.test/m/ordinary", "example.test/m/.github/helper"},
			}
			if mode == "unlisted" {
				runner.Universe = append(runner.Universe, "example.test/m/.github/missing")
			}
			if mode == "root" {
				runner.Universe = append(runner.Universe, "example.test/m/.github")
			}
			_, _, err := runner.discover(context.Background(), Planned{Adapter: AdapterGo, Argv: []string{"go"}}, time.Now().Add(time.Minute))
			if mode == "exact" || mode == "root" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			want := "go list ./... differs from the planned Go universe: 1 unplanned [example.test/m/.github/extra], 0 unlisted []"
			if mode == "unlisted" {
				// The planned package is also listed explicitly (AZU-V0-005),
				// so go list itself refuses the absent package.
				want = "go list ./... failed; the Go universe cannot be confirmed"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("discovery error %v; want %s", err, want)
			}
		})
	}
}

// AZU-V0-9920 / STR-V0-069: an independent helper module and testdata are
// absent from the graph and must not poison root discovery. Nested hidden and
// build directories are indexed source (AZU-V0-005) and must be confirmed.
func TestHiddenHelperModuleBoundary_AZU_V0_9920(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                      "module example.test/m\n\ngo 1.27.3\n",
		"ordinary/ordinary.go":        "package ordinary\n",
		"ordinary/ordinary_test.go":   "package ordinary\nimport \"testing\"\nfunc TestOrdinary(t *testing.T) {}\n",
		".github/helper/helper.go":    "package helper\n",
		".github/nested/go.mod":       "module example.test/nested\n\ngo 1.27.3\n",
		".github/nested/nested.go":    "package nested\n",
		".github/build/build.go":      "package build\n",
		".github/testdata/fixture.go": "package fixture\n",
		".github/.hidden/hidden.go":   "package hidden\n",
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	var universe []string
	for _, id := range graph.UnitIDs() {
		if pkg, ok := strings.CutPrefix(id, "go:"); ok {
			universe = append(universe, pkg)
		}
	}
	if !slices.Equal(universe, []string{"example.test/m/.github/.hidden", "example.test/m/.github/build", "example.test/m/.github/helper", "example.test/m/ordinary"}) {
		t.Fatalf("graph crossed a boundary: %v", universe)
	}
	runner := Runner{Root: root, Environ: append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local"), Universe: universe}
	if _, _, err := runner.discover(context.Background(), Planned{Adapter: AdapterGo, Argv: []string{"go"}}, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("unrelated ordinary package discovery failed: %v", err)
	}
}
