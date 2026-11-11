// SPDX-License-Identifier: Apache-2.0

package analyzerrust

import "github.com/Beamfall/corvint/internal/rustsyntax"

// Range, Body and Unsupported are the Rust lexer's body shapes (internal/rustsyntax), kept
// under the analyzer's names.
type (
	Range       = rustsyntax.Range
	Body        = rustsyntax.Body
	Unsupported = rustsyntax.Unsupported
)

// ExecutableBodies extracts function-body interiors from the Rust dialect's bounded lexer
// (rustsyntax.ExecutableBodies, RSP-V0-005).
func ExecutableBodies(source []byte) ([]Body, error) { return rustsyntax.ExecutableBodies(source) }

// isIdentStart and isIdentContinue are the lexer's ASCII identifier classes (RUST_IDENT).
func isIdentStart(c byte) bool    { return rustsyntax.IdentStart(c) }
func isIdentContinue(c byte) bool { return rustsyntax.IdentContinue(c) }

// indexAnyByte is the first index in b of any byte in set, or -1.
func indexAnyByte(b []byte, set string) int { return rustsyntax.IndexAnyByte(b, set) }
