// Package jstestprovider is an experimental IPR-08 provider slice: one JS/TS
// unit adapter (Vitest) and one E2E adapter (Playwright) that each produce a
// receipt binding test/config/app-build/environment identity to per-test
// execution outcomes, then feed those facts through
// internal/testvalidity.Project (see ../testvalidity/projection.go). This
// package carries no qualification authority: a receipt states what was
// observed, never a "valid" verdict.
package jstestprovider

import (
	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// ExecutionState is jstestreceipt.ExecutionState, kept under its old name.
type ExecutionState = jstestreceipt.ExecutionState

// StatePassed is jstestreceipt.StatePassed, kept under its old name.
const StatePassed = jstestreceipt.StatePassed

// StateFailed is jstestreceipt.StateFailed, kept under its old name.
const StateFailed = jstestreceipt.StateFailed

// StateSkipped is jstestreceipt.StateSkipped, kept under its old name.
const StateSkipped = jstestreceipt.StateSkipped

// StateFlaky is jstestreceipt.StateFlaky, kept under its old name.
const StateFlaky = jstestreceipt.StateFlaky

// StateTimedOut is jstestreceipt.StateTimedOut, kept under its old name.
const StateTimedOut = jstestreceipt.StateTimedOut

// StateInterrupted is jstestreceipt.StateInterrupted, kept under its old name.
const StateInterrupted = jstestreceipt.StateInterrupted

// StateInfrastructure is jstestreceipt.StateInfrastructure, kept under its old name.
const StateInfrastructure = jstestreceipt.StateInfrastructure

// Anchor is jstestreceipt.Anchor, kept under its old name.
type Anchor = jstestreceipt.Anchor

// FailureArtifact is jstestreceipt.FailureArtifact, kept under its old name.
type FailureArtifact = jstestreceipt.FailureArtifact

// TestOutcome is jstestreceipt.TestOutcome, kept under its old name.
type TestOutcome = jstestreceipt.TestOutcome

// AttemptDetail is jstestreceipt.AttemptDetail, kept under its old name.
type AttemptDetail = jstestreceipt.AttemptDetail

// BrowserStep is jstestreceipt.BrowserStep, kept under its old name.
type BrowserStep = jstestreceipt.BrowserStep

// SensitiveInputPolicy is jstestreceipt.SensitiveInputPolicy, kept under its old name.
type SensitiveInputPolicy = jstestreceipt.SensitiveInputPolicy

// InfrastructureFailure is jstestreceipt.InfrastructureFailure, kept under its old name.
type InfrastructureFailure = jstestreceipt.InfrastructureFailure

// Identity is jstestreceipt.Identity, kept under its old name.
type Identity = jstestreceipt.Identity

// AppBuildIdentity is jstestreceipt.AppBuildIdentity, kept under its old name.
type AppBuildIdentity = jstestreceipt.AppBuildIdentity

// Receipt is jstestreceipt.Receipt, kept under its old name.
type Receipt = jstestreceipt.Receipt

// ExecutionSchedule is jstestreceipt.ExecutionSchedule, kept under its old name.
type ExecutionSchedule = jstestreceipt.ExecutionSchedule

// ExecutionStart is jstestreceipt.ExecutionStart, kept under its old name.
type ExecutionStart = jstestreceipt.ExecutionStart

// ProjectIdentity is jstestreceipt.ProjectIdentity, kept under its old name.
type ProjectIdentity = jstestreceipt.ProjectIdentity

// Attempt is jstestreceipt.Attempt, kept under its old name.
type Attempt = jstestreceipt.Attempt

// ExternalLifecycle is jstestreceipt.ExternalLifecycle, kept under its old name.
type ExternalLifecycle = jstestreceipt.ExternalLifecycle

// RunnerProcess is jstestreceipt.RunnerProcess, kept under its old name.
type RunnerProcess = jstestreceipt.RunnerProcess

// ProjectReporters is jstestreceipt.ProjectReporters, kept under its old name.
type ProjectReporters = jstestreceipt.ProjectReporters

// ProjectReporter is jstestreceipt.ProjectReporter, kept under its old name.
type ProjectReporter = jstestreceipt.ProjectReporter

// ApplicationAttestationReceipt is jstestreceipt.ApplicationAttestationReceipt, kept under its old name.
type ApplicationAttestationReceipt = jstestreceipt.ApplicationAttestationReceipt

// ApplicationAttestationProviderIdentity is jstestreceipt.ApplicationAttestationProviderIdentity, kept under its old name.
type ApplicationAttestationProviderIdentity = jstestreceipt.ApplicationAttestationProviderIdentity

// ApplicationAttestationExpectation is jstestreceipt.ApplicationAttestationExpectation, kept under its old name.
type ApplicationAttestationExpectation = jstestreceipt.ApplicationAttestationExpectation

// ApplicationRepositoryExpectation is jstestreceipt.ApplicationRepositoryExpectation, kept under its old name.
type ApplicationRepositoryExpectation = jstestreceipt.ApplicationRepositoryExpectation

// ApplicationAttestationObservation is jstestreceipt.ApplicationAttestationObservation, kept under its old name.
type ApplicationAttestationObservation = jstestreceipt.ApplicationAttestationObservation

// ApplicationAttestation is jstestreceipt.ApplicationAttestation, kept under its old name.
type ApplicationAttestation = jstestreceipt.ApplicationAttestation

// ApplicationRepositoryIdentity is jstestreceipt.ApplicationRepositoryIdentity, kept under its old name.
type ApplicationRepositoryIdentity = jstestreceipt.ApplicationRepositoryIdentity

// ApplicationArtifactIdentity is jstestreceipt.ApplicationArtifactIdentity, kept under its old name.
type ApplicationArtifactIdentity = jstestreceipt.ApplicationArtifactIdentity

// ApplicationInstanceIdentity is jstestreceipt.ApplicationInstanceIdentity, kept under its old name.
type ApplicationInstanceIdentity = jstestreceipt.ApplicationInstanceIdentity

// ApplicationHealth is jstestreceipt.ApplicationHealth, kept under its old name.
type ApplicationHealth = jstestreceipt.ApplicationHealth
