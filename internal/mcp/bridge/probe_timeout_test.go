package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// TestMCPV0038ProbeTimeoutHasDistinctCode injects a gokernel probe overrun through the
// WithProbeBound seam: the caller's context stays live, so the refusal is the probe's own
// deadline and must not read as a missing repository (MCPV0-038, V1-1427).
func TestMCPV0038ProbeTimeoutHasDistinctCode(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := gokernel.WithProbeBound(context.Background(), time.Nanosecond)
	_, callErr := registry.Call(ctx, ToolStatus, []byte(`{}`))
	if callErr == nil || callErr.Code != "repository-probe-timeout" {
		t.Fatalf("probe overrun = %#v, want code repository-probe-timeout", callErr)
	}
	if callErr.ReasonClass != "" {
		t.Fatalf("probe overrun reasonClass = %q, want untyped (unclassified on the wire)", callErr.ReasonClass)
	}
	if ctx.Err() != nil {
		t.Fatal("the caller context expired; the refusal would be cancellation, not a probe overrun")
	}
}
