//go:build !darwin && !linux

package authority

import (
	"os"
	"runtime"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Unsupported platforms fail closed (§3.4: Windows is unsupported; §5.1
// names only Darwin and Linux). Every primitive refuses before touching the
// filesystem.

const platform = runtime.GOOS

func errUnsupportedPlatform() error {
	return wire.Errorf(wire.CodeUnsupportedFilesystem, "", "platform %s is unsupported (SPEC §5.1 qualifies Darwin and Linux only)", runtime.GOOS)
}

var (
	syncFile      = fullSync
	syncDirectory = fullSync
	flockNB       = flockExclusiveNB
	flockRelease  = flockUnlock
)

func flockExclusiveNB(int) error { return errUnsupportedPlatform() }

func flockUnlock(int) error { return errUnsupportedPlatform() }

func isWouldBlock(error) bool { return false }

func isLockUnsupported(error) bool { return true }

func isInterrupted(error) bool { return false }

func isFullSyncUnsupported(error) bool { return true }

func withFD(*os.File, func(int) error) error { return errUnsupportedPlatform() }

func observeFilesystem(*os.File) (Filesystem, error) {
	return Filesystem{Platform: platform}, errUnsupportedPlatform()
}

func fullSync(*os.File) error { return errUnsupportedPlatform() }
