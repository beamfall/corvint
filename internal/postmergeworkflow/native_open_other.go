//go:build !darwin && !linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/open_other.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
package postmergeworkflow

import (
	"errors"
	"os"
)

var nativeOpenUnsupported = errors.New("platform has no qualified safe opening boundary")

func nativeOpenControl(*os.File, func(uintptr) error) error { return nativeOpenUnsupported }
func nativeOpenRoot(string) (*os.Root, error)               { return nil, nativeOpenUnsupported }
func nativeOpenSubRoot(*os.Root, string) (*os.Root, error)  { return nil, nativeOpenUnsupported }
func nativeOpenInRoot(*os.Root, string, int, os.FileMode, bool) (*os.File, error) {
	return nil, nativeOpenUnsupported
}
func nativeOpenFile(string) (*os.File, error) { return nil, nativeOpenUnsupported }
