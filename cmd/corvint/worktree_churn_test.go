package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// LCP-V0-008, LCP-V0-016 (V1-0880, decision 0533): a dogfood event whose repository read was
// refused because the worktree moved reports the one bounded `dogfood-event-worktree-churn` code;
// policy drift keeps its own code, and every other read failure stays the fixed
// `dogfood-event-unavailable`. The read is injected through the dogfoodEventReadKey seam.
func TestV10880WorktreeChurnHasNamedCode(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	unstable := &gokernel.Error{Code: "repository-state-unstable", Message: "repository dirty-path set changed while probing repository state"}
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"probe unstable", unstable, "dogfood-event-worktree-churn"},
		{"wrapped probe unstable", fmt.Errorf("observe: %w", unstable), "dogfood-event-worktree-churn"},
		{"bracket repository drift", dogfoodEventError("dogfood-event-repository-drift"), "dogfood-event-worktree-churn"},
		{"index context drift", dogfoodEventError("dogfood-event-context-drift"), "dogfood-event-worktree-churn"},
		{"policy drift", dogfoodEventError("dogfood-event-policy-drift"), "dogfood-event-policy-drift"},
		{"repository unavailable", &gokernel.Error{Code: "repository-unavailable", Message: "Git failed"}, "dogfood-event-unavailable"},
		{"unlisted error", errors.New("unlisted read failure"), "dogfood-event-unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stderr := churnEventStderr(t, root, test.err)
			if !strings.Contains(stderr, `"code": "`+test.want+`"`) {
				t.Fatalf("refusal %s, want code %s", stderr, test.want)
			}
		})
	}
}

// V1-0880: the host adapter carries the churn code into its degradation reason, and an unlisted
// read failure still reaches it as `dogfood-event-unavailable`.
func TestV10880AdapterReasonNamesChurnAndKeepsUnavailable(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	for err, want := range map[error]string{
		&gokernel.Error{Code: "repository-state-unstable", Message: "repository HEAD changed while probing repository state"}: "corvint-event-rejected:dogfood-event-worktree-churn",
		errors.New("unlisted read failure"): "corvint-event-rejected:dogfood-event-unavailable",
	} {
		ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
		ctx = context.WithValue(ctx, dogfoodEventReadKey{}, func(context.Context, options, map[string]any) (map[string]any, error) {
			return nil, err
		})
		output := runCodexAdapter(ctx, map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": root, "prompt": "inspect requirement"})
		if reason := adapterDegradationReason(output); reason != want {
			t.Fatalf("%v: reason %q, want %q (%+v)", err, reason, want, output)
		}
	}
}

func churnEventStderr(t *testing.T, root string, readErr error) string {
	t.Helper()
	// The minute is a hang detector: the injected read returns at once (decision 0082).
	ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	ctx = context.WithValue(ctx, dogfoodEventReadKey{}, func(context.Context, options, map[string]any) (map[string]any, error) {
		return nil, readErr
	})
	var stdout, stderr bytes.Buffer
	if status := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{}`), &stdout, &stderr); status != 2 || stdout.Len() != 0 {
		t.Fatalf("status %d stdout %s", status, &stdout)
	}
	return stderr.String()
}

// LCP-V0-034 (V1-1976): a prompt the bounded prompt-context contract refuses reports the one
// named `dogfood-event-prompt-bounds` code instead of the uncaused `dogfood-event-unavailable`;
// every other contextindex refusal keeps the fixed unavailable code.
func TestV11976PromptContractRefusalHasNamedCode(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{"anchor bounds", &contextindex.Error{Code: "unsupported-dogfood-context-bounds", Message: "dogfood context exceeds the bounded anchor profile"}, "dogfood-event-prompt-bounds"},
		{"wrapped budget", fmt.Errorf("prompt: %w", &contextindex.Error{Code: "unsupported-dogfood-context-budget", Message: "budget"}), "dogfood-event-prompt-bounds"},
		{"invalid context", &contextindex.Error{Code: "invalid-dogfood-context", Message: "invalid"}, "dogfood-event-prompt-bounds"},
		{"other contextindex refusal", &contextindex.Error{Code: "index-unavailable", Message: "missing"}, "dogfood-event-unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stderr := churnEventStderr(t, root, test.err)
			if !strings.Contains(stderr, `"code": "`+test.want+`"`) {
				t.Fatalf("refusal %s, want code %s", stderr, test.want)
			}
		})
	}
}
