//go:build linux

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// RunGuestEnvelope is a fixed internal role, entered only by the pinned optional
// launcher. The caller owns timeout and process retirement. It publishes one
// observation before the barrier; author output is never another protocol frame.
func RunGuestEnvelope(authorSHA string, in io.Reader, out io.Writer) error {
	if !hostSHA.MatchString(authorSHA) || os.Getuid() != 65532 || os.Geteuid() != 65532 || os.Getgid() != 65532 {
		return ErrHostInput
	}
	cwd, err := os.Getwd()
	if err != nil || cwd != "/product" {
		return ErrHostInput
	}
	info, err := os.Lstat("/tools/author")
	if err != nil {
		return ErrHostInput
	}
	if _, err = readHostFile(HostFileRef{"/tools/author", authorSHA, info.Size()}, 16<<20); err != nil {
		return err
	}
	proc, err := os.Open("/proc/1/environ")
	if err != nil {
		return ErrHostInput
	}
	raw, err := io.ReadAll(io.LimitReader(proc, HostWireLimit+1))
	closeErr := proc.Close()
	if err != nil || closeErr != nil || len(raw) > HostWireLimit || len(raw) == 0 || raw[len(raw)-1] != 0 {
		return ErrHostInput
	}
	hold, err := hostObserveEnvironment(strings.Split(string(raw[:len(raw)-1]), "\x00"))
	if err != nil {
		return err
	}
	// No inherited image/daemon key survives into the observed author envelope.
	os.Clearenv()
	for _, v := range hostExpectedEnvironment() {
		if os.Setenv(v.Key, v.Value) != nil {
			return ErrHostInput
		}
	}
	env := os.Environ()
	observed, err := hostObserveEnvironment(env)
	if err != nil || !hostEnvironmentMatches(observed, hostExpectedEnvironment()) {
		return ErrHostInput
	}
	for _, path := range []string{"/private/home", "/private/tmp"} {
		if err := os.Mkdir(path, 0700); err != nil {
			return ErrHostInput
		}
	}
	record := HostEnvelopeObservation{"corvint-postmerge-internal-envelope/0", authorSHA, observed, hold}
	if json.NewEncoder(out).Encode(record) != nil {
		return ErrHostInput
	}
	line, err := bufio.NewReader(io.LimitReader(in, 9)).ReadString('\n')
	if err != nil || line != "execute\n" {
		return ErrHostInput
	}
	// Close the barrier and all non-log descriptors. The author receives /dev/null
	// as stdin and precisely the environment that was observed before the barrier.
	nullFD, err := syscall.Open("/dev/null", syscall.O_RDONLY, 0)
	if err != nil {
		return ErrHostInput
	}
	// Dup3 exists on every Linux port; Dup2 is absent on linux/arm64.
	if nullFD != 0 && syscall.Dup3(nullFD, 0, 0) != nil {
		_ = syscall.Close(nullFD)
		return ErrHostInput
	}
	if nullFD != 0 {
		_ = syscall.Close(nullFD)
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return ErrHostInput
	}
	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil {
			return ErrHostInput
		}
		if fd > 2 {
			syscall.CloseOnExec(fd)
		}
	}
	return syscall.Exec("/tools/author", []string{"/tools/author", "/admitted/author-input.json", "/product"}, env)
}
