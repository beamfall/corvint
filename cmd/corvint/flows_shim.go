// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"strings"
)

// flowsHelp is the `flows` topic of a binary that holds none of the flows commands but
// test-evidence: the group is an exec-only shim onto corvint-docs (decision 0595, CMS-V0-913).
const flowsHelp = `Application flows and assertions, served by corvint-docs.

Usage:
  corvint [--root PATH] flows [SUBCOMMAND] [OPTIONS]
  corvint [--root PATH] flows appmap build --manifest FILE [--revision REV] [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]... [--workspace FILE | --no-workspace] [--from MAP [--verify]]

The flows commands are not in the corvint binary (decision 0595). This group runs the
corvint-docs executable beside corvint or on PATH with the same arguments, passes it no
authority, and prints its bytes. When it is absent the shim refuses with code not-installed
and names the NOT_INSTALLED gap. The subcommands (coverage, gaps, impact, map, navigate,
record, appmap, export, import, ingest, ingest-coverage, docs, stability, status, request-path),
their options and their help are corvint-docs's own: see corvint-docs help flows and
corvint-docs help --json. corvint keeps the adoption chain's build-step help
(corvint flows appmap build --help) and one verb, which reads retained test evidence:
` + flowsTestEvidenceHelp + `Maturity: experimental.
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
// unchanged (CMS-V0-913). `flows test-evidence` alone stays native, because its evidence
// projection is the tests layer's and the docs product does not import it (CMS-V0-914). Neither
// verb takes --require-clean, so the shim itself refuses it (DIRTY-CACHE-016) whether or not
// corvint-docs is installed.
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
	return forwardToDocs(ctx, args, "corvint "+verb, stdin, stdout, stderr), true
}

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
