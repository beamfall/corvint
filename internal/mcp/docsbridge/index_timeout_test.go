package docsbridge

import (
	"context"
	"testing"
	"time"

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

// TestSDD011CallerEndDuringBuildStaysCancelled ends the caller's context inside the build seam,
// by cancellation and by the caller's own deadline, and returns the index's typed deadline
// refusal: the caller's end takes precedence, so both tools report cancelled, not
// repository-index-timeout.
func TestSDD011CallerEndDuringBuildStaysCancelled(t *testing.T) {
	registry, err := New(docsRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	deadlineRefusal := &contextindex.Error{Message: "Git repository index exceeded its deadline", Cause: context.DeadlineExceeded}
	for name, arguments := range map[string][]byte{
		ToolDraft:   draftArguments(t, "owner.md", "cache"),
		ToolConsume: consumeArguments(t, "owner.md", "cache", "split keys", "draft"),
	} {
		for _, ending := range []string{"cancel", "deadline"} {
			ctx, cancel := context.WithCancel(context.Background())
			if ending == "deadline" {
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			}
			registry.build = func(buildCtx context.Context, _, _ string) (*contextindex.Index, error) {
				if ending == "cancel" {
					cancel()
				}
				<-buildCtx.Done()
				return nil, deadlineRefusal
			}
			structured, text, toolFailure, transportErr := registry.Call(ctx, name, arguments)
			cancel()
			if transportErr == nil || transportErr.Code != "cancelled" {
				t.Fatalf("%s caller %s during build = transport %#v, tool %#v; want cancelled", name, ending, transportErr, toolFailure)
			}
			if structured != nil || text != "" || toolFailure != nil {
				t.Fatalf("%s caller %s leaked a payload: structured=%#v text=%q tool=%#v", name, ending, structured, text, toolFailure)
			}
		}
	}
}
