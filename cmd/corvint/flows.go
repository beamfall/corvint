package main

import (
	"context"
	"io"
	"strings"

	flowscmd "github.com/Beamfall/corvint/internal/command/flows"
)

// flowsEnv hands the flows handlers the root JSON error envelope and the shared affected-revision
// refusal; the handlers themselves live in internal/command/flows.
var flowsEnv = flowscmd.Env{EmitError: emitError, BaseRefusal: affectedBaseRefusal}

func flowInvocation(args []string) (string, []string, bool) {
	root := "."
	for len(args) > 0 && strings.HasPrefix(args[0], "--root") {
		if strings.HasPrefix(args[0], "--root=") {
			root, args = strings.TrimPrefix(args[0], "--root="), args[1:]
			continue
		}
		if args[0] != "--root" || !rootPreambleValue(args, 1) {
			return "", nil, false
		}
		root, args = args[1], args[2:]
	}
	if len(args) > 0 && args[0] == "how" { // `corvint how` is `flows appmap how` (SKH-V0-810)
		return root, append([]string{"appmap", "how"}, args[1:]...), true
	}
	if len(args) == 0 || args[0] != "flows" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runFlows(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	return flowscmd.Run(ctx, root, args, out, diagnostic, flowsEnv)
}
