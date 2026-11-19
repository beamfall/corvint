// SPDX-License-Identifier: Apache-2.0
//go:build !darwin && !linux

package flows

import "os"

func terminalInput(*os.File) bool { return false }
