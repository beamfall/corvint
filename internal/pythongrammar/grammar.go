// Package pythongrammar holds the closed Python 3.12 subset lexer and parser.
// It is deliberately neutral: it carries no request envelope, no fact-output
// budget, and no dependency on the separately buildable Python analyzer, so
// Core may reach the grammar without reaching that candidate (PNC-001).
package pythongrammar

// Failure is a typed rejection reason. The values match the analyzer's wire
// vocabulary so a caller can pass them straight through.
type Failure string

// failure is the spelling the moved lexer and parser use throughout.
type failure = Failure

const (
	failureMalformed   Failure = "MALFORMED_INPUT"
	failureUnsupported Failure = "UNSUPPORTED_SCHEMA"
	failureDynamic     Failure = "DYNAMIC_INPUT"
	failureLimit       Failure = "LIMIT_EXCEEDED"
)

// maxPythonDepth bounds nesting so a hostile source cannot drive unbounded
// parser recursion.
const maxPythonDepth = 256
