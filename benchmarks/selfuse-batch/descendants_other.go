//go:build !darwin && !linux

package main

import (
	"context"
	"errors"
)

type descendantCleanup struct {
	cleaned, leaked []int
	status          string
	failure         error
}

func newDescendantCleanup() *descendantCleanup {
	return &descendantCleanup{cleaned: []int{}, leaked: []int{}, status: "NOT_OBSERVED: platform-unsupported"}
}
func (d *descendantCleanup) stop(context.Context, int) error {
	d.failure = errors.New("descendant observation unavailable on this platform")
	return d.failure
}
func (d *descendantCleanup) start(context.Context, int) error { return d.stop(context.Background(), 0) }
