// SPDX-License-Identifier: Apache-2.0

//go:build windows

package browsercov

import (
	"errors"
	"os/exec"
	"syscall"
)

// unsupportedPlatform refuses Windows: a process under test cannot be asked
// to write its coverage before a runner kills it (LPCV-V0-978).
func unsupportedPlatform() error {
	return errors.New("browsercov: " + refusals)
}

func setGroup(*exec.Cmd) {}

func signalGroup(int, syscall.Signal) error { return errors.ErrUnsupported }

func signalPid(int, syscall.Signal) error { return errors.ErrUnsupported }

func signalUsr2(int) error { return errors.ErrUnsupported }

func alive(int) bool { return false }

func groupAlive(int) bool { return false }

const sigTerm, sigKill = syscall.Signal(15), syscall.Signal(9)
