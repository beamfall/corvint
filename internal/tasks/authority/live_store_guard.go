// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ForbidCommonDirEnv names one Git common directory whose store no writer in this
// process, or in any child that inherits the environment, may lock (PUR-V0-1921). A Go
// test binary sets it to its own source checkout's common directory before any test
// runs, so a helper or a built corvint-tasks child launched with the wrong working
// directory refuses the live store instead of pausing or mutating it (V1-4261).
const ForbidCommonDirEnv = "CORVINT_TASKS_FORBID_COMMON_DIR"

// testSourceCommonDir is the common directory of the checkout this test binary was
// compiled from, or "" outside a Go test binary or when the source path is not
// absolute (a -trimpath build) or resolves to no repository (PUR-V0-1920).
var testSourceCommonDir = sync.OnceValue(func() string {
	if !isTestBinary(os.Args) {
		return ""
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(file) {
		return ""
	}
	common, err := intent.ResolveCommonDir(filepath.Dir(file))
	if err != nil {
		return ""
	}
	return common
})

func init() {
	if dir := testSourceCommonDir(); dir != "" && os.Getenv(ForbidCommonDirEnv) == "" {
		os.Setenv(ForbidCommonDirEnv, dir)
	}
}

// isTestBinary reports whether argv[0] names a `go test` binary (PKG.test, or
// PKG.test.exe). It deliberately avoids importing package testing into production code.
func isTestBinary(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return strings.HasSuffix(strings.TrimSuffix(filepath.Base(args[0]), ".exe"), ".test")
}

// refuseForbiddenStore is the writer-lock guard (PUR-V0-1920, PUR-V0-1921): it refuses
// OUT_OF_SCOPE before any lock file is opened when commonDir is the test binary's own
// source checkout or the directory ForbidCommonDirEnv names. Reads never lock, so they
// are unaffected.
func refuseForbiddenStore(commonDir string) error {
	for _, forbidden := range []string{testSourceCommonDir(), os.Getenv(ForbidCommonDirEnv)} {
		if forbidden != "" && sameDirectory(forbidden, commonDir) {
			return wire.Errorf(wire.CodeOutOfScope, commonDir,
				"this process may not write the store at %s: it is a test process's own checkout or the directory %s names (PUR-V0-1920, PUR-V0-1921); fix: run the command in a fixture repository (t.TempDir, or a disposable `git init`), never the live checkout",
				commonDir, ForbidCommonDirEnv)
		}
	}
	return nil
}

func sameDirectory(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}
