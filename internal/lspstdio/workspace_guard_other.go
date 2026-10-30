//go:build !darwin && !linux

// SPDX-License-Identifier: Apache-2.0
package lspstdio

import "os"

func openWorkspaceFile(string) (*os.File, error) { return nil, errWorkspaceObservation }
