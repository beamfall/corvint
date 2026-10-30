//go:build darwin || linux

package worksource

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
func fileOwner(info os.FileInfo) (int, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(stat.Uid), true
}
func noFollowReadFlags() int { return os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK }

func fileIdentity(info os.FileInfo) [2]uint64 {
	stat := info.Sys().(*syscall.Stat_t)
	return [2]uint64{uint64(stat.Dev), uint64(stat.Ino)}
}

// gitGroupDirectory holds one record per Git process group a scratch-owning
// Source has started (WQO-V0-052).
const gitGroupDirectory = "git-groups"

// gitRunMarker is the inert Git configuration argument naming one Git run.
func gitRunMarker(token string) string { return "corvint.worksourcerun=" + token }

// recordGitGroup completes a record before Git exists: a fresh file named for
// a random run token, exclusively flocked. command inherits the locked
// descriptor and carries `-c corvint.worksourcerun=<token>` as its first
// arguments, so from fork on the lock is held exactly while some process of
// the run lives, and from exec on the Git group leader is identifiable by its
// arguments without a published PID. release removes the record after the reap.
func (source *Source) recordGitGroup(command *exec.Cmd) (release func(), err error) {
	if source.temporary == "" {
		return func() {}, nil
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(random[:])
	directory := filepath.Join(source.temporary, gitGroupDirectory)
	if err := os.Mkdir(directory, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	path := filepath.Join(directory, "group-"+token)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	release = func() {
		_ = os.Remove(path)
		_ = file.Close()
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		release()
		return nil, err
	}
	command.ExtraFiles = append(command.ExtraFiles, file)
	command.Args = append([]string{command.Args[0], "-c", gitRunMarker(token)}, command.Args[1:]...)
	return release, nil
}

// RetireGitGroups retires the Git runs recorded under the acquisition
// scratch directories in parent whose record is still locked, that is whose
// launcher died before reaping them (WQO-V0-052). Until each lock is released
// it finds the run's group leader by its marker argument and stops and kills
// that group under groupreap's identity pinning; a recorded number is never
// signalled. A free lock is a finished run and is never signalled. A nil
// result confirms that no recorded Git process remains; any error means the
// caller must keep the scratch. bound is a hang detector.
func RetireGitGroups(parent string, bound time.Duration) error {
	records, err := filepath.Glob(filepath.Join(parent, "corvint-work-source-*", gitGroupDirectory, "*"))
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
	free := func() (bool, error) {
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
	if done, err := free(); done || err != nil {
		return err
	}
	token, ok := strings.CutPrefix(filepath.Base(path), "group-")
	if !ok || len(token) != 32 || strings.Trim(token, "0123456789abcdef") != "" {
		return fmt.Errorf("unreadable live Git group record %s", path)
	}
	var signalErr error
	for {
		// A run between fork and exec does not carry its marker yet, so the
		// leader is looked for again until the lock is released.
		leaders, err := groupreap.GroupLeadersWithArgument(gitRunMarker(token))
		if err != nil {
			signalErr = err
		}
		for _, leader := range leaders {
			if err := groupreap.StopKillGroup(leader); err != nil && !errors.Is(err, syscall.ESRCH) {
				signalErr = err
			}
		}
		if done, err := free(); done || err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("recorded Git run %s is still alive: %v", path, signalErr)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
