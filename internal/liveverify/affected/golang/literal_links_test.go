// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-9450: literal directory/file aliases retain targets and intermediate
// links, including missing targets. Unknown chains retain whole-repository reads.
func TestLiteralSymlinkReaders_AFPV09450(t *testing.T) {
	for _, tc := range []struct {
		name, literal string
		links         map[string]string
		dirty         []string
		broad         bool
	}{
		{"directory", "alias/config.go", map[string]string{"reader/alias": "../shared"}, []string{"shared/config.go", "reader/alias"}, false},
		{"directory listing", "alias", map[string]string{"reader/alias": "../shared"}, []string{"shared/config.go", "shared/other.go"}, true},
		{"root listing", "alias", map[string]string{"reader/alias": ".."}, []string{"shared/config.go", "shared/other.go"}, true},
		{"file different basename", "alias/input.txt", map[string]string{"reader/alias": "../shared", "shared/input.txt": "config.go"}, []string{"shared/config.go", "shared/input.txt"}, false},
		{"chain", "alias/config.go", map[string]string{"reader/alias": "../bridge/link", "bridge/link": "../shared"}, []string{"shared/config.go", "bridge/link"}, false},
		{"climb after alias", "alias/../config.go", map[string]string{"reader/alias": "../shared/nested"}, []string{"shared/config.go"}, true},
		{"missing leaf", "alias/new.go", map[string]string{"reader/alias": "../shared"}, []string{"shared/new.go"}, false},
		{"cycle", "alias/config.go", map[string]string{"reader/alias": "alias"}, []string{"shared/config.go", "shared/other.go"}, true},
		{"outside root", "alias/config.go", map[string]string{"reader/alias": "../../outside"}, []string{"shared/config.go", "shared/other.go"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			read := "ReadFile"
			if tc.name == "directory listing" || tc.name == "root listing" {
				read = "ReadDir"
			}
			writeFiles(t, root, map[string]string{
				"go.mod":             "module example.test/m\n",
				"shared/config.go":   "package shared\n",
				"shared/other.go":    "package shared\n",
				"shared/x_test.go":   "package shared\n",
				"shared/nested/keep": "fixture",
				"bridge/keep":        "fixture",
				"reader/r.go":        "package reader\nimport \"os\"\nfunc Read() { _, _ = os." + read + "(" + strconv.Quote(tc.literal) + ") }\n",
				"reader/x_test.go":   "package reader\n",
			})
			for name, target := range tc.links {
				if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			selects := func(dirty string) bool {
				for _, selection := range affected.Select(graph, []string{dirty}).Selected {
					if selection.UnitID == "go:example.test/m/reader" {
						return true
					}
				}
				return false
			}
			for _, dirty := range tc.dirty {
				if !selects(dirty) {
					t.Errorf("lost reader on %s", dirty)
				}
			}
			if !tc.broad && selects("shared/other.go") {
				t.Fatal("unrelated sibling selected reader")
			}
		})
	}
}
