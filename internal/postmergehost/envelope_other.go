//go:build !linux

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import "io"

// RunGuestEnvelope fails closed outside the qualified Linux guest role.
func RunGuestEnvelope(_ string, _ io.Reader, _ io.Writer) error { return ErrHostInput }
