// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TSP-V0-7315: both executable builders honor the admitted shared build cache
// even when the test run's TMPDIR starts empty.
func TestExecutableBuildHelpersHonorSharedGoCache(t *testing.T) {
	fake := t.TempDir()
	record := filepath.Join(t.TempDir(), "cache")
	script := "#!/bin/sh\nprintf '%s' \"$GOCACHE\" > \"$PK_CACHE_RECORD\"\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then shift; : > \"$1\"; fi; shift; done\n"
	if err := execfile.WriteFile(filepath.Join(fake, "go"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PK_CACHE_RECORD", record)
	for _, tc := range []struct{ corvint, goCache string }{{"/shared/corvint", "/shared/go"}, {"", "/shared/go"}} {
		t.Setenv("CORVINT_GOCACHE", tc.corvint)
		t.Setenv("GOCACHE", tc.goCache)
		want := tc.corvint
		if want == "" {
			want = tc.goCache
		}
		for _, builder := range []string{"work", "transport"} {
			output := filepath.Join(t.TempDir(), "out")
			if builder == "work" {
				if err := workBuildExecutable(output); err != nil {
					t.Fatal(err)
				}
			} else {
				transportBuild(t, output)
			}
			got, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != want {
				t.Fatalf("%s cache=%q want=%q", builder, got, want)
			}
		}
	}
}

func executableTestGoCache() string {
	if cache := os.Getenv("CORVINT_GOCACHE"); cache != "" {
		return cache
	}
	if cache := os.Getenv("GOCACHE"); cache != "" {
		return cache
	}
	return filepath.Join(os.TempDir(), "corvint-go-build-cache")
}
