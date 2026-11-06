//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			want := "1 unplanned [example.test/m/.github/extra], 0 unlisted []"
			if mode == "unlisted" {
				want = "0 unplanned [], 1 unlisted [example.test/m/.github/missing]"
			}
			if err == nil || !strings.Contains(err.Error(), "go list ./... differs from the planned Go universe: "+want) {
				t.Fatalf("discovery error %v; want %s", err, want)
			}
		})
	}
}
