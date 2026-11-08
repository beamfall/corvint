// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

func parseTestEvidenceInvocation(args []string) (string, []string, bool) {
	i, root := 0, ""
	for i < len(args) && (args[i] == "--root" && rootPreambleValue(args, i+1) || strings.HasPrefix(args[i], "--root=")) {
		if args[i] == "--root" {
			root = args[i+1]
			i += 2
		} else {
			root = strings.TrimPrefix(args[i], "--root=")
			i++
		}
	}
	if i == len(args) || args[i] != "test-evidence" {
		return "", nil, false
	}
	return root, args[i+1:], true
}
func runTestEvidence(ctx context.Context, root string, args []string, out, diag io.Writer) int {
	q, format, err := testevidencedoc.Parse(args)
	if err != nil {
		emitError(diag, argumentError("invalid test-evidence arguments"))
		return 2
	}
	if err = testevidencedoc.Write(out, testevidencedoc.Read(ctx, root, q), format); err != nil {
		emitError(diag, err)
		return 2
	}
	return 0
}

const testEvidenceHelp = `Read retained test dependencies at an immutable commit.

Usage:
  corvint [--root PATH] test-evidence PATH [--lines START:END] [--revision COMMIT] [--index FILE] [--format json|text]
  corvint [--root PATH] test-evidence --test TEST [--revision COMMIT] [--index FILE] [--format json|text]
  corvint [--root PATH] test-evidence --explain TEST --base COMMIT [--revision COMMIT] [--index FILE] [--format json|text]

Exactly one path, --test or --explain is required. --explain also requires --base.
--revision defaults to HEAD. --lines is an inclusive line range. --index reads
an existing repository-relative linecov index, alongside .corvint/test-evidence.
Default output is JSON (corvint-test-evidence-query/1); --format text is readable.
Records include their components, digest and commit. Missing inventory is UNKNOWN
inventory-unavailable, missing run context is run-context-unresolved, absent fresh
records are no-fresh-record, and stale-record includes the freshness reason.
Explanations use the shadow selector; without admitted qualification runners are
FULL unqualified. No inventory, qualification or run identity is inferred from a
record. This command writes nothing, runs no tests and executes no runner.

Example:
  corvint test-evidence src/main.go --lines 10:20 --revision HEAD
  corvint test-evidence --test suite/TestExample
  corvint test-evidence --explain suite/TestExample --base HEAD~1
`
