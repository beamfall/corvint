//go:build !darwin && !linux

package worksource

import (
	"os"
	"os/exec"
)

func setupGitProcess(command *exec.Cmd)   {}
func cleanupGitProcess(command *exec.Cmd) {}
func fileDevice(info os.FileInfo) uint64  { return 0 }
func fileLinks(info os.FileInfo) uint64   { return 0 }
func noFollowReadFlags() int              { return os.O_RDONLY }

func fileIdentity(info os.FileInfo) [2]uint64 { return [2]uint64{} }
