//go:build !darwin && !linux && !freebsd

package main

import "os/exec"

func prepareContainedCommand(_ *exec.Cmd) bool   { return false }
func signalContainedCommand(_ *exec.Cmd, _ bool) {}
