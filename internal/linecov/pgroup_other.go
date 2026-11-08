//go:build !unix

// SPDX-License-Identifier: Apache-2.0

package linecov

import "os/exec"

// isolate leaves cmd in the collector's process group where Unix groups do
// not exist.
func isolate(cmd *exec.Cmd) {}

func cleanup(cmd *exec.Cmd) bool { return false }
