// Package frontiernextrepo re-derives the experimental universe through the
// frozen frontier/0 admission, which admits no map since cem/0.x was retired
// (decision 0525, CEM-V1-210).
package frontiernextrepo

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/frontiernext"
	"github.com/Beamfall/corvint/internal/localauthority"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

type Adapter struct{ Root string }

func (a Adapter) Recompute(ctx context.Context, r frontiernext.Request, s frontiernext.Selection) (frontiernext.Universe, error) {
	return recompute(ctx, a.Root, nil, r, s)
}

type requestAdapter struct{ memo *gitauth.RequestReadMemo }

// NewRequestAdapter shares only immutable primitive successes between the
// canonical verifier and eligibility reader. The protected consumer creates the
// memo after its own view audit; this adapter never admits authority itself.
func NewRequestAdapter(memo *gitauth.RequestReadMemo) frontiernext.Recomputer {
	return requestAdapter{memo: memo}
}

func (a requestAdapter) Recompute(ctx context.Context, r frontiernext.Request, s frontiernext.Selection) (frontiernext.Universe, error) {
	if a.memo == nil {
		return frontiernext.Universe{}, localauthority.ErrInvalid
	}
	return recompute(ctx, "", a.memo, r, s)
}

// recompute admits the request's universe through the frozen frontier/0
// admission, which refuses every map since cem/0.x was retired (decision
// 0525): a cem/1.0 map and a retired cem/0.x map both refuse with
// unsupported-spec (CEM-V1-210), so no edge is ever evaluated. The edge
// evaluation that followed the admission was unreachable and is removed
// (CEM-V1-224).
func recompute(ctx context.Context, root string, memo *gitauth.RequestReadMemo, r frontiernext.Request, _ frontiernext.Selection) (frontiernext.Universe, error) {
	out := frontiernext.Universe{Eligible: map[frontiernext.Edge]bool{}, Claims: map[string][]string{}, Open: []frontiernext.Item{}}
	binding := r.Enrollment.Binding
	if memo == nil {
		_, err := lrfrepo.VerifyUniverse(ctx, root, r.CEM, r.OCM, binding.Base, binding.Target)
		return out, err
	}
	repository, err := memo.Open(gitrun.NewDefaultBudget())
	if err != nil {
		return out, err
	}
	_, err = lrfrepo.VerifyUniverseWithRepository(ctx, repository, r.CEM, r.OCM, binding.Base, binding.Target)
	return out, err
}
