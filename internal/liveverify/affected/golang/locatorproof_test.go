// SPDX-License-Identifier: Apache-2.0
package golang

import (
	"strings"
	"testing"
)

func TestComparisonOnlyLocators_AZU_V0_9930(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		safe       int
	}{
		{"literal guard", `func f() { cwd, err := system.Getwd(); if err != nil || cwd != "/product" { return } }`, 1},
		{"reversed comparison", `func f() { cwd, _ := system.Getwd(); if "/product" == cwd { return } }`, 1},
		{"two comparisons", `func f() { cwd, _ := system.Getwd(); if cwd == "/a" || cwd != "/b" { return } }`, 1},
		{"real read", `func f() { cwd, _ := system.Getwd(); system.ReadFile(cwd) }`, 0},
		{"comparison and real read", `func f() { cwd, _ := system.Getwd(); if cwd != "/product" { return }; system.ReadDir(cwd) }`, 0},
		{"escape closure", `func f() { cwd, _ := system.Getwd(); func() { consume(cwd) }() }`, 0},
		{"return escape", `func f() string { cwd, _ := system.Getwd(); return cwd }`, 0},
		{"alias escape", `func f() { cwd, _ := system.Getwd(); other := cwd; consume(other) }`, 0},
		{"variable comparison", `func f(want string) { cwd, _ := system.Getwd(); if cwd == want { return } }`, 0},
		{"shadowed name still reads", `func f() { cwd, _ := system.Getwd(); if cwd != "/a" { return }; func(cwd string) { if cwd != "/b" { return } }("x"); consume(cwd) }`, 0},
		{"local receiver", `func f(system fake) { cwd, _ := system.Getwd(); if cwd == "/a" { return } }`, 0},
		{"reassignment", `func f() { cwd, _ := system.Getwd(); cwd = "/a"; if cwd == "/a" { return } }`, 0},
		{"one safe one real", `func f() { cwd, _ := system.Getwd(); if cwd != "/a" { return }; other, _ := system.Getwd(); consume(other) }`, 1},
		{"syntax error", `func f() { cwd, _ := system.Getwd(); if cwd != "/a" {`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, prefix := range []string{"", "// header\npackage p\nimport system \"os\"\n"} {
				body := []byte(prefix + tc.body)
				got, _, _ := comparisonOnlyLocators(body, map[string]string{"system": "os"}, nil)
				if len(got) != tc.safe {
					t.Fatalf("proof count = %d, want %d (prefix %q)", len(got), tc.safe, prefix)
				}
				for offset := range got {
					if !strings.HasPrefix(string(body[offset:]), "Getwd") {
						t.Fatalf("wrong offset %d", offset)
					}
				}
			}
		})
	}
}

// AFP-V0-2950: every possible root initializer and assignment must be
// certified, including declarations before a conditional Getwd fallback.
func TestLocatorInitializerAllowlist_AFPV02950(t *testing.T) {
	for _, tc := range []struct {
		name, init, write string
		safe              int
	}{
		{name: "environment initializer", init: `root := system.Getenv("DATA_ROOT")`},
		{name: "unknown initializer", init: `root := mystery()`},
		{name: "parameter initializer", init: `root := given`},
		{name: "var environment initializer", init: `var root = system.Getenv("DATA_ROOT")`},
		{name: "unknown reassignment", init: `root := ""`, write: `root = mystery();`},
		{name: "other Getwd receiver", init: `root := ""`, write: `root, err = other.Getwd();`},
		{name: "empty initializer", init: `root := ""`, safe: 1},
		{name: "zero string", init: `var root string`, safe: 1},
		{name: "same locator reassignment", init: `root := ""`, write: `root, err = system.Getwd();`, safe: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`package p
func f(given string, other fake) {
` + tc.init + `; var err error
if root == "" { root, err = system.Getwd() }; _ = err
` + tc.write + `
system.ReadFile(fp.Join(root, "config.go"))
}`)
			safe, _, _ := comparisonOnlyLocators(body, map[string]string{"system": "os"}, map[string]string{"fp": "path/filepath"})
			if len(safe) != tc.safe {
				t.Fatalf("exempted %d locators, want %d", len(safe), tc.safe)
			}
		})
	}
}
