// SPDX-License-Identifier: Apache-2.0
package tracerecordrepo

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// DCW-V0-7403: preflight uses exact admitted source paths, not raw diff size,
// and never enters either trace mutation entry point.
func TestDCWV07403DogfoodAdmissionReadOnly(t *testing.T) {
	oldOpen, oldRecover := openTraceStore, recoverTraceAppend
	openTraceStore = func(string, map[string]trace.Revision, func() error) (*trace.Store, error) {
		t.Fatal("preflight opened trace store")
		return nil, nil
	}
	recoverTraceAppend = func(string, func() error, func() error) error {
		t.Fatal("preflight entered append recovery")
		return nil
	}
	t.Cleanup(func() { openTraceStore, recoverTraceAppend = oldOpen, oldRecover })
	for _, tc := range []struct {
		name          string
		source, other int
		want          bool
	}{
		{"at-limit", 200, 0, false}, {"over-limit", 201, 0, true}, {"non-source", 0, 201, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newAdapterFixture(t)
			base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			for i := 0; i < tc.source; i++ {
				writeAdapterFixture(t, root, fmt.Sprintf("source/f%03d.go", i), "package source\n")
			}
			for i := 0; i < tc.other; i++ {
				writeAdapterFixture(t, root, fmt.Sprintf("records/f%03d.csv", i), "not an index source\n")
			}
			gitAdapterFixture(t, root, "add", ".")
			gitAdapterFixture(t, root, "commit", "-qm", "admission boundary")
			target := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			before := gitAdapterFixture(t, root, "status", "--porcelain")
			got, err := DogfoodNeedsAggregate(context.Background(), root, base, target)
			if err != nil || got != tc.want {
				t.Fatalf("aggregate=%t want=%t err=%v", got, tc.want, err)
			}
			if after := gitAdapterFixture(t, root, "status", "--porcelain"); after != before {
				t.Fatalf("preflight mutated repository: before=%q after=%q", before, after)
			}
		})
	}
}
