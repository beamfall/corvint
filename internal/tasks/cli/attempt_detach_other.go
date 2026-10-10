//go:build !darwin && !linux

package cli

import (
	"os"
	"syscall"
)

// detachAvailable: detached attempt runs are Darwin and Linux only
// (ATR-V0-008); elsewhere they refuse UNSUPPORTED before any effect.
const detachAvailable = false

const readinessFD = 3

func detachAttr() *syscall.SysProcAttr { return nil }

func protectReadiness() {}

// openRunFile: detached runs are refused here before any run file exists.
func openRunFile(p string) (*os.File, error) { return os.Open(p) }

// openDirNonblock: elsewhere a directory to list is opened as os.Open does.
func openDirNonblock(p string) (*os.File, error) { return os.Open(p) }
