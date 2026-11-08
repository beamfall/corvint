// SPDX-License-Identifier: Apache-2.0

// Package runnerselection computes read-only, language-neutral shadow plans.
// Its inputs are core-owned values, not a plugin protocol or an admission API.
// No result from this package authorizes execution or exclusion in production.
package runnerselection

import (
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

type Class string

const (
	BodyEdit        Class = "body-edit"
	DeclarationEdit Class = "declaration-edit"
	DataEdit        Class = "data-edit"
	Add             Class = "add"
	DeleteRename    Class = "delete-rename"
	ExecTarget      Class = "exec-target"
	RunnerConfig    Class = "runner-config"
	E2EServer       Class = "e2e-server"
)

// Hunk uses inclusive base lines. End == Start-1 is an insertion between
// Start and Start+1; both neighbours participate in intersection.
type Hunk struct{ Start, End int32 }

// Change is a fully decoded core diff entry. Renames carry both paths.
// Decoded=false explicitly prevents opaque tokens from proving non-matches.
type Change struct {
	Repository, Path, OldPath, Kind string // modify, add, delete, rename
	Decoded                         bool
	Hunks                           []Hunk
	ModeChanged, LinkChanged        bool
}

// Qualification is the core's admitted qualification view. Plugin declarations
// cannot supply it. This slice supports the traced model; other models widen.
type Qualification struct {
	Admitted, Passed, SamplingEnabled bool
	// SingleCollectionQualified is established by admitted core qualification,
	// never inferred from a fresh record or supplied by a plugin (RSP-V0-055).
	SingleCollectionQualified bool
	Model                     string
	Classes                   []Class
	Runner                    testevidence.RunnerIdentity
	Execution                 testevidence.ExecutionIdentity
}

type Evidence struct {
	Record   testevidence.Record
	Admitted bool
}

type Mandate struct{ Test, Reason string }

type Runner struct {
	Name                                               string
	Identity                                           testevidence.RunnerIdentity
	Execution                                          testevidence.ExecutionIdentity
	Pieces                                             testrunner.RunnerPieces
	Inventory                                          testrunner.RunnerInventory
	Qualification                                      Qualification
	Records                                            []Evidence
	ConfigurationPaths, ServerPaths                    []testevidence.PathKey
	Mandates                                           []Mandate
	PluginFailure                                      string
	InventoryDivergent, ConfigurationChanged, Disabled bool
	IndexBase                                          string
	IndexVerified                                      bool
}

// BaseDirectories is a complete base-side directory set, including each known
// repository root. It is needed only for additions. Snapshot is also base-side:
// comparing recorded blobs to head would discard the line-intersection proof.
type Input struct {
	Base, Head, Repository string
	Repositories           map[string]bool
	Changes                []Change
	DiffComplete           bool
	DiffReason             string
	BaseDirectories        map[testevidence.PathKey]bool
	DirectoriesComplete    bool
	Snapshot               testevidence.Snapshot
	Runners                []Runner
	LoadingNS              *int64
}

type PathClass struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Class      Class  `json:"class"`
}

type SelectedTest struct {
	Test     string   `json:"test"`
	Rules    []string `json:"rules"`
	Mandates []string `json:"mandates,omitempty"`
}

type ExcludedTest struct {
	Test         string      `json:"test"`
	RecordDigest string      `json:"record_digest"`
	Components   []string    `json:"components"`
	Changes      []PathClass `json:"changes"`
	Invalidation string      `json:"invalidation"`
}

type Counts struct {
	Inventoried    int  `json:"inventoried"`
	Selected       int  `json:"selected"`
	Coverage       int  `json:"coverage"`
	Fallback       int  `json:"fallback"`
	Excluded       int  `json:"excluded"`
	InventoryKnown bool `json:"inventory_known"`
}

type RunnerPlan struct {
	Runner         string         `json:"runner"`
	Mode           string         `json:"mode"`
	Reasons        []string       `json:"reasons"`
	ExecutionModel string         `json:"execution_model"`
	Changes        []PathClass    `json:"changes"`
	Selected       []SelectedTest `json:"selected"`
	Excluded       []ExcludedTest `json:"excluded"`
	Counts         Counts         `json:"counts"`
}

type Plan struct {
	Shadow        bool         `json:"shadow"`
	GoverningPlan string       `json:"governing_plan"`
	Soundness     string       `json:"soundness"`
	Runners       []RunnerPlan `json:"runners"`
	LoadingNS     *int64       `json:"loading_ns"`
	SelectionNS   *int64       `json:"selection_ns"`
}

const shadowClaim = "shadow only; no soundness claim before enforcement; governing static plans are outside this claim"
const invalidation = "any recorded blob OID or identity changes"

// MaxInputs bounds each core input collection. Exceeding it yields FULL.
const MaxInputs = testrunner.MaxTests
