// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import "context"

// VerifyRawLogicalForTestV2 exposes the non-minting raw verifier to the
// external host collector test. It returns the logical graph only and never
// a VerifiedProcessesV2 token.
func VerifyRawLogicalForTestV2(ctx context.Context, policy ProcessPolicyV2, policySHA string, expected GraphBindingV2,
	proof []byte, store ArtifactReaderV2) ([]LogicalProcessV2, error) {
	result, err := verifyRawProcessV2(ctx, policy, policySHA, expected, proof, store)
	return result.logical, err
}

// ExitFactsForTestV2 exposes the retirement exit-fact derivation.
func ExitFactsForTestV2(c ProcCaptureV2) (string, error) { return exitFactsV2(&c) }
