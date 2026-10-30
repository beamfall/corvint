package testvalidity

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// vectorFile mirrors conformance/test-validity-v0/vectors.json, the shared
// vector set the Go projector and the VS Code TypeScript mirror
// (extensions/vscode/src/testvalidity.ts) both run (roadmap IPR-06).
type vectorFile struct {
	Cases []vectorCase `json:"cases"`
}

type vectorCase struct {
	Name     string      `json:"name"`
	Input    vectorInput `json:"input"`
	Expected Projection  `json:"expected"`
}

type vectorInput struct {
	Claim     *ClaimFacts     `json:"claim"`
	Execution *ExecutionFacts `json:"execution"`
	Mutation  *vectorMutation `json:"mutation"`
}

// vectorMutation matches MutationFacts field for field but keeps its own
// type so the vector's plain "verdict"/"detail" strings decode without
// requiring mutate.Verdict's underlying type.
type vectorMutation struct {
	Verdict string          `json:"verdict"`
	Detail  string          `json:"detail"`
	Witness *mutate.Witness `json:"witness"`
}

func (input vectorInput) toInput() Input {
	var mutation *MutationFacts
	if input.Mutation != nil {
		mutation = &MutationFacts{
			Verdict: input.Mutation.Verdict,
			Detail:  input.Mutation.Detail,
			Witness: input.Mutation.Witness,
		}
	}
	return Input{Claim: input.Claim, Execution: input.Execution, Mutation: mutation}
}

func TestProjectMatchesSharedVectors(t *testing.T) {
	raw, err := os.ReadFile("../../conformance/test-validity-v0/vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var file vectorFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("decode vectors: %v", err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("vectors.json has no cases")
	}
	for _, testCase := range file.Cases {
		t.Run(testCase.Name, func(t *testing.T) {
			actual := Project(testCase.Input.toInput())
			if !reflect.DeepEqual(actual, testCase.Expected) {
				t.Fatalf("Project() = %#v, want %#v", actual, testCase.Expected)
			}
		})
	}
}

// TestSharedVectorsCoverRequiredCases pins the minimum case set LPCV-V0-050
// and LPCV-V0-052 name (docs/specs/live-proof-carrying-verification-v0.md:297). The vector
// runs above iterate whatever vectors.json carries, so deleting a required
// case would leave both projectors green while the mandated coverage is
// gone; this asserts the named cases are still there.
func TestSharedVectorsCoverRequiredCases(t *testing.T) {
	required := []string{
		"empty-test",
		"always-skipped-test",
		"wrong-target",
		"stale-execution",
		"unmatched-report",
		"passing-reported-result",
		"unsupported-input",
		"surviving-mutation",
		"killed-mutation",
		"skipped-reported-result",
		"skipped-with-cause-abstains",
	}
	raw, err := os.ReadFile("../../conformance/test-validity-v0/vectors.json")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var file vectorFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("decode vectors: %v", err)
	}
	present := map[string]bool{}
	for _, testCase := range file.Cases {
		present[testCase.Name] = true
	}
	for _, name := range required {
		if !present[name] {
			t.Errorf("vectors.json is missing the LPCV-V0-050/052 required case %q", name)
		}
	}
}
