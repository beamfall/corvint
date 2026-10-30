package cemcandidate

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/verify"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
)

type Result struct {
	Profile                           string                 `json:"profile"`
	CandidatePath                     string                 `json:"candidatePath"`
	CandidateSha256                   string                 `json:"candidateSha256"`
	Verification                      verify.CandidateResult `json:"verification"`
	DeclaredInputGitBinding           string                 `json:"declaredInputGitBinding"`
	TargetRevision                    string                 `json:"targetRevision"`
	InputInventorySha256              string                 `json:"inputInventorySha256"`
	NativeCaptureSemanticVerification string                 `json:"nativeCaptureSemanticVerification"`
	ExecutionAtCommit                 string                 `json:"executionAtCommit"`
}

// Assemble converted a canonically verified cem/0.2 source map into a candidate
// bundle. cem/0.x is retired (decision 0525 §1), so after the request and its
// pinned inputs are checked the source map is admitted only to be refused: a 0.x
// map with the retired-spec fix, a cem/1.0 map because it needs no assembly
// (CEM-V1-210). No bundle is written.
func Assemble(ctx context.Context, r Request, out string) (Result, error) {
	result := Result{Profile: "cem-candidate-assembly-result/0", TargetRevision: r.Target, NativeCaptureSemanticVerification: "NOT_OBSERVED", ExecutionAtCommit: "NOT_OBSERVED", DeclaredInputGitBinding: "NOT_OBSERVED"}
	if e := r.validate(); e != nil {
		return result, e
	}
	source, e := readSource(ctx, r.inputs())
	if e != nil {
		return result, e
	}
	return result, refuseSource(source)
}

// readSource reads every pinned input, so a changed or unsafe input still refuses
// first, and returns the source map bytes.
func readSource(ctx context.Context, inputs []FileRef) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	var source []byte
	for i, p := range inputs {
		b, e := readPinned(ctx, p)
		if e != nil {
			return nil, e
		}
		if i == 0 {
			source = b
		}
	}
	return source, nil
}

// refuseSource is the only source admission left: a cem/0.x map refuses with the
// retired-spec fix (CEM-V1-200) and a cem/1.0 map refuses as already stable.
func refuseSource(raw []byte) error {
	if _, e := cw.ParseStable(raw); e != nil {
		return e
	}
	return cemcode.New(cemcode.UnsupportedSpec, "candidate assembly converts only a cem/0.x source, which is retired; a cem/1.0 map needs no assembly")
}
