//go:build !unix

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"
	"os"
)

func pollable(f *os.File) (*os.File, error) { return nil, errors.New("unsupported platform") }
