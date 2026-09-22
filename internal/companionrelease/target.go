package companionrelease

import "fmt"

// supportedTarget is the only GOOS/GOARCH pair this package will build for.
// Every other target is a refusal, not a best-effort attempt: PUB-V0-004
// scopes the first companion cut to macOS arm64 and requires every other
// platform to read NOT_RUN rather than untested.
const supportedTarget = "darwin/arm64"

// NotRunTargets are recorded verbatim in every report so a reader never
// infers qualification for a platform this build did not attempt.
var NotRunTargets = []string{"darwin/amd64", "linux/amd64", "linux/arm64", "windows/amd64"}

func splitTarget(target string) (goos, arch string, err error) {
	for i := 0; i < len(target); i++ {
		if target[i] == '/' {
			return target[:i], target[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("target %q is not GOOS/GOARCH", target)
}

func validateTarget(target string) error {
	if target != supportedTarget {
		return fmt.Errorf("unsupported target %q: only %q is admitted; see NotRunTargets for the rest", target, supportedTarget)
	}
	return nil
}
