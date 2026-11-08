// SPDX-License-Identifier: Apache-2.0
//go:build !linux || !amd64

package proctrace

import (
	"context"
	"testing"
)

func TestRSPV0020AbsentBackend(t *testing.T) {
	r := New().Trace(context.Background(), Request{})
	if r.Backend != "absent" || r.Reads.State != Absent || r.External.State != Absent || len(r.Reads.Reasons) == 0 || r.Outcome.Started {
		t.Fatalf("unsupported platform fabricated a trace: %+v", r)
	}
}
