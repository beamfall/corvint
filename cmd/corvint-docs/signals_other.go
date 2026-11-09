// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import "os"

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
