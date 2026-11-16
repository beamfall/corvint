// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/flowreports"
)

// flowsHelp is the `flows` topic. The group is an exec-only shim onto corvint-docs (decision 0595,
// CMS-V0-913) except for the verbs that join flows with test evidence or results, which run here
// (PROD-DOCS-011): test-evidence (CMS-V0-914) and the status, impact and gaps reports (AFL-V1).
const flowsHelp = `Application flows and assertions: the test-joining reports run in corvint, the rest are served by corvint-docs.

Usage:
  corvint [--root PATH] flows [SUBCOMMAND] [OPTIONS]
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV] [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]... [--workspace FILE | --no-workspace] [--from MAP [--verify]]
  corvint [--root PATH] flows appmap build --manifest FILE --worktree [--no-workspace]

The flow reports that join flows with test evidence or results run in this binary with no
other executable installed: status, impact, gaps and test-evidence (PROD-DOCS-011, AFL-V1).
Every other flows subcommand (coverage, map, navigate, record, appmap, export, import, ingest,
ingest-coverage, docs, stability, request-path) is not in the corvint binary (decision 0595):
the group runs the corvint-docs executable beside corvint or on PATH with the same arguments,
passes it no authority, and prints its bytes. When it is absent the shim refuses with code
not-installed and names the NOT_INSTALLED gap. Those subcommands' options and help are
corvint-docs's own: see corvint-docs help flows and corvint-docs help --json. corvint keeps
the adoption chain's build-step help (corvint flows appmap build --help).
` + flowreports.Help + flowsTestEvidenceHelp + `Maturity: experimental.
`

// howHelp is the `how` topic: the verb is the same shim (decision 0595, CMS-V0-913).
const howHelp = `Answer "how do I GOAL" from an application map, served by corvint-docs.

Usage:
  corvint [--root PATH] how [OPTIONS] GOAL

how is not in the corvint binary (decision 0595). It runs the corvint-docs executable
beside corvint or on PATH with the same arguments, passes it no authority, and prints its
bytes: the site-knowledge-how/0 answer of corvint-docs flows appmap how. When corvint-docs
is absent the shim refuses with code not-installed and names the NOT_INSTALLED gap. The
options and their help are corvint-docs's own: see corvint-docs help how.
Maturity: experimental.

Example:
  corvint --root . how --map appmap-map.json --source e2e "ship an order"
`

// runFlowsShim forwards `[--root PATH] flows ...` and `[--root PATH] how ...` to corvint-docs
// unchanged (CMS-V0-913). The verbs that join flows with test evidence or results stay native:
// `flows test-evidence`, because its evidence projection is the tests layer's and the docs
// product does not import it (CMS-V0-914), and `flows status|impact|gaps` (internal/flowreports,
// AFL-V1-001, AFL-V1-007, PROD-DOCS-011). Neither verb takes --require-clean, so the shim itself
// refuses it (DIRTY-CACHE-016) whether or not corvint-docs is installed.
func runFlowsShim(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	position := commandPositionAfterRoots(args)
	if position < 0 || (args[position] != "flows" && args[position] != "how") {
		return 0, false
	}
	verb, rest := args[position], args[position+1:]
	if verb == "flows" && len(rest) > 0 && rest[0] == "test-evidence" {
		return runFlowsTestEvidence(ctx, lastRoot(args[:position]), rest[1:], stdout, stderr), true
	}
	for _, arg := range rest {
		if arg == "--" {
			break
		}
		if name, _, _ := strings.Cut(arg, "="); name == "--require-clean" {
			emitError(stderr, argumentError("argument --require-clean: corvint "+verb+" takes no --require-clean"))
			return 2, true
		}
	}
	if verb == "flows" && len(rest) > 0 && flowreports.IsVerb(rest[0]) {
		// AFL-V1-001, AFL-V1-007: the reports run here, on the shared readers, with or without
		// corvint-docs installed.
		return flowreports.Run(ctx, lastRoot(args[:position]), rest[0], rest[1:], stdout, stderr, flowReportsEnv), true
	}
	return forwardToDocs(ctx, args, "corvint "+verb, stdin, stdout, stderr), true
}

// flowReportsEnv hands the flow reports corvint's JSON error envelope and its affected-revision
// refusal for a --base that names no commit.
var flowReportsEnv = flowreports.Env{EmitError: emitError, BaseRefusal: affectedBaseRefusal}

// lastRoot is the root a leading run of --root options names: the last one, or "." for none.
func lastRoot(preamble []string) string {
	root := "."
	for index := 0; index < len(preamble); index++ {
		if value, inline := strings.CutPrefix(preamble[index], "--root="); inline {
			root = value
		} else if index+1 < len(preamble) {
			index++
			root = preamble[index]
		}
	}
	return root
}
