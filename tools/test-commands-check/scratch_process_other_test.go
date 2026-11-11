// SPDX-License-Identifier: Apache-2.0
//go:build !linux && !darwin

package main

import (
	"os/exec"
	"testing"
)

func guardScratchBash(t *testing.T, c *exec.Cmd) {}
