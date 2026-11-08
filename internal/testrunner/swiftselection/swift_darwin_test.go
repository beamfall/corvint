// SPDX-License-Identifier: Apache-2.0

package swiftselection_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/proctrace"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func TestRSPV0001SwiftDarwinTraceClassesRemainFull(t *testing.T) {
	// The Darwin absence backend never starts a process, even for this empty
	// request. An empty trace is absence, not observed independence.
	trace := proctrace.New().Trace(context.Background(), proctrace.Request{})
	if trace.Backend != "absent" || trace.Outcome.ExitCode != -1 || trace.Reads.State != proctrace.Absent || trace.Listings.State != proctrace.Absent || trace.Lookups.State != proctrace.Absent || trace.Execs.State != proctrace.Absent || trace.External.State != proctrace.Absent || !slices.Contains(trace.Reads.Reasons, "darwin-mechanism-unqualified") {
		t.Fatalf("%+v", trace)
	}
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		o := swiftOptions(t, runner)
		p := swiftPlugin(t, o)
		in := swiftInventory(t, p, o)
		// Neither trace absence nor a caller's qualification boolean can admit a
		// class while this runner has no qualification piece/retained record.
		exclusions := []tr.Exclusion{}
		for _, test := range in.Tests {
			exclusions = append(exclusions, tr.Exclusion{ID: test.ID, Fresh: true})
		}
		plan := registry.NewSelectionSession(p, o.Key, true).Plan(in, exclusions, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != len(in.Tests) {
			t.Fatalf("%s: %+v", runner, plan)
		}
	}
}
