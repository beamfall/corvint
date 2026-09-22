//go:build !darwin && !linux

package main

import "os/exec"

// Unsupported hosts fail before starting a child, including full fallback.
func attachGroup(cmd *exec.Cmd) { cmd.Path = "/corvint-pr-tests-unsupported-platform" }
func killGroup(cmd *exec.Cmd)   {}
