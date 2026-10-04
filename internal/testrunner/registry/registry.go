// Package registry composes explicit experimental runner profiles without
// letting one language's result hide another provider's unknown frontier.
package registry

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
	"github.com/Beamfall/corvint/internal/testrunner/mobile"
	"github.com/Beamfall/corvint/internal/testrunner/native"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
	sqlrunner "github.com/Beamfall/corvint/internal/testrunner/sql"
	"sort"
)

func Runners() []string {
	out := append(append(dynamic.Runners(), native.Runners()...), platform.Runners()...)
	out = append(out, sqlrunner.Runners()...)
	out = append(out, mobile.Runners()...)
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
func Build(r testrunner.Request) (testrunner.Invocation, error) {
	switch {
	case has(dynamic.Runners(), r.Runner):
		return dynamic.Build(r)
	case has(native.Runners(), r.Runner):
		return native.Build(r)
	case has(platform.Runners(), r.Runner):
		return platform.Build(r)
	case has(mobile.Runners(), r.Runner):
		return mobile.Build(r)
	case has(sqlrunner.Runners(), r.Runner):
		return sqlrunner.Build(r)
	}
	return testrunner.Invocation{}, fmt.Errorf("unsupported runner")
}
func Parse(in testrunner.Input) (testrunner.Observation, error) {
	var o testrunner.Observation
	var e error
	switch {
	case has(dynamic.Runners(), in.Runner):
		o, e = dynamic.Parse(in)
	case has(native.Runners(), in.Runner):
		o, e = native.Parse(in)
	case has(platform.Runners(), in.Runner):
		o, e = platform.Parse(in)
	case has(mobile.Runners(), in.Runner):
		o, e = mobile.Parse(in)
	case has(sqlrunner.Runners(), in.Runner):
		o, e = sqlrunner.Parse(in)
	default:
		e = fmt.Errorf("unsupported runner")
	}
	return testrunner.Normalize(in, o), e
}
