// SPDX-License-Identifier: Apache-2.0
//go:build !darwin && (!linux || !amd64)

// RSP-V0-020: an unsupported backend has no observed components.
package proctrace

import "context"

type platformBackend struct{}

func (platformBackend) Trace(_ context.Context, _ Request) Result {
	r := emptyResult("absent")
	r.unavailable(Absent, "unsupported-platform")
	return r
}
