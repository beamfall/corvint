// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

package pinnedread

import "os"

// Receipt reads need O_NOFOLLOW directory pinning, which only darwin and linux
// qualify; elsewhere every open refuses with the typed platform refusal
// (PRS-V1-022).
var platformRefusal = ErrPlatformUnsupported

func OpenFile(parent *os.Root, name string) (*os.File, error) {
	return nil, ErrPlatformUnsupported
}

func OpenDirectory(parent *os.Root, name string) (*os.Root, error) {
	return nil, ErrPlatformUnsupported
}

func OpenTopDirectory(name string) (*os.Root, error) {
	return nil, ErrPlatformUnsupported
}
