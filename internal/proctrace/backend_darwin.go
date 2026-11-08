// SPDX-License-Identifier: Apache-2.0

// RSP-V0-021, RSP-V0-022 and RSP-V0-023: no macOS mechanism has qualified
// under the unprivileged whole-tree contract. See the mechanism decision in
// docs/build-log/2026-11-08-rsp-macos-tracing.md before enabling observations.
package proctrace

import "context"

type platformBackend struct{}

func (platformBackend) Trace(_ context.Context, _ Request) Result {
	// Do not launch an unobserved command or infer completeness from an empty
	// event stream. Absence applies even to malformed or cancelled requests.
	r := emptyResult("absent")
	r.unavailable(Absent, "unsupported-platform")
	r.unavailable(Absent, "darwin-mechanism-unqualified")
	return r
}
