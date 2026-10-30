//go:build !darwin && !linux

package dispatch

import (
	"errors"
	"os"
	"syscall"
	"time"
)

const platformSupported = false

type proc struct{ comm string }

var errPlatform = errors.New("dispatch is supported only on darwin and linux")

func lockExclusive(f *os.File) error      { return errPlatform }
func lockShared(f *os.File) error         { return errPlatform }
func observeProcs() (map[int]proc, error) { return nil, errPlatform }
func launch(argv, env []string, dir, logDir string) (int, string, <-chan exitStatus, error) {
	return 0, "", nil, errPlatform
}
func refreshTree(w *Worker, procs map[int]proc, exempt map[int]string) error { return errPlatform }
func leaderAlive(w *Worker) bool                                             { return false }
func signal(m Proc, s syscall.Signal)                                        {}
func killTree(w *Worker, grace time.Duration, exempt map[int]string) (bool, error) {
	return false, errPlatform
}
func detachedSupervisor(pid int, identity string, leader int) bool { return false }
func busyChild(w *Worker, procs map[int]proc, ignore []string, base childCPU, now time.Time) bool {
	return false
}
func preciseCPU(procs map[int]proc, pids []int)       {}
func dupForExec(f *os.File) (int, error)              { return 0, errPlatform }
func closeFD(fd int)                                  {}
func markCloseOnExec(fd int)                          {}
func execImage(path string, argv, env []string) error { return errPlatform }
func waitChild(pid int) <-chan exitStatus             { return nil }
func childSample(w *Worker, procs map[int]proc, ignore []string, now time.Time) childCPU {
	return childCPU{at: now}
}
func liveTrees() (map[int]bool, map[int]bool, bool)           { return nil, nil, false }
func treeMayRun(int, string, map[int]bool, map[int]bool) bool { return true }

// Marker evidence cannot authorize a reader on an unsupported platform.
func openReaderMarker(string) (*os.File, error) { return nil, errPlatform }
