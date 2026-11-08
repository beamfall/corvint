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
			if err := contractAgreement(tc.audit, tc.manifest, []string{"m/a"}); (err == nil) != tc.pass {
				t.Fatalf("pass=%v, error=%v", tc.pass, err)
			}
		})
	}
}

// AFP-V0-9501: scope labels and unknown unit forms cannot certify FULL coverage.
func TestSelectionContractFullCoverage_AFPV09501(t *testing.T) {
	root := t.TempDir()
	baselineFile(t, root, "go.mod", "module example.test/full\n\ngo 1.22\n")
	baselineFile(t, root, "a/a.go", "package a\n")
	baselineFile(t, root, "b/b_test.go", "package b\nimport \"testing\"\nfunc TestB(t *testing.T) {}\n")
	baselineFile(t, root, "nested/go.mod", "module example.test/nested\n")
	baselineFile(t, root, "nested/n.go", "package nested\n")
	baselineFile(t, root, "nested/.hidden/h.go", "package hidden\n")
	universe, err := contractUniverse(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"example.test/full/a", "example.test/full/b"}; !reflect.DeepEqual(universe, want) {
		t.Fatalf("universe=%v, want %v", universe, want)
	}
	a := manifestUnit{Adapter: "go", Unit: "go:example.test/full/a"}
	b := manifestUnit{Adapter: "go", Unit: "go:example.test/full/b"}
	for _, tc := range []struct {
		name  string
		units []manifestUnit
		pass  bool
	}{
		{"complete", []manifestUnit{a, b}, true},
		{"unreadable declaration", nil, false},
		{"truncated", []manifestUnit{a}, false},
		{"unknown full form", []manifestUnit{{Adapter: "go", Unit: "go:future-full-universe"}}, false},
		{"declared command is not package evidence", []manifestUnit{{Adapter: "go", Unit: "go:./..."}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest{Scope: "FULL", Units: tc.units}
			if tc.name == "unreadable declaration" {
				m.Incomplete = []string{"the declared full scope is unreadable"}
			}
			if err := contractAgreement("FALLBACK unknown frontier", m, universe); (err == nil) != tc.pass {
				t.Fatalf("pass=%v error=%v", tc.pass, err)
			}
		})
	}
	if err := contractAgreement("FALLBACK unknown frontier", manifest{Scope: "FULL", Units: []manifestUnit{a, b}}, nil); err == nil {
		t.Fatal("unobserved universe passed")
	}
	// Hidden root packages are part of the explicit root execution universe.
	baselineFile(t, root, ".ci/ci.go", "package ci\n")
	hiddenUniverse, err := contractUniverse(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"example.test/full/.ci", "example.test/full/a", "example.test/full/b"}; !reflect.DeepEqual(hiddenUniverse, want) {
		t.Fatalf("hidden universe=%v, want %v", hiddenUniverse, want)
	}
	if err := contractAgreement("FALLBACK unknown frontier", manifest{Scope: "FULL", Units: []manifestUnit{a, b}}, hiddenUniverse); err == nil {
		t.Fatal("hidden package omission passed")
	}
	baselineFile(t, root, "go.mod", "not a module declaration\n")
	if _, err := contractUniverse(context.Background(), root); err == nil {
		t.Fatal("unreadable universe passed")
	}
}
