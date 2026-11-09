package main

import (
	"context"
	"io"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cemcandidate"
)

// runCEMInvocation runs the cem commands that own their whole argv before the
// general router: the stable verifier and the out-of-tree bundle.
func runCEMInvocation(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	// A help request for either action prints that action's help before the
	// closed argv parsers would refuse it (V1-2109).
	if topic, requested, err := parseHelpInvocation(args); requested && err == nil &&
		(topic == cemActionHelpPrefix+"verify-stable" || topic == cemActionHelpPrefix+"bundle") {
		if _, err := io.WriteString(stdout, helpText(topic)); err != nil {
			return 1, true
		}
		return 0, true
	}
	if code, handled := cemcli.RunStableInvocation(ctx, args, stdout, stderr); handled {
		return code, true
	}
	// The out-of-tree bundle command owns its closed argv (CSB-V0-001).
	if code, handled := cemcandidate.RunBundleInvocation(ctx, args, stdout, stderr); handled {
		return code, true
	}
	return 0, false
}
