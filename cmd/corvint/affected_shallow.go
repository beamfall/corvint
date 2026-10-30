package main

import (
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedShallowCode names a --base plan refused in a shallow clone (ASD-V0-062, V1-3230).
const affectedShallowCode = "shallow-repository"

// refuseAffectedShallow refuses a --base plan in a shallow clone: its grafted history can hide the
// base or the merge base, so the range would be wrong before anything says the clone is shallow.
// A git failure leaves the later range checks to report it.
func refuseAffectedShallow(invocation affectedInvocation) error {
	if invocation.Base == "" {
		return nil
	}
	stdout, err := affectedAbbrevGit(invocation.Root, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(stdout) != "true" {
		return nil
	}
	return &gokernel.Error{Code: affectedShallowCode, Message: "this repository is a shallow clone, so --base " + invocation.Base + " and its merge base may lie past the shallow boundary; run `git fetch --unshallow origin` (or reclone without --depth), then rerun"}
}
