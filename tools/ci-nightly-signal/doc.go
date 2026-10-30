// SPDX-License-Identifier: Apache-2.0

// Package nightlysignal holds the focused tests of the SOP-V0-013 nightly
// failure signal in .github/nightly-signal: the release-gates job that comments
// on, or opens, one nightly-triage issue when a scheduled run on main fails. The
// script is sh, gh and jq only; this package runs it against `gh run view --json
// jobs` fixtures recorded from real failed nightly runs, with a gh stand-in that
// records every write instead of posting it.
package nightlysignal
