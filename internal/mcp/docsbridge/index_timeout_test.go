package docsbridge

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestSDD011IndexDeadlineHasDistinctCode injects the source index's own deadline refusal (the
// exact error contextindex returns when its Git deadline expires) on both docs tools while the
// caller's context stays live: the refusal is a bounded overrun and must not read as a missing
// repository (SDD-V0-011, V1-1432). A Git read failure without that cause keeps
// repository-unavailable.
func TestSDD011IndexDeadlineHasDistinctCode(t *testing.T) {
	registry, err := New(docsRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		refusal error
		want    string
	}{
		{&contextindex.Error{Message: "Git repository index exceeded its deadline", Cause: context.DeadlineExceeded}, "repository-index-timeout"},
		{&contextindex.Error{Message: "Git object read failed"}, "repository-unavailable"},
	} {
		registry.build = func(context.Context, string, string) (*contextindex.Index, error) { return nil, tc.refusal }
		for name, arguments := range map[string][]byte{
			ToolDraft:   draftArguments(t, "owner.md", "cache"),
			ToolConsume: consumeArguments(t, "owner.md", "cache", "split keys", "draft"),
		} {
			structured, text, toolFailure, transportErr := registry.Call(context.Background(), name, arguments)
			if transportErr == nil || transportErr.Code != tc.want {
				t.Fatalf("%s %q = transport %#v, tool %#v; want code %s", name, tc.refusal, transportErr, toolFailure, tc.want)
			}
			if structured != nil || text != "" || toolFailure != nil {
				t.Fatalf("%s %q leaked a payload: structured=%#v text=%q tool=%#v", name, tc.refusal, structured, text, toolFailure)
			}
		}
	}
}
