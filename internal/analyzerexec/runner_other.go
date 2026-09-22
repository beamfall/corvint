//go:build !darwin && !linux

package analyzerexec

import "context"

func Run(context.Context, Plan) (Result, error) {
	return Result{CleanupState: CleanupNotRun, Termination: TerminationNotRun, CleanupError: CleanupErrorNone}, &Error{Failure: Unsupported}
}
