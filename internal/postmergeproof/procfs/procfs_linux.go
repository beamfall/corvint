//go:build linux && (amd64 || arm64)

// SPDX-License-Identifier: AGPL-3.0-or-later

package procfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Supported reports whether this build reads the Linux procfs profile.
const Supported = true

const (
	bootIDPath       = "/proc/sys/kernel/random/boot_id"
	namespacePath    = "/proc/self/ns/pid"
	nativeStartPath  = "/bin/ps"
	statLimit        = 64 << 10
	cmdlineLimit     = 4 << 20
	nativeToolLimit  = 64 << 20
	nativeStartLimit = 256
)

// CaptureBirth reads one bracketed capture of pid in a fixed order: boot ID,
// stat, argv, executable link and bytes, PID namespace, native start, then
// argv, link, executable digest and stat again. PID identity is relative to
// this observer's procfs, so the namespace is the observer's own
// /proc/self/ns/pid, which also stays readable for a zombie target. A zombie
// has no argv or executable; those reads are empty rather than failures.
func CaptureBirth(ctx context.Context, pid int, executableLimit int64) (BirthCapture, error) {
	var c BirthCapture
	if pid <= 0 || executableLimit <= 0 {
		return c, errors.New("procfs capture: invalid PID or executable bound")
	}
	dir := "/proc/" + strconv.Itoa(pid)
	var err error
	if c.BootID, err = readBounded(bootIDPath, 4096); err != nil {
		return c, err
	}
	if c.StatBefore, err = readBounded(dir+"/stat", statLimit); err != nil {
		return c, err
	}
	zombie := zombieStat(c.StatBefore)
	optional := func(data []byte, err error) ([]byte, error) {
		if err != nil && zombie {
			return []byte{}, nil
		}
		return data, err
	}
	if c.Cmdline, err = optional(readBounded(dir+"/cmdline", cmdlineLimit)); err != nil {
		return c, err
	}
	if c.ExecutableLink, err = optional(readLink(dir + "/exe")); err != nil {
		return c, err
	}
	if c.Executable, err = optional(readBounded(dir+"/exe", executableLimit)); err != nil {
		return c, err
	}
	if c.NamespaceLink, err = readLink(namespacePath); err != nil {
		return c, err
	}
	var st syscall.Stat_t
	if err := syscall.Stat(namespacePath, &st); err != nil {
		return c, fmt.Errorf("procfs capture: PID namespace: %w", err)
	}
	c.NamespaceDevice, c.NamespaceInode = uint64(st.Dev), uint64(st.Ino)
	if c.NativeStartOutput, c.NativeStartTool, err = nativeStart(ctx, pid); err != nil {
		return c, err
	}
	if c.CmdlineAfter, err = optional(readBounded(dir+"/cmdline", cmdlineLimit)); err != nil {
		return c, err
	}
	if c.ExecutableLinkAfter, err = optional(readLink(dir + "/exe")); err != nil {
		return c, err
	}
	if c.ExecutableAfterSHA256, c.ExecutableAfterBytes, err = digestBounded(dir+"/exe", executableLimit); err != nil {
		if !zombie {
			return c, err
		}
		c.ExecutableAfterSHA256, c.ExecutableAfterBytes = "", 0
	}
	if c.StatAfter, err = readBounded(dir+"/stat", statLimit); err != nil {
		return c, err
	}
	return c, nil
}

// SweepProcesses lists /proc in directory order and reads each numeric
// entry's stat once. A row whose stat cannot be read is a read failure, never
// an absent process.
func SweepProcesses(ctx context.Context, limit int) (Sweep, error) {
	var s Sweep
	var err error
	if s.BootID, err = readBounded(bootIDPath, 4096); err != nil {
		return s, err
	}
	if s.NamespaceLink, err = readLink(namespacePath); err != nil {
		return s, err
	}
	dir, err := os.Open("/proc")
	if err != nil {
		return s, fmt.Errorf("procfs sweep: %w", err)
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return s, fmt.Errorf("procfs sweep: %w", err)
	}
	s.PIDDirectory, s.Processes, s.ReadFailures = []byte{}, []SweptProcess{}, []string{}
	for _, name := range names {
		pid, ok := pidName(name)
		if !ok {
			continue
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}
		if len(s.Processes)+len(s.ReadFailures) >= limit {
			return s, errors.New("procfs sweep: process bound exceeded")
		}
		s.PIDDirectory = append(s.PIDDirectory, name+"\n"...)
		stat, err := readBounded("/proc/"+name+"/stat", statLimit)
		if err != nil {
			s.ReadFailures = append(s.ReadFailures, name+": stat unreadable")
			continue
		}
		s.Processes = append(s.Processes, SweptProcess{PID: pid, Stat: stat})
	}
	return s, nil
}

func pidName(name string) (int, bool) {
	if name == "" || len(name) > 10 || name[0] == '0' {
		return 0, false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return 0, false
		}
	}
	pid, err := strconv.Atoi(name)
	return pid, err == nil && pid > 0
}

func zombieStat(stat []byte) bool {
	closing := bytes.LastIndex(stat, []byte(") "))
	if closing < 0 || closing+2 >= len(stat) {
		return false
	}
	state := stat[closing+2]
	return state == 'Z' || state == 'X' || state == 'x'
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("procfs read: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("procfs read: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, errors.New("procfs read: " + path + " exceeds its bound")
	}
	return data, nil
}

func digestBounded(path string, limit int64) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("procfs read: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil {
		return "", 0, fmt.Errorf("procfs read: %w", err)
	}
	if n > limit {
		return "", 0, errors.New("procfs read: " + path + " exceeds its bound")
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func readLink(path string) ([]byte, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return nil, fmt.Errorf("procfs readlink: %w", err)
	}
	return []byte(link), nil
}

// nativeStart runs the host's native ps for the second-granularity lstart
// under a fixed C/UTC environment. A host without /bin/ps yields no native
// start, which the verifier treats as unavailable when a join needs it.
func nativeStart(ctx context.Context, pid int) ([]byte, []byte, error) {
	if _, err := os.Stat(nativeStartPath); errors.Is(err, os.ErrNotExist) {
		return []byte{}, []byte{}, nil
	}
	tool, err := readBounded(nativeStartPath, nativeToolLimit)
	if err != nil {
		return nil, nil, err
	}
	command := exec.CommandContext(ctx, nativeStartPath, "-o", "lstart=", "-p", strconv.Itoa(pid))
	command.Env = []string{"LC_ALL=C", "TZ=UTC"}
	output, err := command.Output()
	if err != nil || len(output) > nativeStartLimit {
		return nil, nil, errors.New("procfs capture: native start unavailable")
	}
	return output, tool, nil
}
