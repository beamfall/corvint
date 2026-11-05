//go:build !linux

// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
)

func saveNotifications(context.Context, string) (<-chan struct{}, error) {
	return nil, errors.New("native save notifications unavailable: use -events-stdin or -once")
}
