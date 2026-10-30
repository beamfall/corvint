//go:build !darwin && !linux

package authoritystore

import "os"

func openBindingMetadata(string) (*os.File, error) { return nil, errUnavailable }
