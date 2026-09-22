//go:build !unix

package main

import "os/exec"

// setProcessGroup and killProcessGroup are unreachable on this platform:
// runProcess returns "posix-required" before starting a command here.
func setProcessGroup(*exec.Cmd) {}

func killProcessGroup(*exec.Cmd) error { return nil }

// processAlive is unreachable on this platform: runProcess returns
// "posix-required" before starting a command here.
func processAlive(int) bool { return false }
