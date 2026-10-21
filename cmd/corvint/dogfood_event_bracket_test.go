package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

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
			stdout, stderr, code := runCandidateWithEnvironment(t, test.input, []string{path, helperLiftedEventBounds + "=1"}, dogfoodEventCLIArguments(root, test.event)...)
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
	// Lifted event deadline and Git probe bound: this verifies the refusal, not latency; host load
	// expired the probe's 10-second bound first (V1-0969, decision 0082).
	ctx := context.WithValue(lifecycleDeadlineContext(), dogfoodEventBuildKey{}, drifting)
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
	if code != 2 || !strings.Contains(stderr.String(), `"dogfood-event-worktree-churn"`) || stdout.Len() != 0 {
		t.Fatalf("native surface answered through drift: exit %d, stdout %q, stderr %s", code, stdout.String(), &stderr)
	}
}

// TestDogfoodEventPrefetchesTheSnapshotFromTheOpeningIdentity pins proposed LCP-V0-033
// (V1-1524, V1-2144): a context-reading dogfood event reads the snapshot once, from the
// opening identity alone (no dirty set or status digest, so before that observation's status
// read finished), and answers exactly the bytes the sequential read answers. Not parallel: it
// wraps the package-level loader seam.
func TestDogfoodEventPrefetchesTheSnapshotFromTheOpeningIdentity(t *testing.T) {
	root := ledgerRepository(t)
	sharedObservationStates["dirty-snapshot-present"](t, root)
	var mutex sync.Mutex
	var calls []contextindex.Observation
	loader := loadSnapshotObserved
	loadSnapshotObserved = func(root string, observation contextindex.Observation) (*contextindex.Index, bool, error) {
		mutex.Lock()
		calls = append(calls, observation)
		mutex.Unlock()
		return loader(root, observation)
	}
	t.Cleanup(func() { loadSnapshotObserved = loader })
	for _, test := range []struct {
		event string
		input map[string]any
	}{
		{"user-prompt", map[string]any{"task": "revoke an expired device session"}},
		{"session-start", map[string]any{"startSource": "compact"}},
	} {
		t.Run(test.event, func(t *testing.T) {
			eventOptions := options{root: root, host: "claude-code", event: test.event, budgetBytes: 8000}
			calls = nil
			sequential, err := localEventRead(lifecycleDeadlineContext(), eventOptions, test.input, dogfoodEventEnvelope, dogfoodEventContext, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 || calls[0].StatusSHA256 == "" {
				t.Fatalf("sequential read: %d loads %+v, want one against the complete observation", len(calls), calls)
			}
			calls = nil
			prefetched, err := dogfoodEvent(lifecycleDeadlineContext(), eventOptions, test.input)
			if err != nil {
				t.Fatal(err)
			}
			mutex.Lock()
			defer mutex.Unlock()
			if len(calls) != 1 || calls[0].Revision == "" || calls[0].StatusSHA256 != "" || calls[0].DirtyPaths != nil {
				t.Fatalf("LCP-V0-033: %d loads %+v, want one against the opening identity alone", len(calls), calls)
			}
			want, _ := json.Marshal(sequential)
			got, _ := json.Marshal(prefetched)
			if !bytes.Equal(got, want) {
				t.Fatalf("LCP-V0-033: prefetched answer differs\n got %s\nwant %s", got, want)
			}
		})
	}
}

// TestSnapshotPrefetchServesOnlyItsOwnTree pins the LCP-V0-033 fallback: a prefetch the
// bracket never started, or one that read another object format or tree than the opening
// observation names, is not taken, so the event reads the snapshot sequentially as before.
func TestSnapshotPrefetchServesOnlyItsOwnTree(t *testing.T) {
	t.Parallel()
	observation := contextindex.Observation{ObjectFormat: "sha1", CommitRevision: strings.Repeat("c", 40), Revision: strings.Repeat("a", 40), StatusSHA256: "digest"}
	unstarted := &snapshotPrefetch{done: make(chan struct{})}
	if _, _, _, ok := unstarted.take("root", observation); ok {
		t.Fatal("an unstarted prefetch was taken")
	}
	for name, prefetch := range map[string]*snapshotPrefetch{
		"other tree":   {objectFormat: "sha1", tree: strings.Repeat("b", 40), done: make(chan struct{})},
		"other format": {objectFormat: "sha256", tree: strings.Repeat("a", 40), done: make(chan struct{})},
	} {
		if _, _, _, ok := prefetch.take("root", observation); ok {
			t.Fatalf("%s: a prefetch of another snapshot was taken", name)
		}
	}
	missed := &snapshotPrefetch{objectFormat: "sha1", tree: strings.Repeat("a", 40), done: make(chan struct{})}
	close(missed.done)
	if index, hit, err, ok := missed.take("root", observation); !ok || hit || index != nil || err != nil {
		t.Fatalf("a prefetched miss: index %v hit %v err %v ok %v, want a taken miss", index, hit, err, ok)
	}
}

// TestAdoptObservationMatchesTheObservedLoader pins the LCP-V0-033 adoption against
// contextindex.LoadSnapshotObserved: an identity-only hit that then adopts the complete
// observation carries the same root, sorted unique dirty set, status digest and HEAD as the
// loader's own hit for that observation.
func TestAdoptObservationMatchesTheObservedLoader(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	sharedObservationStates["dirty-snapshot-present"](t, root)
	output, err := exec.Command("git", "-C", root, "rev-parse", "--show-object-format", "HEAD", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	identity := strings.Fields(string(output))
	observation := contextindex.Observation{ObjectFormat: identity[0], CommitRevision: identity[1], Revision: identity[2],
		DirtyPaths: []string{"notes.txt", "internal/auth/session.go", "notes.txt"}, StatusSHA256: strings.Repeat("d", 64)}
	want, hit, err := loadSnapshotObserved(root, observation)
	if err != nil || !hit {
		t.Fatalf("observed loader: hit %v err %v", hit, err)
	}
	got, hit, err := loadSnapshotObserved(root, contextindex.Observation{ObjectFormat: identity[0], CommitRevision: identity[1], Revision: identity[2]})
	if err != nil || !hit {
		t.Fatalf("identity-only loader: hit %v err %v", hit, err)
	}
	adoptObservation(got, root, observation)
	if got.Root != want.Root || got.StatusSHA256 != want.StatusSHA256 || got.CommitRevision != want.CommitRevision || !slices.Equal(got.DirtyPaths, want.DirtyPaths) {
		t.Fatalf("LCP-V0-033 adoption differs: got %q %q %q %q, want %q %q %q %q", got.Root, got.StatusSHA256, got.CommitRevision, got.DirtyPaths, want.Root, want.StatusSHA256, want.CommitRevision, want.DirtyPaths)
	}
}
