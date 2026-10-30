package testsupport

import (
	"os"
	"path/filepath"
)

// WorkRunParentEnvironment is worksource.RunParentEnvironment (WQO-V0-094). It is
// repeated because worksource's own tests import this package.
const WorkRunParentEnvironment = "CORVINT_WORK_RUN_PARENT"

// PrivateWorkRunParent points CORVINT_WORK_RUN_PARENT at a fresh private
// directory under the test temporary root, which follows TMPDIR, so the work run
// roots and source scratch a package's tests and their inheriting subprocesses
// allocate stay off the fixed /tmp (V1-2768). A value already set, by an
// enclosing test process or the operator, is kept. Call it from TestMain; cleanup
// removes only a directory this call created.
func PrivateWorkRunParent() (cleanup func() error, err error) {
	if _, set := os.LookupEnv(WorkRunParentEnvironment); set {
		return func() error { return nil }, nil
	}
	directory, err := os.MkdirTemp("", "corvint-work-run-parent-")
	if err != nil {
		return nil, err
	}
	cleanup = func() error { return os.RemoveAll(directory) }
	canonical, err := filepath.EvalSymlinks(directory)
	if err == nil {
		err = os.Chmod(canonical, 0700)
	}
	if err == nil {
		err = os.Setenv(WorkRunParentEnvironment, canonical)
	}
	if err != nil {
		_ = cleanup()
		return nil, err
	}
	return cleanup, nil
}
