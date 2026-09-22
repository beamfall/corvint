// Package lrf implements the frozen Lexical Relevance Floor V0 projection.
package lrf

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	Profile        = "lrf/0"
	AuthorityClass = "producer-declared"
	OCMSpec        = "ocm/0.1-experimental"

	maxLexicalBytes  = 16_777_216
	maxTerms         = 256
	maxEdges         = 8_192
	maxResults       = 8_192
	maxIssues        = 8_192
	maxOutputBytes   = 4_194_304
	maxEvidenceBytes = 512
	maxEvidenceLines = 20
)

// Limits are the frozen deterministic evaluation bounds. Values may be
// tightened by internal conformance callers but never raised above DefaultLimits.
type Limits struct {
	LexicalBytes  int
	Terms         int
	Edges         int
	Results       int
	Issues        int
	OutputBytes   int
	EvidenceBytes int
	EvidenceLines int
}

// DefaultLimits returns a fresh copy of the frozen profile maxima.
func DefaultLimits() Limits {
	return Limits{
		LexicalBytes:  maxLexicalBytes,
		Terms:         maxTerms,
		Edges:         maxEdges,
		Results:       maxResults,
		Issues:        maxIssues,
		OutputBytes:   maxOutputBytes,
		EvidenceBytes: maxEvidenceBytes,
		EvidenceLines: maxEvidenceLines,
	}
}

// Context is the successful structural-verification envelope bound into every
// LRF result.
type Context struct {
	CEMSpec          string
	CEMMapSHA256     string
	PatchSource      string
	BaseRevision     string
	TargetRevision   *string
	PatchSHA256      string
	ExcludedPath     *string
	OCMSpec          *string
	OCMMapSHA256     *string
	IntentPath       *string
	IntentBlobOID    *string
	IntentStart      *int64
	IntentEnd        *int64
	IntentSpanSHA256 *string
}

func (c Context) inputs() [14]any {
	return [14]any{
		c.CEMSpec, c.CEMMapSHA256, c.PatchSource, c.BaseRevision,
		optionalString(c.TargetRevision), c.PatchSHA256, optionalString(c.ExcludedPath),
		optionalString(c.OCMSpec), optionalString(c.OCMMapSHA256), optionalString(c.IntentPath),
		optionalString(c.IntentBlobOID), optionalInt(c.IntentStart), optionalInt(c.IntentEnd),
		optionalString(c.IntentSpanSHA256),
	}
}

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// Evidence is a verified evidence identity paired with its exact pinned span.
type Evidence struct {
	ID   string
	Path string
	Span []byte
}

// Hunk is a verified patch hunk paired with its admitted added payload.
// Basis reuses the CEM wire relation type.
type Hunk struct {
	ID          string
	OldPath     *string
	NewPath     *string
	Added       []byte
	Disposition string
	Basis       []wire.Basis
	Ordinal     int
}

func (h Hunk) path() string {
	if h.NewPath != nil {
		return *h.NewPath
	}
	if h.OldPath != nil {
		return *h.OldPath
	}
	return ""
}

func (h Hunk) lexicalPath() string {
	if h.NewPath == nil {
		return ""
	}
	return *h.NewPath
}

// Obligation is one verified linked OCM obligation projection.
type Obligation struct {
	ID         string
	Statement  []byte
	HunkIDs    []string
	ClaimPaths []string
	Ordinal    int
}

// Request is an immutable post-structural projection; it performs no Git or
// network access.
type Request struct {
	Context     Context
	Hunks       []Hunk
	Evidence    []Evidence
	Obligations []Obligation
}

type ResultTuple [6]string
type IssueTuple [5]string

// Result is the complete canonical LRF result.
type Result struct {
	inputs        [14]any
	issues        []IssueTuple
	results       []ResultTuple
	boundExceeded bool
	canonical     []byte
}

func (r Result) ExitCode() int {
	if r.boundExceeded {
		return 1
	}
	return 0
}

// Inputs returns the fixed 14-slot structural authority tuple.
func (r Result) Inputs() [14]any { return r.inputs }

// Issues returns a copy of the canonical issue tuples.
func (r Result) Issues() []IssueTuple { return append([]IssueTuple(nil), r.issues...) }

// Results returns a copy of the canonical result tuples.
func (r Result) Results() []ResultTuple { return append([]ResultTuple(nil), r.results...) }

// BoundExceeded reports whether this is the single-issue aggregate bound document.
func (r Result) BoundExceeded() bool { return r.boundExceeded }

// CanonicalBytes returns a copy of the sealed canonical document. A zero,
// caller-constructed Result has no issued bytes.
func (r Result) CanonicalBytes() []byte { return append([]byte(nil), r.canonical...) }

// Error is a structural, compatibility, or operational failure for which no
// LRF result bytes may be emitted.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func invalid(message string) error {
	return &Error{Code: "invalid-lrf-request", Message: message}
}

func unsupported(message string) error {
	return &Error{Code: "unsupported-lrf-context", Message: message}
}
