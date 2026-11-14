// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"strings"
	"testing"
)

// TestNCMV1019SelectorMakeFlagsAssignment pins V1-5561 in the selector: a
// make file that assigns MAKEFLAGS or MAKEOVERRIDES passes `PKG=./tool` to
// the make its recipe runs, and to its own run, where it beats the file's
// own PKG, so GNU make runs `go test ./tool` while the scan reads
// `./other`. Changing a recipe line of such a file runs everything with the
// variable's reason from Select, SelectStructural and SelectQualified, for
// each form; a file that only references MAKEFLAGS stays narrowed.
func TestNCMV1019SelectorMakeFlagsAssignment(t *testing.T) {
	files := map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"tool/tool.go":          "package tool\n",
		"tool/tool_test.go":     makeFixtureTest("tool", ""),
		"other/other.go":        "package other\n",
		"other/other_test.go":   makeFixtureTest("other", ""),
		"flags.mk":              "MAKEFLAGS += PKG=./tool\n",
		"mkread/mkread_test.go": makeFixtureTest("mkread", "var makefile = \"../Makefile\"\n\n"),
	}
	// Each edit puts `: ; ` before the first recipe line, which belongs to
	// target.
	for _, tc := range []struct {
		name, makefile, target, reason string
	}{
		{"append", "PKG ?= ./other\nMAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"plain", "PKG ?= ./other\nMAKEFLAGS = PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"target specific", "PKG ?= ./other\ntop: MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"plain default", "PKG = ./other\nMAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"overrides", "PKG ?= ./other\nMAKEOVERRIDES = PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEOVERRIDES"},
		{"non-recursive", "PKG ?= ./other\nMAKEFLAGS += PKG=./tool\ntop: check\ncheck:\n\tgo test $(PKG)\n", "check", "make:makeflags-assignment:MAKEFLAGS"},
		{"override", "PKG ?= ./other\noverride MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"define", "PKG ?= ./other\ndefine MAKEFLAGS\nPKG=./tool\nendef\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
		{"included", "PKG ?= ./other\ninclude flags.mk\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:include"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files["Makefile"] = tc.makefile
			root, base := baseRepository(t, files, nil)
			writeMakefile(t, root, strings.Replace(tc.makefile, "\n\t", "\n\t: ; ", 1))
			want := `FALLBACK never-narrowed-path Makefile: make edit: make target "` + tc.target + `" runs a command or make goal that does not resolve: ` + tc.reason
			requireVerdict(t, makeEditPlan(base, "Makefile"), root, want)
			requireQualified(t, makeEditPlan(base, "Makefile"), root, want)
		})
	}
	files["Makefile"] = "PKG ?= ./other\ntop:\n\t$(MAKE) check\ncheck:\n\techo $(MAKEFLAGS); go test $(PKG)\n"
	root, base := baseRepository(t, files, nil)
	writeMakefile(t, root, strings.Replace(files["Makefile"], "\n\t", "\n\t: ; ", 1))
	requireVerdict(t, makeEditPlan(base, "Makefile"), root, "run "+modules("mkread", "other"), "noncode make-targets Makefile -> top")
}
