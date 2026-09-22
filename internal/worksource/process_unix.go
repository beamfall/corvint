//go:build darwin || linux

package worksource

import (
	"os"
	"os/exec"
	"syscall"
)

func setupGitProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
func cleanupGitProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
func fileDevice(info os.FileInfo) uint64 { return uint64(info.Sys().(*syscall.Stat_t).Dev) }
func fileLinks(info os.FileInfo) uint64  { return uint64(info.Sys().(*syscall.Stat_t).Nlink) }
func noFollowReadFlags() int             { return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK }

func fileIdentity(info os.FileInfo) [2]uint64 {
	stat := info.Sys().(*syscall.Stat_t)
	return [2]uint64{uint64(stat.Dev), uint64(stat.Ino)}
}
