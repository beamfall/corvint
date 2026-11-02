// Package cem2proto is an EXPERIMENTAL, unwired prototype of the proposed CEM
// 2.0 format (docs/specs/cem-2.0-intent.md, CEM2-V0-001..022). It encodes a
// canonical cem/1.0 map into a small header and a Git-derived index body, and
// projects that pair back to the byte-identical cem/1.0 map from Git.
//
// Nothing outside this package imports it, it uses the non-final label
// cem/2.0-proto, and its error codes are local, not registry entries. The
// intent is proposed, not accepted: this package is evidence for the owner's
// decision, never a delivered capability (CEM2-V0-001, CEM2-V0-016).
package cem2proto
