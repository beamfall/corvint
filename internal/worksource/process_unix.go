//go:build darwin || linux

package worksource

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

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

// gitGroupDirectory holds one record per Git process group a scratch-owning
// Source has started (WQO-V0-052).
const gitGroupDirectory = "git-groups"

// recordGitGroup gives command an inherited descriptor on a fresh, exclusively
// flocked record file. The lock lives as long as any process holding that
// descriptor, so it outlives a SIGKILLed producer exactly while the Git group
// does. record writes the group ID once the leader has started; release
// removes the record after the reap.
func (source *Source) recordGitGroup(command *exec.Cmd) (record, release func(), err error) {
	if source.temporary == "" {
		return func() {}, func() {}, nil
	}
	directory := filepath.Join(source.temporary, gitGroupDirectory)
	if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, nil, err
	}
	file, err := os.CreateTemp(directory, "group-")
	if err != nil {
		return nil, nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = os.Remove(file.Name())
		_ = file.Close()
		return nil, nil, err
	}
	command.ExtraFiles = append(command.ExtraFiles, file)
	record = func() { _, _ = file.WriteString(strconv.Itoa(command.Process.Pid) + "\n") }
	release = func() {
		_ = os.Remove(file.Name())
		_ = file.Close()
	}
	return record, release, nil
}

// RetireGitGroups kills every Git process group recorded under the
// acquisition scratch directories in parent whose record is still locked, then
// waits until each lock is released, that is until no process holding it
// remains. A SIGKILLed producer cannot reap its Git children, so the owner of
// parent calls this before removing it (WQO-V0-052). bound is a hang detector.
func RetireGitGroups(parent string, bound time.Duration) error {
	records, err := filepath.Glob(filepath.Join(parent, "corvint-work-source-*", gitGroupDirectory, "group-*"))
	if err != nil {
		return err
	}
	deadline := time.Now().Add(bound)
	var problems []error
	for _, path := range records {
		problems = append(problems, retireGitGroup(path, deadline))
	}
	return errors.Join(problems...)
}

func retireGitGroup(path string, deadline time.Time) error {
	file, err := os.OpenFile(path, noFollowReadFlags(), 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	fd := int(file.Fd())
	locked := func() (bool, error) {
		for {
			err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
			if err == nil {
				return true, nil
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return false, nil
			}
			return false, err
		}
	}
	if free, err := locked(); free || err != nil {
		return err
	}
	// A held lock means a holder is alive, and Git children stay in the
	// leader's group, so the recorded group ID cannot have been reused.
	raw, _ := io.ReadAll(io.LimitReader(file, 32))
	if text := string(raw); strings.HasSuffix(text, "\n") {
		if group, err := strconv.Atoi(strings.TrimSuffix(text, "\n")); err == nil && group > 1 {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
	}
	for {
		if free, err := locked(); free || err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("recorded Git process group %s did not exit", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
