// SPDX-License-Identifier: Apache-2.0
package flows

import (
	"context"
	"errors"
	"io"
)

// refuseFlowsTestEvidence keeps `flows test-evidence` out of the docs product: the retained-evidence
// projection is the tests layer's (internal/testevidencedoc), which this layer does not import, so
// the verb stays native in corvint and this binary names where it runs (CMS-V0-914).
func refuseFlowsTestEvidence(context.Context, string, []string, io.Writer, io.Writer) error {
	return errors.New(TestEvidenceRefusal)
}

// TestEvidenceRefusal is the invalid-arguments message `corvint-docs flows test-evidence` exits 2 with.
const TestEvidenceRefusal = "flows test-evidence reads retained test evidence and runs in corvint, not corvint-docs; run corvint flows test-evidence"

// TestEvidenceHelp points the flows help at the one verb that stays native in corvint.
const TestEvidenceHelp = `
Retained test evidence linked to application flows (RSP-V0-051) is read by corvint, not this binary:
  corvint-docs [--root PATH] flows test-evidence ...
refuses with invalid-arguments (exit 2) and names corvint flows test-evidence; see corvint flows --help.
`

// refuseMovedReport keeps a flow report that joins flows with test results out of the docs
// product: `flows status`, `flows impact` and `flows gaps` run in corvint (internal/flowreports,
// PROD-DOCS-011, AFL-V1-001, AFL-V1-007), and this binary's verb is a notice naming that command,
// exit 2, nothing on stdout.
func refuseMovedReport(verb string) flowSubcommand {
	return func(context.Context, string, []string, io.Writer, io.Writer) error {
		return errors.New(MovedReportRefusal(verb))
	}
}

// MovedReportRefusal is the invalid-arguments message `corvint-docs flows VERB` exits 2 with for a
// report that moved to corvint.
func MovedReportRefusal(verb string) string {
	return "flows " + verb + " joins flows with test results and moved to corvint; run corvint flows " + verb
}

// MovedReports are the flow reports that moved to corvint, in help order.
var MovedReports = []string{"status", "impact", "gaps"}

// MovedReportSummary is the command-tree summary of a moved report's node, which its
// `corvint-docs flows VERB --help` prints (AFL-V1-007).
func MovedReportSummary(verb string) string {
	return "moved to corvint, where it joins flows with test results; this verb refuses with exit 2: run corvint flows " + verb
}

// MovedReportsHelp points the flows help at the reports that run in corvint.
const MovedReportsHelp = `
Flow reports that join flows with test results run in corvint, not this binary (AFL-V1):
  corvint-docs [--root PATH] flows status|impact|gaps ...
refuses with invalid-arguments (exit 2) and names corvint flows status, corvint flows impact or
corvint flows gaps; see corvint flows --help.
`
