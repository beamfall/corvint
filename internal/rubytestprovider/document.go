// Package rubytestprovider is the experimental RSpec provider behind
// corvint-ruby-test-provider (docs/specs/live-proof-carrying-verification-v0.md
// LPCV-V0-090..094, LPCV-V0-131..138). It runs RSpec with its JSON formatter, binds the run to the
// host-observed test repository identity before and after, and records each
// example's file:line identity, full description, status and the Git blob IDs
// of its spec file and the conventional helper files. The document carries
// observations only; testvaliditydoc recomputes every projection from them.
package rubytestprovider

import (
	"github.com/Beamfall/corvint/internal/rubytestreceipt"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Profile is rubytestreceipt.Profile, kept under its old name.
const Profile = rubytestreceipt.Profile

// Producer is rubytestreceipt.Producer, kept under its old name.
const Producer = rubytestreceipt.Producer

// HelperBasis is rubytestreceipt.HelperBasis, kept under its old name.
const HelperBasis = rubytestreceipt.HelperBasis

// MaxExamples is rubytestreceipt.MaxExamples, kept under its old name.
const MaxExamples = rubytestreceipt.MaxExamples

// MaxSnapshotFiles is rubytestreceipt.MaxSnapshotFiles, kept under its old name.
const MaxSnapshotFiles = rubytestreceipt.MaxSnapshotFiles

// MaxDescriptionBytes is rubytestreceipt.MaxDescriptionBytes, kept under its old name.
const MaxDescriptionBytes = rubytestreceipt.MaxDescriptionBytes

// Document is rubytestreceipt.Document, kept under its old name.
type Document = rubytestreceipt.Document

// SelectorOutcome is rubytestreceipt.SelectorOutcome, kept under its old name.
type SelectorOutcome = rubytestreceipt.SelectorOutcome

// Runtime is rubytestreceipt.Runtime, kept under its old name.
type Runtime = rubytestreceipt.Runtime

// RuntimeFact is rubytestreceipt.RuntimeFact, kept under its old name.
type RuntimeFact = rubytestreceipt.RuntimeFact

// SharedExample is rubytestreceipt.SharedExample, kept under its old name.
type SharedExample = rubytestreceipt.SharedExample

// BoundFile is rubytestreceipt.BoundFile, kept under its old name.
type BoundFile = rubytestreceipt.BoundFile

// Example is rubytestreceipt.Example, kept under its old name.
type Example = rubytestreceipt.Example

// Summary is rubytestreceipt.Summary, kept under its old name.
type Summary = rubytestreceipt.Summary

// JUnit is rubytestreceipt.JUnit, kept under its old name.
type JUnit = rubytestreceipt.JUnit

// Infrastructure is rubytestreceipt.Infrastructure, kept under its old name.
type Infrastructure = rubytestreceipt.Infrastructure

// Validate is rubytestreceipt.Validate, kept under its old name.
func Validate(document Document) error { return rubytestreceipt.Validate(document) }

// BlobID is rubytestreceipt.BlobID, kept under its old name.
func BlobID(format string, data []byte) string { return rubytestreceipt.BlobID(format, data) }

// HelperSetDigest is rubytestreceipt.HelperSetDigest, kept under its old name.
func HelperSetDigest(helpers []BoundFile) string { return rubytestreceipt.HelperSetDigest(helpers) }

// ProjectExample is rubytestreceipt.ProjectExample, kept under its old name.
func ProjectExample(document Document, example Example) testvalidity.Projection {
	return rubytestreceipt.ProjectExample(document, example)
}

// ProjectRun is rubytestreceipt.ProjectRun, kept under its old name.
func ProjectRun(document Document) testvalidity.Projection {
	return rubytestreceipt.ProjectRun(document)
}

// Freshness is rubytestreceipt.Freshness, kept under its old name.
func Freshness(document Document, read func(name string) ([]byte, error), helpers func(projectDirectory string) ([]string, error)) testvalidity.Axis {
	return rubytestreceipt.Freshness(document, read, helpers)
}
