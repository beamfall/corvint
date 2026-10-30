//go:build !darwin && !linux

// SPDX-License-Identifier: Apache-2.0
package gitstatus

import "errors"

func verifyWorkerGroup() error { return errors.New("worker platform unsupported") }
