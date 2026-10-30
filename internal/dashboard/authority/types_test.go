package authority

import "context"

type fakeAuthority struct{}

func (fakeAuthority) Snapshot() Snapshot { return Snapshot{} }

func (fakeAuthority) QualifyTrace(context.Context, string, []string) TraceQualification {
	return TraceQualification{Code: TraceQualified}
}

func (fakeAuthority) Finish(context.Context) FinishResult {
	return FinishResult{Code: FinishStable}
}

var _ Authority = fakeAuthority{}
var _ TraceQualifier = fakeAuthority{}
