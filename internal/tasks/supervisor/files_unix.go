//go:build darwin || linux

package supervisor

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"syscall"
)

func DirectoryIdentity(path string) (string, error) {
	r, e := safeopen.Root(path)
	if e != nil {
		return "", e
	}
	defer r.Close()
	st, e := r.Stat(".")
	if e != nil {
		return "", e
	}
	native, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("directory identity unavailable")
	}
	return fmt.Sprintf("%d:%d", native.Dev, native.Ino), nil
}
