// Package registry composes explicit experimental runner profiles without
// letting one language's result hide another provider's unknown frontier.
//
// Dispatch is table-driven: each runner family registers its runner names,
// its Build and Parse, and the runners that admit exec members, and the
// registry names no runner itself (docs/specs/language-plugins-v0.md,
// LGP-V0-010..013).
package registry

import (
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
	"github.com/Beamfall/corvint/internal/testrunner/mobile"
	"github.com/Beamfall/corvint/internal/testrunner/native"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
	sqlrunner "github.com/Beamfall/corvint/internal/testrunner/sql"
)

// Family is one registered runner plugin family.
type Family struct {
	// Name is the family's package, relative to internal/.
	Name    string
	Runners func() []string
	Build   func(testrunner.Request) (testrunner.Invocation, error)
	Parse   func(testrunner.Input) (testrunner.Observation, error)
	// ExecMemberRunners names the family's runners that admit ExecPrefix,
	// PathMap and Formatter (TRE-V0-081); nil admits none.
	ExecMemberRunners func() []string
}

// families is the dispatch table, in lookup order (LGP-V0-010).
var families = []Family{
	{Name: "testrunner/dynamic", Runners: dynamic.Runners, Build: dynamic.Build, Parse: dynamic.Parse, ExecMemberRunners: dynamic.ExecMemberRunners},
	{Name: "testrunner/native", Runners: native.Runners, Build: native.Build, Parse: native.Parse},
	{Name: "testrunner/platform", Runners: platform.Runners, Build: platform.Build, Parse: platform.Parse},
	{Name: "testrunner/mobile", Runners: mobile.Runners, Build: mobile.Build, Parse: mobile.Parse},
	{Name: "testrunner/sql", Runners: sqlrunner.Runners, Build: sqlrunner.Build, Parse: sqlrunner.Parse},
}

// Families returns a copy of the dispatch table.
func Families() []Family { return append([]Family(nil), families...) }

// Lookup returns the family that registers runner.
func Lookup(runner string) (Family, bool) {
	for _, f := range families {
		if has(f.Runners(), runner) {
			return f, true
		}
	}
	return Family{}, false
}

func Runners() []string {
	var out []string
	for _, f := range families {
		out = append(out, f.Runners()...)
	}
	sort.Strings(out)
	return out
}

func execMemberRunners() []string {
	var out []string
	for _, f := range families {
		if f.ExecMemberRunners != nil {
			out = append(out, f.ExecMemberRunners()...)
		}
	}
	sort.Strings(out)
	return out
}

func has(list []string, name string) bool {
	for _, v := range list {
		if v == name {
			return true
		}
	}
	return false
}

// Build admits r through testrunner.BuildWith, the shared admission, and
// then builds it with the family that registers its runner.
func Build(r testrunner.Request) (testrunner.Invocation, error) {
	return testrunner.BuildWith(r, execMemberRunners(), func(r testrunner.Request) (testrunner.Invocation, error) {
		if f, ok := Lookup(r.Runner); ok {
			return f.Build(r)
		}
		return testrunner.Invocation{}, fmt.Errorf("unsupported runner")
	})
}

// Parse parses in with the family that registers its runner and normalizes
// the observation through testrunner.ParseWith.
func Parse(in testrunner.Input) (testrunner.Observation, error) {
	return testrunner.ParseWith(in, func(in testrunner.Input) (testrunner.Observation, error) {
		if f, ok := Lookup(in.Runner); ok {
			return f.Parse(in)
		}
		return testrunner.Observation{}, fmt.Errorf("unsupported runner")
	})
}
