// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"strings"

	flowscmd "github.com/Beamfall/corvint/internal/command/flows"
	"github.com/Beamfall/corvint/internal/diagnostic"
)

// flowsHelp is the `flows` topic: the application-flow group this binary took over from corvint
// (decision 0595, CMS-V0-912). `corvint flows ...` forwards here unchanged.
const flowsHelp = `Inspect experimental web-flow assertion candidates and runtime observations.

Usage:
  corvint-docs [--root PATH] flows --manifest FILE [--evidence FILE]
  corvint-docs [--root PATH] flows record --manifest FILE --evidence FILE --output FILE

Read-only inspection binds declared tracked source and test files to Git and emits
application-flow-report/0. Static assertions are candidates, not passing tests.
Stale, missing, unsupported and contradicted evidence remains visible. Universal
application completeness is never claimed. Core reads launch no browser or server.

The separate corvint-web-flows companion parses JS/TS and optionally observes an
explicitly trusted local app. Its proposed execution profile remains experimental.
record explicitly writes one screened private evidence file and refuses overwrite;
it does not update ranking or accept inferred intent. No automatic ledger is written.
` + flowscmd.IntentHelp + flowscmd.NavigateHelp + flowscmd.AppmapHelp + flowscmd.AppmapPlanHelp + flowscmd.RequestPathHelp + flowscmd.TestEvidenceHelp

// flowsEnv hands the flows handlers this binary's JSON error envelope and the affected-revision
// refusal corvint gives a --base that names no commit.
var flowsEnv = flowscmd.Env{EmitError: emitError, BaseRefusal: affectedBaseRefusal}

// affectedBaseRefusal is corvint's refusal of the same name, byte for byte, so `flows impact
// --base` refuses identically through either binary (CMS-V0-912).
func affectedBaseRefusal(base string) error {
	return refused("unsupported-affected-revision", "base "+base+" is not a commit in this repository", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: base},
		SupportedFixes: []string{"affected.use-commit-base", "affected.omit-base"},
	})
}

// flowInvocation claims `[--root PATH] flows ARGS` and `[--root PATH] how ARGS`, returning the
// root and the flows arguments; `how` is `flows appmap how` (SKH-V0-810).
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
	if len(args) > 0 && args[0] == "how" {
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
