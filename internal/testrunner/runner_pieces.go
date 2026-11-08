// SPDX-License-Identifier: Apache-2.0

package testrunner

import "github.com/Beamfall/corvint/internal/langplugin/runnernames"

// RunnerPiece and RunnerPieces live in the runnernames leaf so the langplugin
// manifest declares them without importing the runner registry (RSP-V0-001).
type (
	RunnerPiece  = runnernames.RunnerPiece
	RunnerPieces = runnernames.RunnerPieces
)

// AbsentRunnerPieces explicitly keeps each legacy runner on the current path.
func AbsentRunnerPieces(runner string) RunnerPieces { return runnernames.AbsentRunnerPieces(runner) }
