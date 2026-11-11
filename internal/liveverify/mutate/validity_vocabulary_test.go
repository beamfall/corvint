// SPDX-License-Identifier: Apache-2.0
package mutate

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The shared test-validity projection mirrors this package's KILLED and
// SURVIVED verdicts and its witness shape instead of importing the runner;
// the two must stay equal.
func TestVerdictsMatchTheTestValidityStrengthVocabulary(t *testing.T) {
	if string(Killed) != testvalidity.StrengthKilled || string(Survived) != testvalidity.StrengthSurvived {
		t.Fatalf("verdicts %q/%q, testvalidity %q/%q", Killed, Survived, testvalidity.StrengthKilled, testvalidity.StrengthSurvived)
	}
	witness := Witness{Operator: "op", Start: 1, End: 2, KillingTest: "TestX"}
	mirrored := testvalidity.MutationWitness(witness)
	if mirrored.Operator != witness.Operator || mirrored.Start != witness.Start || mirrored.End != witness.End || mirrored.KillingTest != witness.KillingTest {
		t.Fatalf("witness %+v mirrored as %+v", witness, mirrored)
	}
}
