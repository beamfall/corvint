//go:build darwin || linux

package worksource

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/Beamfall/corvint/internal/groupreap"
)

func setupGitProcess(command *exec.Cmd) {
	// Cancellation stops the leader; groupreap.Drain sweeps the group
	// before the reap, after the output pipes drain or their bound expires.
	groupreap.Contain(command)
}
func fileDevice(info os.FileInfo) uint64 { return uint64(info.Sys().(*syscall.Stat_t).Dev) }
func fileLinks(info os.FileInfo) uint64  { return uint64(info.Sys().(*syscall.Stat_t).Nlink) }
func noFollowReadFlags() int             { return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK }

func fileIdentity(info os.FileInfo) [2]uint64 {
	stat := info.Sys().(*syscall.Stat_t)
	return [2]uint64{uint64(stat.Dev), uint64(stat.Ino)}
}
