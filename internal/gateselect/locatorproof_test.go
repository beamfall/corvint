// SPDX-License-Identifier: Apache-2.0
package gateselect

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
				got := comparisonOnlyLocators(body, map[string]string{"system": "os"})
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

// AZU-V0-9930 exercises the mirrored gate scanner, not just the proof helper.
func TestLocatorComparisonScanner_AZU_V0_9930(t *testing.T) {
	aliases := map[string]string{"system": "os"}
	guard := []byte("package p\nfunc f() { cwd, _ := system.Getwd(); if cwd != \"/product\" { return } }\n")
	reader := []byte("package p\nfunc f() { cwd, _ := system.Getwd(); if cwd != \"/product\" { return }; consume(cwd) }\n")
	if got := scanSource(guard, aliases, nil).calls; len(got) != 0 {
		t.Fatalf("guard calls %v", got)
	}
	if got := scanSource(reader, aliases, nil).calls; len(got) != 1 {
		t.Fatalf("reader calls %v", got)
	}
}
