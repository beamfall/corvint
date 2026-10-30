//go:build !darwin && !linux

package worksource

import (
	"os"
	"os/exec"
	"time"
)

func setupGitProcess(command *exec.Cmd)  {}
func fileDevice(info os.FileInfo) uint64 { return 0 }
func fileLinks(info os.FileInfo) uint64  { return 0 }

// fileOwner reports no owner, so the run parent override is refused here.
func fileOwner(os.FileInfo) (int, bool) { return 0, false }
func noFollowReadFlags() int            { return os.O_RDONLY }

func fileIdentity(info os.FileInfo) [2]uint64 { return [2]uint64{} }

func (source *Source) recordGitGroup(*exec.Cmd) (release func(), err error) {
	return func() {}, nil
}

// RetireGitGroups has no process groups to retire on this platform.
func RetireGitGroups(string, time.Duration) error { return nil }
