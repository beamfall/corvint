package testsupport

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ParentPIDEnv names the environment variable a test sets, to its own pid, on
// a self-exec helper that waits for a signal or event from it.
const ParentPIDEnv = "CORVINT_TEST_HELPER_PARENT"

// ExitWhenParentGone bounds a self-exec test helper's lifetime by its
// parent's (V1-1498). A helper that blocks until its parent signals it would
// otherwise wait forever once the parent dies without running its cleanup
// (a timeout panic, SIGKILL, an interrupted lane). Reparenting is the observed
// event: once the pid in ParentPIDEnv is no longer this process's parent, the
// helper exits 3, at once when the parent is already gone at the call. Without
// the variable it does nothing. Windows does not
// reparent, so there it bounds nothing; the helpers that need it are unix.
func ExitWhenParentGone() {
	value := os.Getenv(ParentPIDEnv)
	if value == "" {
		return
	}
	want, err := strconv.Atoi(value)
	if err != nil || want <= 0 {
		fmt.Fprintf(os.Stderr, "test helper: invalid parent pid %q\n", value)
		os.Exit(3)
	}
	exitIfGone := func() {
		if os.Getppid() != want {
			fmt.Fprintln(os.Stderr, "test helper: parent gone; exiting")
			os.Exit(3)
		}
	}
	exitIfGone()
	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			exitIfGone()
		}
	}()
}

// ParentPIDEntry is the ParentPIDEnv entry naming this process, for a helper's
// environment.
func ParentPIDEntry() string { return ParentPIDEnv + "=" + strconv.Itoa(os.Getpid()) }
