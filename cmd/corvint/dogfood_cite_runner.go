// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// dogfoodCitationRunner keeps the selected executable's identity and runs its
// citation operations in-process only when it is this executable. In particular,
// an explicitly selected older verifier must never execute this tree's code.
// DCW-V0-335: prepare still uses the public cite parser and refusal formatter.
func dogfoodCitationRunner(path, self string) dogfoodflow.Runner {
	runner := dogfoodflow.Exec(path)
	// Admit only an executable resolved by the same lookup as Exec. A bare
	// name in the working directory does not establish what PATH will execute.
	resolved, err := exec.LookPath(path)
	if err != nil {
		return runner
	}
	selected, selectedErr := os.Stat(resolved)
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
