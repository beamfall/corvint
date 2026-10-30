//go:build !darwin && !linux

package jstestprovider

import (
	"errors"
	"os"
)

// This experimental observer has no qualified nonblocking opener elsewhere.
func freshOpenDependency(string) (*os.File, error) {
	return nil, errors.New("dependency observation platform unsupported")
}
