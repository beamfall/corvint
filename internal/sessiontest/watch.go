// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/testduration"
)

// ToolWatch is the tool a receipt names when `corvint watch` wrote it for one save's test run
// (RVD-V1-004).
const ToolWatch = "corvint watch"

// GoEvents is what one `go test -json` stream reported: each package's outcome and, for a
// package whose tests ran, its package-level Elapsed.
type GoEvents struct {
	packages map[string]string
	elapsed  map[string]float64
}

// ReadGoEvents reads a `go test -json` stream to its end with the parser the Go adapter of
// `corvint test` uses (PTD-V1-002, PTD-V1-003). out receives the output that adapter echoes.
func ReadGoEvents(stream io.Reader, out io.Writer) GoEvents {
	packages, elapsed := parseTestEventsTimed(stream, out)
	return GoEvents{packages: packages, elapsed: elapsed}
}

// WatchRun is one `go test -json` invocation of a `corvint watch` save.
type WatchRun struct {
	Argv []string
	// Package is the import path the invocation tested; Selected is set when a -run filter
	// narrowed it to some of its tests.
	Package             string
	Selected            bool
	Runner, Version     string
	Exit                int
	ElapsedMilliseconds int64
	Events              GoEvents
}

// WatchReceipt is the corvint-test-run/0 receipt of one `corvint watch` save's test run: the
// document `corvint test` writes, so the run evidence store keeps the same record shape for both
// (RVD-V1-004). source is the HEAD the save was planned against with the working-tree paths that
// differ from it. Each run is one go adapter invocation with one outcome for its package, outcome
// and time read exactly as `corvint test` reads them; a package the stream never ended is NOT_RUN.
// Scope is NARROWED and no manifest digest is named: a watch plan is not a `corvint test`
// manifest. goDir, GoUnitDir of the watched root, stamps each outcome's canonical testId as
// Runner.Execute does, so a watch record and a `corvint test` record name a package by the same
// ID (CTI-V1-009); a nil goDir stamps none.
func WatchReceipt(source SourceIdentity, runs []WatchRun, completed time.Time, goDir func(string) (string, bool)) Receipt {
	m := Manifest{
		Base: source.Head, Checks: []string{}, Incomplete: []string{}, Placement: PlacementUniverse,
		RunnerProfile: ProfileLocal, Scope: ScopeNarrowed, ShardCount: 1, Source: source,
	}
	if m.Source.Paths == nil {
		m.Source.Paths = []PathDigest{}
	}
	for _, run := range runs {
		slice := SliceWhole
		if run.Selected {
			slice = SliceSelected
		}
		m.Units = append(m.Units, Unit{Adapter: AdapterGo, Slice: slice, Unit: "go:" + run.Package})
	}
	if digest, err := Digest(m.Source); err == nil {
		m.SourceDigest = digest
	}
	receipt := NewReceipt(m, "", 0, ToolWatch)
	for index, run := range runs {
		unit := m.Units[index]
		outcome := Outcome{Outcome: OutcomeNotRun, Slice: unit.Slice, Unit: unit.Unit}
		if value, ok := run.Events.packages[run.Package]; ok {
			outcome.Outcome = value
			measure := testduration.Missing()
			if seconds, ok := run.Events.elapsed[run.Package]; ok {
				measure = testduration.FromSeconds(seconds, testduration.SourceGo)
			}
			outcome = timedOutcome(outcome, &measure)
		}
		receipt.Invocations = append(receipt.Invocations, Invocation{
			Adapter: AdapterGo, AdapterManifest: AdapterManifestProfile, Argv: nonNil(run.Argv),
			ElapsedMilliseconds: run.ElapsedMilliseconds, Env: []string{}, Exit: run.Exit,
			Outcomes: []Outcome{outcome}, Runner: run.Runner, Version: run.Version,
		})
	}
	receipt.CompletedAt = completed.UTC().Format(time.RFC3339Nano)
	stampTestIDs(receipt.Invocations, goDir)
	return Finish(receipt, m)
}
