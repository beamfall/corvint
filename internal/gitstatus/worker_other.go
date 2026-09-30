//go:build !darwin && !linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package gitstatus

import "errors"

func verifyWorkerGroup() error { return errors.New("worker platform unsupported") }
