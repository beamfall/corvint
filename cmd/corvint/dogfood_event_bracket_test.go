package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func dogfoodEventCLIArguments(root, event string) []string {
	return append([]string{"--root", root, "dogfood", "event"}, dogfoodEventArguments(event)...)
}

// TestDogfoodEventSpawnsOneBracket pins proposed LCP-V0-016 (V1-0881): a
// dogfood event spawns exactly four Git processes -- one identity-and-status
// observation before its read and one after -- whatever the event, with a
// snapshot hit read against the opening observation. Before this, stop spawned
// ten (two probes of two observations and an `ls-tree` nothing emitted) and an
// index-backed event thirteen (the loader's own pair and closing identity).
func TestDogfoodEventSpawnsOneBracket(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh for the git shim")
	}
	cases := map[string]struct {
		state string
		event string
		input string
	}{
		"stop":                         {"clean-snapshot-present", "stop", `{"stopHookActive":false}`},
		"user-prompt/snapshot-present": {"clean-snapshot-present", "user-prompt", `{"task":"revoke an expired device session"}`},
		"user-prompt/dirty-snapshot":   {"dirty-snapshot-present", "user-prompt", `{"task":"revoke an expired device session"}`},
		"startup-session-start":        {"clean-snapshot-present", "session-start", `{"startSource":"startup"}`},
		"compact-session-start/dirty":  {"dirty-snapshot-present", "session-start", `{"startSource":"compact"}`},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := ledgerRepository(t)
			sharedObservationStates[test.state](t, root)
			path, spawns := countingGit(t)
			stdout, stderr, code := runCandidateWithEnvironment(t, test.input, []string{path}, dogfoodEventCLIArguments(root, test.event)...)
			if code != 0 {
				t.Fatalf("%s exit %d: %s", test.event, code, stderr)
			}
			if got := spawns(); got != 4 {
				t.Fatalf("LCP-V0-016: %s spawned %d Git processes, want 4", test.event, got)
			}
			if !bytes.Contains(stdout, []byte(`"repository":{`)) {
				t.Fatalf("no repository block: %s", stdout)
			}
			if test.event != "stop" && !bytes.Contains(stdout, []byte(`"context":{`)) {
				t.Fatalf("no context packet: %s", stdout)
			}
		})
	}
}

// TestDogfoodEventRefusesRepositoryDriftInsideTheBracket pins the refusal the
// single bracket keeps (LCP-V0-016): a worktree that changes while the event
// reads it -- here between the index build of a snapshot miss and the closing
// observation -- is dogfood-event-repository-drift, and the native surface
// reports it as its fixed refusal with no envelope, never as an answer.
func TestDogfoodEventRefusesRepositoryDriftInsideTheBracket(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	drifting := func(ctx context.Context, root, task string) (*contextindex.Index, error) {
		index, err := contextindex.BuildContext(ctx, root, task)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, "drift.txt"), []byte("written while the event read\n"), 0o644); err != nil {
			return nil, err
		}
		return index, nil
	}
	// A minute, not the production deadline: this verifies the refusal, not latency (decision 0082).
	ctx := context.WithValue(context.Background(), dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	ctx = context.WithValue(ctx, dogfoodEventBuildKey{}, drifting)
	input := map[string]any{"startSource": "startup"}
	_, err := localEventRead(ctx, options{root: root, host: "codex", event: "session-start", budgetBytes: 8000}, input, dogfoodEventEnvelope, dogfoodEventContext, "")
	var kernelErr *gokernel.Error
	if !errors.As(err, &kernelErr) || kernelErr.Code != "dogfood-event-repository-drift" {
		t.Fatalf("drift inside the bracket: err = %v, want dogfood-event-repository-drift", err)
	}
	if err := os.Remove(filepath.Join(root, "drift.txt")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runLocalCompletionEvent(ctx, root, dogfoodEventArguments("session-start"), strings.NewReader(`{"startSource":"startup"}`), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `"dogfood-event-unavailable"`) || stdout.Len() != 0 {
		t.Fatalf("native surface answered through drift: exit %d, stdout %q, stderr %s", code, stdout.String(), &stderr)
	}
}
