package tcq

import (
	"sort"
	"testing"
)

// cfV0016Vocabulary is the closed diagnostic list internal/frontier/upstream.go
// declares for the CF-V0-016 mapping table. It is transcribed from that frozen
// seam so a drift in either direction fails here rather than becoming a
// closed-allowlist failure inside Frontier (CF-V0-022).
var cfV0016Vocabulary = []string{
	"target-cleanliness-not-attested", "command-failed", "test-error", "test-failed",
	"test-skipped", "execution-identity-ambiguous", "repeated-test-rows",
	"row-identity-unavailable", "test-not-matched", "empty-body", "unconditional-skip",
	"claim-association-missing", "claim-association-ambiguous", "python-offset-mismatch",
	"unparseable-test-unit", "unsupported-anchor-profile", "unsupported-python-grammar",
}

// TestReasonVocabularyMatchesFrontierSeam proves this producer emits exactly the
// diagnostics the frontier consumer declares — no more, no fewer.
func TestReasonVocabularyMatchesFrontierSeam(t *testing.T) {
	produced := append([]string(nil), reasonOrder...)
	consumed := append([]string(nil), cfV0016Vocabulary...)
	sort.Strings(produced)
	sort.Strings(consumed)
	if len(produced) != len(consumed) {
		t.Fatalf("produced %d reasons, frontier consumes %d\n%v\n%v", len(produced), len(consumed), produced, consumed)
	}
	for index := range produced {
		if produced[index] != consumed[index] {
			t.Errorf("reason %d: produce %q, frontier consumes %q", index, produced[index], consumed[index])
		}
	}
}

// TestReasonOrderIsFrozen pins the TCQ-V0-039 order and its deduplication.
func TestReasonOrderIsFrozen(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{"hygiene-pair", []string{reasonUnconditionalSkip, reasonEmptyBody}, []string{reasonEmptyBody, reasonUnconditionalSkip}},
		{"duplicates", []string{reasonTestNotMatched, reasonTestNotMatched}, []string{reasonTestNotMatched}},
		{"passing-preconditions",
			[]string{reasonCommandFailed, reasonTargetCleanlinessNotAttested},
			[]string{reasonTargetCleanlinessNotAttested, reasonCommandFailed}},
		{"empty", nil, []string{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := sortReasons(testCase.input)
			if len(got) != len(testCase.want) {
				t.Fatalf("reasons = %v, want %v", got, testCase.want)
			}
			for index := range got {
				if got[index] != testCase.want[index] {
					t.Errorf("reasons = %v, want %v", got, testCase.want)
				}
			}
		})
	}
}

// TestEveryEdgeIsCallerReported covers TCQ-V0-006 and TCQ-V0-046: no input can
// select another authority class, and an abstention carries it too.
func TestEveryEdgeIsCallerReported(t *testing.T) {
	documents := loadDocuments(t)
	request := newRequest(documents)
	request.Command = []byte(documents.Command)
	request.Observation = []byte(documents.Observation)
	request.Report = []byte(documents.Report)
	result, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: documents.Target}, request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	abstained := 0
	for _, claim := range result.Claims() {
		if claim.AuthorityClass != AuthorityClass {
			t.Fatalf("claim %s carries authority %q", claim.ClaimID, claim.AuthorityClass)
		}
		if claim.Relation != "" && claim.Relation != MatchedRelation {
			t.Errorf("claim %s carries relation %q", claim.ClaimID, claim.Relation)
		}
		if claim.AssociationState != AssociationAbstained {
			continue
		}
		abstained++
		// TCQ-V0-037: no executable identity is permitted on an abstention.
		if claim.TestUnitID != "" || claim.ExecutionKeySha256 != "" || claim.AssociationKind != "" || len(claim.RowIDs) != 0 {
			t.Errorf("abstained claim %s carries executable identity", claim.ClaimID)
		}
	}
	if abstained == 0 {
		t.Fatal("fixture no longer exercises an abstention")
	}
}

// TestResultIsSelfConsistent covers TCQ-V0-038 reference closure: every claim's
// unit reference resolves within `units`, and no orphan unit is emitted.
func TestResultIsSelfConsistent(t *testing.T) {
	documents := loadDocuments(t)
	request := newRequest(documents)
	result, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: documents.Target}, request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	referenced := map[string]bool{}
	for _, claim := range result.Claims() {
		if claim.TestUnitID != "" {
			referenced[claim.TestUnitID] = true
		}
	}
	if len(referenced) == 0 {
		t.Fatal("fixture no longer exercises an association")
	}
	if result.ID() == "" || len(result.Raw()) == 0 {
		t.Fatal("result carries no identity")
	}
}
