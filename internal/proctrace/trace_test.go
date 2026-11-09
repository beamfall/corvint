// SPDX-License-Identifier: Apache-2.0
package proctrace

import (
	"context"
	"testing"
	"time"
)

func TestRSPV0020InvalidRequest(t *testing.T) {
	r := New().Trace(context.Background(), Request{})
	if r.Reads.State == Present || len(r.Reads.Reasons) == 0 {
		t.Fatalf("unsafe empty request: %+v", r)
	}
}
func TestRSPV0024Parity(t *testing.T) {
	r := emptyResult("fixture")
	r.Outcome = Outcome{Started: true, ExitCode: 0}
	r.Duration = 2 * time.Second
	q := Compare("fixture", r.Outcome, time.Second, &r)
	if !q.Enabled || q.Overhead != time.Second || r.Reads.State != Present {
		t.Fatalf("%+v %+v", q, r)
	}
}
func TestRSPV0024Mismatch(t *testing.T) {
	r := emptyResult("fixture")
	r.Outcome = Outcome{Started: true, ExitCode: 1}
	q := Compare("fixture", Outcome{Started: true, ExitCode: 0}, 0, &r)
	if q.Enabled || q.Reason != "outcome-mismatch" || r.Reads.State != Absent || r.External.State != Absent {
		t.Fatalf("%+v %+v", q, r)
	}
}

func TestRSPV0006Components(t *testing.T) {
	r := emptyResult("fixture")
	if r.Reads.State != Present || r.Listings.State != Present || r.Lookups.State != Present || r.Writes.State != Present || r.Execs.State != Present || r.External.State != Present {
		t.Fatal(r)
	}
	r.unavailable(Incomplete, "lost-observation")
	if r.Reads.State != Incomplete || r.Listings.State != Incomplete || r.Lookups.State != Incomplete || r.Writes.State != Incomplete || r.Execs.State != Incomplete || r.External.State != Incomplete {
		t.Fatal(r)
	}
}
func TestRSPV0024UnobservedParity(t *testing.T) {
	for _, outcome := range []Outcome{{ExitCode: 0}, {Started: true, ExitCode: 0, Error: "not observed"}} {
		r := emptyResult("fixture")
		r.Outcome = outcome
		if q := Compare("fixture", outcome, 0, &r); q.Enabled || r.Reads.State != Absent {
			t.Fatalf("unknown parity admitted: %+v", q)
		}
	}
	if q := Compare("fixture", Outcome{}, 0, nil); q.Enabled || q.Reason != "missing-trace" {
		t.Fatal(q)
	}
}
