//go:build darwin

package authority

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// XNU ABI, verified against Go 1.27.0's local cmd/vendor/golang.org/x/sys/
// unix/zsysnum_darwin_{arm64,amd64}.go and zsyscall_darwin_arm64.go.
const (
	fixtureLinkat   = 471
	fixtureRenameat = 465
	fixtureUnlinkat = 472
	fixtureMkdirat  = 475
)

func fixtureObserveMount(f *os.File, _ fixtureMount) (result fixtureMount, err error) {
	err = safeopen.Control(f, func(fd uintptr) error {
		var st syscall.Stat_t
		var fs syscall.Statfs_t
		if err := syscall.Fstat(int(fd), &st); err != nil {
			return err
		}
		if err := syscall.Fstatfs(int(fd), &fs); err != nil {
			return err
		}
		kind, on, from := cString(fs.Fstypename[:]), cString(fs.Mntonname[:]), cString(fs.Mntfromname[:])
		// Fsid plus the descriptor's mounted-on/from identities distinguish
		// mounted instances under the explicitly stable topology assumption.
		if !Classify("darwin", kind) || fs.Flags&mntLocal == 0 || fs.Fsid == (syscall.Fsid{}) || !strings.HasPrefix(on, "/") || from == "" {
			return fixtureUnsupported
		}
		result = fixtureMount{uint64(st.Dev), fmt.Sprintf("%s:%v", kind, fs.Fsid), on + "\x00" + from}
		return nil
	})
	return result, err
}

func fixtureRename(a *os.File, from string, b *os.File, to string, source, dest *os.File) error {
	return fixtureTwoPaths(fixtureRenameat, a, from, b, to, source, dest)
}
