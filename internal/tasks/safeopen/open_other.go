//go:build !darwin && !linux

package safeopen

import (
	"errors"
	"os"
)

const Supported = false

var unsupported = errors.New("platform has no qualified safe opening boundary")

func Control(*os.File, func(uintptr) error) error                       { return unsupported }
func Root(string) (*os.Root, error)                                     { return nil, unsupported }
func SubRoot(*os.Root, string) (*os.Root, error)                        { return nil, unsupported }
func InRoot(*os.Root, string, int, os.FileMode, bool) (*os.File, error) { return nil, unsupported }
func File(string) (*os.File, error)                                     { return nil, unsupported }
func PinDir(*os.Root) (*os.File, error)                                 { return nil, unsupported }
func InDir(*os.File, string, int, os.FileMode) (*os.File, error)        { return nil, unsupported }
func PinSubDir(*os.Root, string) (*os.File, error)                      { return nil, unsupported }
func RootOf(*os.File) (*os.Root, error)                                 { return nil, unsupported }
