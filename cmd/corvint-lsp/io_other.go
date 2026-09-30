//go:build !unix

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"errors"
	"os"
)

func pollable(f *os.File) (*os.File, error) { return nil, errors.New("unsupported platform") }
