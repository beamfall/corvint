//go:build !darwin && !linux

package authoritystore

import "os"

func openBindingDirectory(string) (*os.File, error) { return nil, errUnavailable }
