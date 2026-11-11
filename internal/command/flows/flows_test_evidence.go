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
