//go:build !darwin && !linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import "os"

func openWorkspaceFile(string) (*os.File, error) { return nil, errWorkspaceObservation }
