// Package testvaliditydoc builds the corvint-test-validity/0 document
// (docs/specs/live-proof-carrying-verification-v0.md LPCV-V0-051): the shared
// five-axis projection of every test-level result a JavaScript receipt or Go
// preview-session event carries. Projections are recomputed from observations,
// never trusted from the input. The corvint test-validity verb and the
// experimental MCP test-validity profile both call this package.
package testvaliditydoc

import (
	"github.com/Beamfall/corvint/internal/testvaliditymodel"
)

// Schema is testvaliditymodel.Schema, kept under its old name.
const Schema = testvaliditymodel.Schema

// MaxInputBytes is testvaliditymodel.MaxInputBytes, kept under its old name.
const MaxInputBytes = testvaliditymodel.MaxInputBytes

// Document is testvaliditymodel.Document, kept under its old name.
type Document = testvaliditymodel.Document

// Test is testvaliditymodel.Test, kept under its old name.
type Test = testvaliditymodel.Test

// Input is testvaliditymodel.Input, kept under its old name.
type Input = testvaliditymodel.Input

// Unsupported is testvaliditymodel.Unsupported, kept under its old name.
func Unsupported() Document { return testvaliditymodel.Unsupported() }

// Decode is testvaliditymodel.Decode, kept under its old name.
func Decode(data []byte) (Input, error) { return testvaliditymodel.Decode(data) }

// Project is testvaliditymodel.Project, kept under its old name.
func Project(input Input) Document { return testvaliditymodel.Project(input) }

// ProjectRouteStubs is Project with the routeStubs member requested (LPCV-V0-077): every test of a
// JavaScript receipt carries it, so a receipt without any producer record, or a profiled one, states
// NOT_PRODUCED with an explicit reason instead of omitting the member. Project keeps the omission
// and every legacy byte.
func ProjectRouteStubs(input Input) Document {
	if input.JS != nil {
		return testvaliditymodel.ProjectJavaScript(*input.JS, true)
	}
	return Project(input)
}
