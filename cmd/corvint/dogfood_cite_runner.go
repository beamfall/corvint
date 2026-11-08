// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// dogfoodCitationRunner keeps the selected executable's identity and runs its
// citation operations in-process only when it is this executable. In particular,
// an explicitly selected older verifier must never execute this tree's code.
// DCW-V0-335: prepare still uses the public cite parser and refusal formatter.
func dogfoodCitationRunner(path, self string) dogfoodflow.Runner {
	runner := dogfoodflow.Exec(path)
	selected, selectedErr := os.Stat(path)
	current, currentErr := os.Stat(self)
	if selectedErr != nil || currentErr != nil || !os.SameFile(selected, current) {
		return runner
	}
	external := runner.Run
	runner.Run = func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		if len(args) >= 2 && args[0] == "cem" && (args[1] == "cite" || args[1] == "mark") {
			return runContext(ctx, append([]string{"--root", root}, args...), nil, stdout, stderr)
		}
		return external(ctx, root, args, stdout, stderr)
	}
	return runner
}
