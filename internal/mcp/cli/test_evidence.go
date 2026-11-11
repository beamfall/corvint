// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"context"

	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

// parseTestEvidence supplies the corvint.test_evidence tool group its query
// parse and read: the same projection as `corvint test-evidence`.
func parseTestEvidence(argv []string) (toolgroup.TestEvidenceRead, error) {
	q, _, err := testevidencedoc.Parse(argv)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, root string) any { return testevidencedoc.Read(ctx, root, q) }, nil
}
