// SPDX-License-Identifier: Apache-2.0

// Package runnerqualification evaluates core-captured runner qualification
// experiments. It never admits its own output or enables production selection.
package runnerqualification

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const TrialFloor = 300
const MaxPopulation = 100000
const MaxSamples = 100000

// Identity includes every invalidator in RSP-V0-036. Configuration and semantic
// environment digests live in the existing T1 identities. NonSemanticDigest
// binds the declared non-semantic name list, not its excluded values.
type Identity struct {
	Runner                                                             testevidence.RunnerIdentity
	Execution                                                          testevidence.ExecutionIdentity
	SelectorVersion, RecordFormat, NonSemanticDigest, VerifierRevision string
}

type Corpus struct{ Repository, Commit string }

// Declaration is a claim, never evidence that the runner is qualified. Checkout
// is an operator argument to Run and is deliberately absent from the record.
type Declaration struct {
	Runner   string
	Corpus   Corpus
	Identity Identity
	Model    string
	Classes  []runnerselection.Class
	Mutation []langplugin.SubPlugin
}

// Edit operates on immutable repository-relative file bytes. Nil Before means
// absent and nil After means deletion; an empty, non-nil slice is an empty file.
type Edit struct {
	Path          string
	Before, After []byte
}
type Mutant struct {
	ID, Operator, Piece string // Piece is "core-file" or a declared mutation piece
	Class               runnerselection.Class
	Edits               []Edit
}
type Population struct {
	Complete bool
	Reason   string
	Mutants  []Mutant
}

// Merge pins the first parent (Base) and merged result (Head == Commit).
// The driver verifies ancestry against the named corpus before any observation.
type Merge struct {
	Commit, Base, Head string
	Classes            []runnerselection.Class
}

// Observation contains the actual FULL run and its retained shadow selection.
// It reuses T3 inventory/result types rather than introducing a second outcome
// model. Timings are explicitly observed; zero alone cannot prove measurement.
type Observation struct {
	Inventory                     testrunner.RunnerInventory
	Results                       []testrunner.RunnerResult
	Plan                          testrunner.RunSet
	InventoryWall, CollectionWall time.Duration
	TimingsObserved               bool
}

type Pair struct{ Traced, Untraced Observation }

// Experiment is evaluated in a fresh disposable copy of the pinned commit.
// A nil Mutant requests an unchanged baseline or negative control.
type Experiment struct {
	// Base binds a replay head's retained Plan to the exact Base..Commit diff.
	// It is empty for unchanged baselines, controls and explicit mutants.
	Base    string `json:",omitempty"`
	Commit  string
	Mutant  *Mutant
	Control runnerselection.Class
}

// Driver is core-owned, not a plugin-supplied verifier. Capture must actually
// run FULL, retaining selected identities before observing outcomes. Providers
// must compute a replay head's Plan for Experiment.Base..Experiment.Commit.
// The retained capture keeps that experiment beside the plan. Mutation providers
// supply complete populations through the declared mutation pieces. A capped
// population is incomplete, even if it contains useful candidates. The live
// driver owns checkout isolation, process containment and trusted-base inputs.
type Driver interface {
	Verify(context.Context, Declaration, string, []Merge) error
	Population(context.Context, Declaration, string, runnerselection.Class) (Population, error)
	Capture(context.Context, Declaration, string, Experiment) (Pair, error)
}

type Request struct {
	Declaration Declaration
	Checkout    string
	Merges      []Merge
	Seed        int64
	Samples     int
	Budget      time.Duration
}

type Miss struct {
	Experiment string
	Tests      []string
}
type ClassRecord struct {
	Class                                     runnerselection.Class
	PopulationDigest                          string
	Seed                                      int64
	PopulationSize, Sampled, Unkilled, Trials int
	Draws                                     []string
	SelectedSizes                             []int
	Misses                                    []Miss
	Upper95                                   float64
	ControlPassed, ReplayPassed, Passed       bool
	Reasons                                   []string
}
type Timing struct {
	Experiment, Model             string
	InventoryWall, CollectionWall time.Duration
}
type Record struct {
	Declaration  Declaration
	Merges       []Merge
	Classes      []ClassRecord
	Timings      []Timing
	Budget, Wall time.Duration
	Complete     bool
	Reasons      []string
}

// Admission is supplied only by the core's protected-ref evidence verifier.
// Sequence is the verifier's monotonic ordering, not a plugin timestamp.
type Admission struct {
	Record   Record
	Admitted bool
	Sequence uint64
}
type Job struct {
	Identity    Identity
	Declaration Declaration
	Budget      time.Duration
	Reason      string
}
type Plan struct {
	Mode, Status  string
	Reasons       []string
	Jobs          []Job
	Qualification runnerselection.Qualification
	// Sequence is the admission sequence of the exact record a qualified plan
	// accepted; zero unless Status is "qualified".
	Sequence uint64
}
