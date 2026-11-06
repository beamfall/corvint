package store_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// leaderFault, when set by a platform test file, replaces the lane leader
// while CORVINT_TEST_LEADER_FAULT is set in its environment.
var leaderFault func(dir string)

var checkTestLeaders = func() error { return nil }

// testRootEnv carries the package root test process's executable to every
// process it starts. A test that re-executes the binary as a child inherits
// it, so only the root, whose m.Run spans every test, checks for leaked lane
// leaders. A child's check matched the live leaders of the root's parallel
// tests by the same executable path and killed them after its grace
// (ATR-V0-7202, V1-4782).
const testRootEnv = "CORVINT_TEST_STORE_ROOT"

// idleLeaderEnv makes a lane-leader invocation hold until its standard input
// closes, standing in for another test's live leader (V1-4782).
const idleLeaderEnv = "CORVINT_TEST_IDLE_LEADER"

// holderStopEnv names a file whose creation, or its directory's removal,
// releases the output holder (V1-2625).
const holderStopEnv = "CORVINT_TEST_HOLDER_STOP"

// holdOutput keeps the inherited standard output open. Without a stop file it
// holds 3s. With one it holds until the test creates the file or removes its
// directory, at most two minutes: a fixed hold ended before the drain under
// load, so the drain proved the output closed and the stage was released.
func holdOutput() {
	stop := os.Getenv(holderStopEnv)
	if stop == "" {
		time.Sleep(3 * time.Second)
		return
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(filepath.Dir(stop)); err != nil {
			return
		}
		if _, err := os.Stat(stop); err == nil {
			return
		}
	}
}

// TestMain lets supervised-workflow tests use the test binary itself as the
// pinned lane leader (`self lane-leader --directory DIR --capsule DIGEST`).
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "output-holder" {
		holdOutput()
		os.Exit(0)
	}
	if len(os.Args) == 6 && os.Args[1] == "lane-leader" {
		if os.Getenv(idleLeaderEnv) != "" {
			_, _ = io.Copy(io.Discard, os.Stdin)
			os.Exit(0)
		}
		if leaderFault != nil && os.Getenv("CORVINT_TEST_LEADER_FAULT") != "" {
			leaderFault(os.Args[3])
		}
		if e := supervisor.Leader(context.Background(), os.Args[3], os.Args[5]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		os.Exit(0)
	}
	// safeopen refuses symlinked ancestors, and the macOS default temp root
	// sits under /var -> /private/var, so test dirs come from its resolved
	// path (V1-0753).
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		os.Setenv("TMPDIR", dir)
	}
	// Every git the tests spawn runs without the account's global and system
	// configuration, as testsupport.IsolateGit does (V1-2633). Tasks imports
	// no Core package (TestImportDirection), so the variables are set here.
	for _, variable := range [][2]string{
		{"GIT_CONFIG_GLOBAL", os.DevNull},
		{"GIT_CONFIG_NOSYSTEM", "1"},
		{"GIT_CONFIG_PARAMETERS", "'maintenance.auto'='false' 'gc.auto'='0'"},
		{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.test"},
		{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.test"},
	} {
		if err := os.Setenv(variable[0], variable[1]); err != nil {
			panic(err)
		}
	}
	self, err := os.Executable()
	if err != nil {
		panic(err)
	}
	root := os.Getenv(testRootEnv) != self
	if root {
		if err := os.Setenv(testRootEnv, self); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	if root {
		if err := checkTestLeaders(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}
