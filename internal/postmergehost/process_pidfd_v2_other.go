//go:build !(linux && (amd64 || arm64))

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import "syscall"

func openProcessV2(int) (int, error) { return -1, syscall.ENOSYS }

func signalProcessV2(int, syscall.Signal) error { return syscall.ENOSYS }

func stopProcessV2(int) error { return syscall.ENOSYS }

// closeProcessV2 has nothing to release: openProcessV2 never returns a handle here.
func closeProcessV2(int) {}
