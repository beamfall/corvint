// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"io"
)

func runTestCommand(ctx context.Context, arguments []string, stdout, stderr io.Writer) (int, bool) {
	if root, rest, ok := parseTestEvidenceInvocation(arguments); ok {
		return runTestEvidence(ctx, root, rest, stdout, stderr), true
	}
	return 0, false
}
