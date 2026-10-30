//go:build !unix

package main

import "os/exec"

// ownProcessGroup is a no-op where process groups are unavailable.
func ownProcessGroup(command *exec.Cmd) {}
