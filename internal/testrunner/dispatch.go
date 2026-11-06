package testrunner

import (
	"fmt"
	"slices"
	"strings"
)

// BuildWith applies the admission every runner dispatch applies before a
// family builds, then calls build: the request's selection is admitted
// (AdmitSelection), and ExecPrefix, PathMap and Formatter are refused for a
// runner outside execMemberRunners (TRE-V0-081, LGP-V0-011). The registry
// and a caller fixed to one family share it, so neither can skip a guard.
func BuildWith(r Request, execMemberRunners []string, build func(Request) (Invocation, error)) (Invocation, error) {
	if err := AdmitSelection(r); err != nil {
		return Invocation{}, err
	}
	if r.HasRSpecOnlyMembers() && !slices.Contains(execMemberRunners, r.Runner) {
		return Invocation{}, fmt.Errorf("execPrefix, pathMap and formatter are admitted only for %s", strings.Join(execMemberRunners, ", "))
	}
	return build(r)
}

// ParseWith calls parse and normalizes its observation against in
// (Normalize), returning parse's error unchanged.
func ParseWith(in Input, parse func(Input) (Observation, error)) (Observation, error) {
	o, err := parse(in)
	return Normalize(in, o), err
}
