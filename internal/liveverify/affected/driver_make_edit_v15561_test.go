// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"testing"
)

// makeFlagsRows are the make files of V1-5561, each checked against GNU
// make 4.4.1: an assignment to MAKEFLAGS or MAKEOVERRIDES, in any form,
// passes `PKG=./tool` to a make a recipe runs as a command-line override,
// which beats a `?=` default and a plain assignment alike, and make 4.4
// applies one placed in MAKEFLAGS to its own run as well, so `top: check`
// runs `go test ./tool` with no recursion at all. MFLAGS and GNUMAKEFLAGS
// are the same channel's other names. Target is the target each row's edit
// changes; reason is the list the reach reports for it.
var makeFlagsRows = []struct {
	name, makefile, target, reason string
}{
	{"append", "PKG ?= ./other\nMAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"plain", "PKG ?= ./other\nMAKEFLAGS = PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"target specific", "PKG ?= ./other\ntop: MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"plain default", "PKG = ./other\nMAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"overrides", "PKG ?= ./other\nMAKEOVERRIDES = PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEOVERRIDES"},
	{"non-recursive", "PKG ?= ./other\nMAKEFLAGS += PKG=./tool\ntop: check\ncheck:\n\tgo test $(PKG)\n", "check", "make:makeflags-assignment:MAKEFLAGS"},
	{"conditional default", "PKG ?= ./other\nMAKEFLAGS ?= PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"immediate", "PKG ?= ./other\nMAKEFLAGS := PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"override", "PKG ?= ./other\noverride MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"define", "PKG ?= ./other\ndefine MAKEFLAGS\nPKG=./tool\nendef\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"pattern specific", "PKG ?= ./other\n%: MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"target override overrides", "PKG ?= ./other\ntop: override MAKEOVERRIDES += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEOVERRIDES"},
	{"other target", "PKG ?= ./other\ngate: MAKEFLAGS += PKG=./tool\ngate: top\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	{"mflags", "PKG ?= ./other\nMFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MFLAGS"},
	{"gnumakeflags", "PKG ?= ./other\nGNUMAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:GNUMAKEFLAGS"},
	{"computed name", "PKG ?= ./other\nN = MAKEFLAGS\n$(N) += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:computed-name"},
	{"target computed name", "PKG ?= ./other\nN = MAKEFLAGS\ntop: $(N) += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:computed-name"},
	{"computed target", "PKG ?= ./other\nT = top\n$(T): MAKEFLAGS += PKG=./tool\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:makeflags-assignment:MAKEFLAGS"},
	// Make reads an included file's assignments as its own; every include
	// already leaves every target of the file unfollowed (AFP-V0-3121).
	{"included", "PKG ?= ./other\ninclude flags.mk\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:include"},
	{"included unmodelled", "PKG ?= ./other\ninclude flags.env\ntop:\n\t$(MAKE) check\ncheck:\n\tgo test $(PKG)\n", "top", "make:include"},
}

// makeFlagsFiles is the repository of a makeFlagsRows make file.
func makeFlagsFiles(makefile string) map[string]string {
	return map[string]string{
		"Makefile":  makefile,
		"flags.mk":  "MAKEFLAGS += PKG=./tool\n",
		"flags.env": "MAKEFLAGS += PKG=./tool\n",
	}
}

// TestReachMakeFlagsAssignment_NCMV1019 pins V1-5561 in the reach: every
// target of a make file that assigns MAKEFLAGS, MFLAGS, GNUMAKEFLAGS or
// MAKEOVERRIDES, in any form and for any target, is unfollowed with the
// variable's name, or computed-name for a name make computes; one that
// includes a file assigning one is unfollowed by its include. A reference
// to MAKEFLAGS stays bounded.
func TestReachMakeFlagsAssignment_NCMV1019(t *testing.T) {
	ctx := context.Background()
	for _, tc := range makeFlagsRows {
		t.Run(tc.name, func(t *testing.T) {
			root := makeReachRepository(t, makeFlagsFiles(tc.makefile))
			reach := ReachMakeTargets(ctx, root, "Makefile", []string{tc.target})
			if want := `make target "` + tc.target + `" runs a command or make goal that does not resolve: ` + tc.reason; reach.Reason != want {
				t.Fatalf("reason %q, want %q (paths %v)", reach.Reason, want, reach.Paths)
			}
		})
	}
	root := makeReachRepository(t, makeFlagsFiles("PKG ?= ./other\ntop:\n\t$(MAKE) check\ncheck:\n\techo $(MAKEFLAGS); go test $(PKG)\n"))
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"top"}); reach.Reason != "" || len(reach.Paths) != 1 || reach.Paths[0] != "other" {
		t.Fatalf("reference: reason %q, paths %v, want bounded to [other]", reach.Reason, reach.Paths)
	}
}
