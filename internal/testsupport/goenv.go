package testsupport

import "strings"

// GoEnv returns env for a go build, go run or go test subprocess a test starts
// in a temporary fixture: its last GOFLAGS entry gains -buildvcs=false, or
// GOFLAGS=-buildvcs=false is appended when env sets none. Without it, a fixture
// under a directory holding a stray or empty .git fails VCS stamping with
// "error obtaining VCS status: exit status 128" (AFP-V0-7920, V1-4501). An
// entry that already names -buildvcs is kept as the caller set it. Production
// builds and release stamping never use it. env itself is not modified.
func GoEnv(env []string) []string {
	result := append([]string(nil), env...)
	for index := len(result) - 1; index >= 0; index-- {
		value, found := strings.CutPrefix(result[index], "GOFLAGS=")
		if !found {
			continue
		}
		for _, flag := range strings.Fields(value) {
			if flag == "-buildvcs" || strings.HasPrefix(flag, "-buildvcs=") || strings.HasPrefix(flag, "--buildvcs") {
				return result
			}
		}
		result[index] = "GOFLAGS=" + strings.TrimSpace(value+" -buildvcs=false")
		return result
	}
	return append(result, "GOFLAGS=-buildvcs=false")
}
