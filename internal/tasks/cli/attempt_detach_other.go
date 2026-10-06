//go:build !darwin && !linux

package cli

import "syscall"

// detachAvailable: detached attempt runs are Darwin and Linux only
// (ATR-V0-008); elsewhere they refuse UNSUPPORTED before any effect.
const detachAvailable = false

const readinessFD = 3

func detachAttr() *syscall.SysProcAttr { return nil }

func protectReadiness() {}
