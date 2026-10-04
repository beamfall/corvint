//go:build !darwin && !linux

package cli

import (
	"errors"
	"os"
)

func openStableMap(string) (*os.File, error) { return nil, errors.New("stable platform unsupported") }
