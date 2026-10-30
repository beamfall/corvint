//go:build !darwin && !linux

package testvaliditydoc

import "os"

// Receipt reads need O_NOFOLLOW directory pinning, which only darwin and linux
// qualify; elsewhere every open refuses with the typed platform refusal
// (PRS-V1-022).
var platformRefusal = ErrPlatformUnsupported

func openReceiptFile(parent *os.Root, name string) (*os.File, error) {
	return nil, ErrPlatformUnsupported
}

func openReceiptDirectory(parent *os.Root, name string) (*os.Root, error) {
	return nil, ErrPlatformUnsupported
}

func openTopDirectory(name string) (*os.Root, error) {
	return nil, ErrPlatformUnsupported
}
