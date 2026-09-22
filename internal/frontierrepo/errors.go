package frontierrepo

import (
	"errors"

	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

// coded republishes an upstream failure's EXPLICIT code in the one shape
// Frontier's CF-V0-022 translation reads: `interface{ Code() string }`.
//
// The producers this package composes each declare their code as a struct
// FIELD (`lrfrepo.Error.Code`, `tcq.Error.Code`), which cannot also be a
// method, so neither is visible to that translation and both would otherwise
// land on `frontier-internal-error` — a real code silently downgraded. Nothing
// is invented here: an error carrying no explicit code is returned unchanged,
// so it reaches the closed allowlists as the unknown outcome it is.
//
// Error() returns the code alone. CF-V0-024 forbids echoing an unverified
// path, OID, digest, ID, count, XML value, argv element, sibling canary, or
// exception, and `lrfrepo.Error.Error()` appends a formatted message that can
// carry exactly those. The cause stays reachable through Unwrap for a Go
// caller's debugger and, load-bearingly, so that a caught cancellation or
// deadline still satisfies `errors.Is` in Frontier's interruption check.
type coded struct {
	code  string
	cause error
}

func (err *coded) Error() string { return err.code }
func (err *coded) Code() string  { return err.code }
func (err *coded) Unwrap() error { return err.cause }

// translate normalizes one upstream failure. TCQ is consulted first because a
// `tcq.Error` re-raises inherited CEM/OCM codes under its own type, and the
// TCQ-V0-042 stage that raised it is the one Frontier must attribute it to.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var producer *tcq.Error
	if errors.As(err, &producer) {
		return &coded{code: producer.Code, cause: err}
	}
	if code := lrfrepo.CodeOf(err); code != "" {
		return &coded{code: code, cause: err}
	}
	return err
}
