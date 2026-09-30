//go:build !darwin && !linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/open_other.go at 25971bda1ca1664d8a546d751cb2bb9bbd454daf.
// Kept private to the Core observer to preserve decision 0397 component separation.
package safeopen

import (
	"errors"
)

const Supported = false

var unsupported = errors.New("platform has no qualified safe opening boundary")

func Root(string) (*Directory, error)                { return nil, unsupported }
func SubRoot(*Directory, string) (*Directory, error) { return nil, unsupported }
func InRoot(*Directory, string, bool) (*File, error) { return nil, unsupported }
