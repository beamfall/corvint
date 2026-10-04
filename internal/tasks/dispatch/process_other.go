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
func observeProcs() (map[int]proc, error) { return nil, errPlatform }
func launch(argv, env []string, dir, logDir string) (int, string, <-chan int, error) {
	return 0, "", nil, errPlatform
}
func refreshTree(w *Worker, procs map[int]proc) error               { return errPlatform }
func leaderAlive(w *Worker) bool                                    { return false }
func signal(m Proc, s syscall.Signal)                               {}
func killTree(w *Worker, grace time.Duration) (bool, error)         { return false, errPlatform }
func busyChild(w *Worker, procs map[int]proc, ignore []string) bool { return false }

// Marker evidence cannot authorize a reader on an unsupported platform.
func openReaderMarker(string) (*os.File, error) { return nil, errPlatform }
