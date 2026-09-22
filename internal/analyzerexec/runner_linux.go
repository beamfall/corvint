//go:build linux

package analyzerexec

import "context"

func containedBackendSupported() bool { return false }

// Linux remains unsupported until a namespace/seccomp/cgroup/pidfd backend is
// available. A process group alone is not containment.
func runContained(context.Context, sealedArtifact, Plan) (Result, error) {
	return noStartResult(), &Error{Failure: Unsupported}
}
