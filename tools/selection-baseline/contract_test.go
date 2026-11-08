// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// AFP-V0-9501: the actual replay caller must supply CI's required wrapper
// argument and stay a dry run. Argument refusals and unknown exits remain errors
// even when a manifest exists; only the existing exit-3 exception is allowed.
func TestSelectionContractDryRunArguments_AFPV09501(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake corvint is a POSIX shell script")
	}
	for _, profile := range []string{"local", "ci"} {
		for _, code := range []int{0, 2, 4} {
			t.Run(fmt.Sprintf("%s/exit%d", profile, code), func(t *testing.T) {
				t.Setenv("SELECTION_CONTRACT_CHECK", "")
				if profile == "ci" {
					t.Setenv("SELECTION_CONTRACT_CHECK", "1")
				}
				dir := t.TempDir()
				corvint := filepath.Join(dir, "corvint")
				manifestPath := filepath.Join(dir, "plan with spaces.json")
				script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > \"$6.args\"\nprintf '{}' > \"$6\"\nexit %d\n", code)
				if err := execfile.WriteFile(corvint, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
				err := dryRunManifest(context.Background(), dir, corvint, "base", manifestPath)
				if (err == nil) != (code == 0) {
					t.Fatalf("exit %d: error %v", code, err)
				}
				data, err := os.ReadFile(manifestPath + ".args")
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"test", "--base", "base", "--dry-run", "--manifest", manifestPath}
				if profile == "ci" {
					confine := filepath.Join(dir, "test-confine")
					if !filepath.IsAbs(confine) {
						t.Fatal("fixture wrapper path is not absolute")
					}
					want = append(want, "--profile", "ci", "--confine", confine)
				}
				if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !reflect.DeepEqual(got, want) {
					t.Fatalf("argv = %q, want %q", got, want)
				}
			})
		}
	}
}

// AFP-V0-9501: no undeclared difference can pass the replay. Slicing,
// ordering and non-root adapters are the explicit execution differences.
func TestSelectionContractReplay_AFPV09501(t *testing.T) {
	goUnit := func(pkg string) manifestUnit { return manifestUnit{Adapter: "go", Unit: "go:" + pkg} }
	for _, tc := range []struct {
		name, audit string
		manifest    manifest
		pass        bool
	}{
		{"same", "selected m/a\nrun m/a m/b", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/b"), goUnit("m/a")}}, true},
		{"profiles", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a"), {Adapter: "ruby", Unit: "ruby:spec/x"}, {Adapter: "go", Unit: "go-module:nested"}}}, true},
		{"non-Go only", "NOTHING no root work", manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "ruby", Unit: "ruby:spec/x"}}}, true},
		{"full", "FALLBACK unknown frontier", manifest{Scope: "FULL", Units: []manifestUnit{goUnit("m/a")}}, true},
		{"nothing", "NOTHING no change", manifest{Scope: "NOTHING"}, true},
		{"driver extra", "run m/a m/b", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"driver omission", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a"), goUnit("m/b")}}, false},
		{"scope", "run m/a", manifest{Scope: "FULL", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"unknown verdict", "future m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{goUnit("m/a")}}, false},
		{"empty run", "run ", manifest{Scope: "NARROWED"}, false},
		{"bad unit", "run m/a", manifest{Scope: "NARROWED", Units: []manifestUnit{{Adapter: "go", Unit: "m/a"}}}, false},
		{"nothing hides units", "NOTHING no change", manifest{Scope: "NOTHING", Units: []manifestUnit{goUnit("m/a")}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := contractAgreement(tc.audit, tc.manifest); (err == nil) != tc.pass {
				t.Fatalf("pass=%v, error=%v", tc.pass, err)
			}
		})
	}
}
