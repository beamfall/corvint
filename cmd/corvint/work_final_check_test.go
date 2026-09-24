//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// The real bounded adapter runs snapshot/details/first verify unchanged. Only
// the mandatory second verify uses the injected response, in owned HOME scratch.
func workFinalFixture(t *testing.T, final string) string {
	t.Helper()
	root := workProductionFixture(t)
	path := filepath.Join(root, "script/corvint-work-queue")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line, body, _ := strings.Cut(string(raw), "\n")
	final = strings.ReplaceAll(final, "@ROOT@", root)
	script := line + "\n" + workSharedCompilerPrelude(t) + "\nwork_fixture_body() {\n" + body + "\n}\n" + `
if [ "$1" != verify ]; then work_fixture_body "$@"; exit $?; fi
if [ -f "$HOME/first-checkpoint" ]; then
` + final + `
exit $?
fi
work_fixture_body "$@" > "$HOME/first-checkpoint" || exit $?
cat "$HOME/first-checkpoint"
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	materializationGit(t, root, "add", ".")
	materializationGit(t, root, "commit", "-qm", "final checkpoint fixture")
	workFinalIncompleteScope(t, root)
	return root
}

// workFinalIncompleteScope places an untracked FIFO inside the qualified
// source's Git metadata directory (one of observeWork's monitoredRoots,
// cmd/corvint/work.go) so the WQO-V0-017 manifest scan reports an
// unsupported entry there (workManifestEntry, work_mutation.go) on every
// capture this fixture drives. .git is outside what the caller-tree
// qualification check inspects, so this keeps monitoredComplete false and so
// keeps every final-check fixture's initial (and closing) scope incomplete,
// independent of whether the worklist-adapter mapping byte-reproduces
// (WQO-V0-046, decision 0348). Without it, once decision-0046-v0 qualifies,
// ProposeWave's full StateValidated path runs against this fixture's minimal
// worklist/envelope instead of the closed-scope path these tests exercise.
func workFinalIncompleteScope(t *testing.T, root string) {
	t.Helper()
	if err := syscall.Mkfifo(filepath.Join(root, ".git", "unsupported-manifest-entry"), 0600); err != nil {
		t.Fatal(err)
	}
}

func workFinalEnvelope(t *testing.T) (*workqueue.CapacityEnvelope, string) {
	t.Helper()
	envelope := &workqueue.CapacityEnvelope{Available: []workqueue.CapacityClass{}, Capabilities: []string{}, Profile: workqueue.EnvelopeProfile, RepositoryAuthorityID: "repo:corvint"}
	workqueue.RefreshEnvelope(envelope)
	path := filepath.Join(t.TempDir(), "capacity.json")
	if err := os.WriteFile(path, envelope.Canonical(), 0600); err != nil {
		t.Fatal(err)
	}
	return envelope, path
}

// WQO-V0-025/032: a failed final operation is not evidence of checkpoint drift.
func TestWorkFinalCheckCommandFailures(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-032", func(t *testing.T) {
		cases := []struct{ name, script, code string }{
			{"nonzero", "exit 7", "ADAPTER_FAILED"},
			{"malformed", "printf '{'", "MALFORMED_INPUT"},
			{"hostile", "sed 's/\"version\":\"[^\"]*\"/\"version\":\"\u202e\"/' \"$HOME/first-checkpoint\"", "HOSTILE_INPUT"},
			{"overflow", "dd if=/dev/zero bs=65537 count=1 2>/dev/null", "INPUT_LIMIT"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				workCaptureSlot(t)
				root := workFinalFixture(t, tc.script)
				_, envelope := workFinalEnvelope(t)
				// runWork maps any deadline, its own hang detector included, onto
				// INPUT_LIMIT, so a parent deadline surfaces here as the very
				// code the overflow case expects. These cases assert an adapter
				// outcome, never a deadline; the cancelled and deadline subtests
				// below are where a short context is the behaviour under test.
				var output, stderr bytes.Buffer
				exit := runWork(context.Background(), root, []string{"propose-wave", "--envelope", envelope, "--limit", "1"}, &output, &stderr)
				workAssertFinalError(t, output.Bytes(), exit, tc.code)
				if stderr.Len() != 0 {
					t.Fatalf("unexpected stderr: %s", &stderr)
				}
			})
		}
	})
}

func workFinalCapture(t *testing.T, script string) (string, *workCapture) {
	t.Helper()
	root := workFinalFixture(t, script)
	// A parent deadline here would retire the setup capture before the
	// behaviour under test ever runs, and reports the overrun as INPUT_LIMIT.
	// Every other observeWork caller passes Background (work_observe_test.go).
	capture, code := observeWork(context.Background(), root, io.Discard)
	if code != "" {
		t.Fatalf("initial capture: %s", code)
	}
	t.Cleanup(capture.Close)
	if capture.observation.State != workqueue.StateUnknown || capture.observation.MutationState != "UNKNOWN" {
		t.Fatalf("initial capture did not retain incomplete scope: %+v", capture.observation)
	}
	return root, capture
}

func workFinalPriorMutation(capture *workCapture) {
	capture.observation.MutationState = "CHANGED"
	capture.observation.Unknowns = append(capture.observation.Unknowns, workqueue.UnknownMutationDetected)
	workqueue.RefreshObservation(capture.observation)
}

func workAssertFinalProposal(t *testing.T, capture *workCapture, envelope *workqueue.CapacityEnvelope, output []byte, exit int, observationState, proposalState string) {
	t.Helper()
	if exit != 0 || capture.observation.State != observationState {
		t.Fatalf("exit=%d adapter failure=%v failed receipt=%+v observation=%+v output=%s", exit, capture.runner.failure, capture.runner.failureReceipt, capture.observation, output)
	}
	result := workFinalResult(t, output)
	var proposal map[string]json.RawMessage
	if err := json.Unmarshal(result["proposal"], &proposal); err != nil {
		t.Fatal(err)
	}
	if string(proposal["state"]) != `"`+proposalState+`"` || string(proposal["observationId"]) != `"`+capture.observation.ID+`"` || string(proposal["entries"]) != "[]" || string(proposal["mutationAuthority"]) != "false" {
		t.Fatalf("proposal lost final state/binding/empty selection: %s", result["proposal"])
	}
	if capture.snapshot.ObservationID != capture.observation.ID || capture.snapshot.ObservationState != capture.observation.State || !reflect.DeepEqual(capture.snapshot.ObservationUnknowns, capture.observation.Unknowns) {
		t.Fatal("snapshot binding did not follow final observation")
	}
	want, err := workqueue.ProposeWave(capture.snapshot, envelope, capture.closure, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposalState == "STALE" {
		want.MarkStale()
	}
	command := &workqueue.CommandResult{State: "OK", Proposal: want}
	workqueue.RefreshCommandResult(command)
	if !bytes.Equal(output, command.Canonical()) {
		t.Fatalf("final proposal diagnostics or canonical command identity differ: %s", output)
	}
}

// WQO-V0-014/017/021/025/031/032: complete final evidence binds the proposal
// before independent drift suppression; source inability is not a source witness.
func TestWorkFinalCheckCaptureBinding(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-021", func(t *testing.T) {
		// Scripts that commit disable detached auto maintenance: its grandchild
		// outlives the adapter leader and reads as residue or an unfinished drain.
		cases := []struct {
			name, change, script, observation, proposal string
			priorMutation, finalMutation, closingError  bool
		}{
			{name: "equal", observation: "UNKNOWN", proposal: "EMPTY"},
			{name: "policy-contradiction", change: "policy", observation: "CONFLICTED", proposal: "EMPTY"},
			{name: "snapshot-contradiction", change: "snapshot", observation: "CONFLICTED", proposal: "EMPTY"},
			{name: "checkpoint-drift", change: "checkpoint", observation: "STALE", proposal: "STALE"},
			{name: "prior-mutation-and-drift", change: "checkpoint", priorMutation: true, observation: "UNKNOWN", proposal: "STALE"},
			{name: "closing-inability", script: "git -C '@ROOT@' -c maintenance.auto=false -c gc.auto=0 -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -qm advance\nprintf dirty > '@ROOT@/untracked-final'\n", finalMutation: true, closingError: true, observation: "UNKNOWN", proposal: "EMPTY"},
			{name: "prior-mutation-and-closing-inability", script: "git -C '@ROOT@' -c maintenance.auto=false -c gc.auto=0 -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -qm advance\nprintf dirty > '@ROOT@/untracked-final'\n", priorMutation: true, finalMutation: true, closingError: true, observation: "UNKNOWN", proposal: "EMPTY"},
			{name: "source-drift-and-mutation", script: "git -C '@ROOT@' -c maintenance.auto=false -c gc.auto=0 -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -qm advance\n", finalMutation: true, observation: "UNKNOWN", proposal: "STALE"},
			{name: "contradiction-and-source-drift", change: "policy", script: "git -C '@ROOT@' -c maintenance.auto=false -c gc.auto=0 -c user.name=Test -c user.email=test@example.invalid commit --allow-empty -qm advance\n", finalMutation: true, observation: "CONFLICTED", proposal: "STALE"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				workCaptureSlot(t)
				response := filepath.Join(t.TempDir(), "final.json")
				root, capture := workFinalCapture(t, tc.script+"cat '"+response+"'")
				if tc.priorMutation {
					workFinalPriorMutation(capture)
				}
				initialID := capture.observation.ID
				checkpoint := *capture.checkpoint
				switch tc.change {
				case "policy":
					checkpoint.PolicyID = "work-queue-policy:sha256:" + strings.Repeat("a", 64)
				case "snapshot":
					checkpoint.SnapshotID = "work-queue-snapshot:sha256:" + strings.Repeat("a", 64)
				case "checkpoint":
					checkpoint.Checkpoint.Version = "advanced"
				}
				workqueue.RefreshCheckpoint(&checkpoint)
				if err := os.WriteFile(response, checkpoint.Canonical(), 0600); err != nil {
					t.Fatal(err)
				}
				envelope, _ := workFinalEnvelope(t)
				ctx, cancel := context.WithCancel(context.Background()) // go test -timeout is the hang detector (decision 0082)
				defer cancel()
				var output bytes.Buffer
				exit := proposeWork(ctx, root, capture, envelope, 1, &output, io.Discard)
				workAssertFinalProposal(t, capture, envelope, output.Bytes(), exit, tc.observation, tc.proposal)
				if tc.change != "" || (tc.finalMutation && !tc.priorMutation) {
					if capture.observation.ID == initialID {
						t.Fatal("final observation was not rebound")
					}
				}
				if tc.priorMutation || tc.finalMutation {
					if capture.observation.MutationState != "CHANGED" || !strings.Contains(string(output.Bytes()), `"MUTATION_DETECTED"`) {
						t.Fatal("positive mutation evidence lost")
					}
				}
				if tc.closingError != (capture.closingError != nil) {
					t.Fatalf("closing qualification cause: %v", capture.closingError)
				}
				if tc.proposal != "STALE" && strings.Contains(output.String(), `"CHECKPOINT_CHANGED"`) {
					t.Fatal("inability or contradiction invented drift")
				}
			})
		}
	})
}

// WQO-V0-025/032: an initial observation cannot replace a failed mandatory
// source recheck, including when that observation already records mutation.
func TestWorkFinalCheckSourceBoundary(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-025", func(t *testing.T) {
		for _, cause := range []string{"unqualified", "cancelled", "deadline", "final-nonzero", "qualified-drift"} {
			t.Run(cause, func(t *testing.T) {
				t.Parallel()
				workCaptureSlot(t)
				root, capture := workFinalCapture(t, "exit 91")
				workFinalPriorMutation(capture)
				initial := capture.observation.ID
				ctx, cancel := context.WithCancel(context.Background()) // go test -timeout is the hang detector (decision 0082)
				defer cancel()
				code := ""
				switch cause {
				case "unqualified":
					cemWrite(t, root, "untracked-final", "dirty")
					code = "SOURCE_UNQUALIFIED"
				case "cancelled":
					cancel()
					code = "CANCELLED"
				case "deadline":
					var stop context.CancelFunc
					ctx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
					defer stop()
					code = "INPUT_LIMIT"
				case "qualified-drift":
					materializationGit(t, root, "commit", "--allow-empty", "-qm", "advance before final verify")
				case "final-nonzero":
					code = "ADAPTER_FAILED"
				}
				envelope, _ := workFinalEnvelope(t)
				var output bytes.Buffer
				exit := proposeWork(ctx, root, capture, envelope, 1, &output, io.Discard)
				if code != "" {
					workAssertFinalError(t, output.Bytes(), exit, code)
				} else {
					workAssertFinalProposal(t, capture, envelope, output.Bytes(), exit, "UNKNOWN", "STALE")
				}
				if capture.observation.ID != initial || capture.observation.MutationState != "CHANGED" {
					t.Fatal("terminal source check erased the prior diagnostic capture")
				}
			})
		}
	})
}

// WQO-V0-025/032/034: run and interrupt the final adapter's actual process group.
// Child PIDs are published before waiting, the shell owns EXIT/INT/TERM cleanup,
// the child has its own finite lifetime, and the existing runner owns group reap.
func TestWorkFinalCheckInterruptedAdapter(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-034", func(t *testing.T) {
		for _, cause := range []string{"cancelled", "deadline"} {
			t.Run(cause, func(t *testing.T) {
				t.Parallel()
				workCaptureSlot(t)
				root, capture := workFinalCapture(t, `
sleep 8 &
child=$!
trap 'kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :' EXIT
trap 'exit 0' INT TERM
printf '%s\n' $$ > "$HOME/final-leader"
printf '%s\n' "$child" > "$HOME/final-child"
wait "$child"
`)
				workFinalPriorMutation(capture)
				initial := capture.observation.ID
				envelope, _ := workFinalEnvelope(t)
				ctx, interrupt := workFinalInterruption(cause)
				defer interrupt()
				var home string
				for _, entry := range capture.runner.env {
					if value, ok := strings.CutPrefix(entry, "HOME="); ok {
						home = value
					}
				}
				leader, child := filepath.Join(home, "final-leader"), filepath.Join(home, "final-child")
				t.Cleanup(func() {
					workAssertHelperGone(t, leader)
					workAssertHelperGone(t, child)
				})
				watcher := make(chan struct{})
				go func() {
					defer close(watcher)
					ticker := time.NewTicker(5 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if raw, err := os.ReadFile(child); err == nil && len(raw) != 0 {
								interrupt()
								return
							}
						}
					}
				}()
				defer func() { interrupt(); <-watcher }()
				var output bytes.Buffer
				exit := proposeWork(ctx, root, capture, envelope, 1, &output, io.Discard)
				code := "CANCELLED"
				if cause == "deadline" {
					code = "INPUT_LIMIT"
				}
				workAssertFinalError(t, output.Bytes(), exit, code)
				for _, path := range []string{leader, child} {
					if raw, err := os.ReadFile(path); err != nil || len(raw) == 0 {
						t.Fatalf("final subprocess was not reached: %s %v", path, err)
					}
					workAssertHelperGone(t, path)
				}
				if capture.observation.ID != initial || capture.observation.MutationState != "CHANGED" {
					t.Fatal("terminal adapter failure erased prior mutation evidence")
				}
			})
		}
	})
}

// WQO-V0-032: cancellation after a complete verify is still terminal when the
// closing acquisition fails. The existing private verifier callback places the
// context cause at that exact boundary; no production timeout option is added.
func TestWorkFinalCheckClosingContext(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-032", func(t *testing.T) {
		for _, cause := range []string{"cancelled", "deadline"} {
			t.Run(cause, func(t *testing.T) {
				t.Parallel()
				workCaptureSlot(t)
				root, capture := workFinalCapture(t, `cat "$HOME/first-checkpoint"`)
				workFinalPriorMutation(capture)
				initial := capture.observation.ID
				ctx, interrupt := workFinalInterruption(cause)
				defer interrupt()
				verify, calls := capture.runner.verifyTarget, 0
				capture.runner.verifyTarget = func(check context.Context) error {
					if err := verify(check); err != nil {
						return err
					}
					calls++
					if calls == 2 {
						interrupt()
						<-ctx.Done()
					}
					return nil
				}
				envelope, _ := workFinalEnvelope(t)
				var output bytes.Buffer
				exit := proposeWork(ctx, root, capture, envelope, 1, &output, io.Discard)
				code, expected := "CANCELLED", context.Canceled
				if cause == "deadline" {
					code, expected = "INPUT_LIMIT", context.DeadlineExceeded
				}
				workAssertFinalError(t, output.Bytes(), exit, code)
				if calls != 2 || !errors.Is(capture.closingError, expected) {
					t.Fatalf("closing cause not retained independently: calls=%d error=%v", calls, capture.closingError)
				}
				if capture.observation.ID != initial || capture.observation.MutationState != "CHANGED" {
					t.Fatal("closing context failure erased prior mutation evidence")
				}
			})
		}
	})
}

// workFinalInterruption returns a parent context and the action that ends it at
// the witnessed point: cancellation, or a deadline that expires only when called.
// No wall clock runs before the behaviour under test (decision 0082).
func workFinalInterruption(cause string) (context.Context, func()) {
	if cause == "cancelled" {
		return context.WithCancel(context.Background())
	}
	deadline := &workWitnessedDeadline{Context: context.Background(), done: make(chan struct{})}
	return deadline, deadline.expire
}

type workWitnessedDeadline struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func (ctx *workWitnessedDeadline) Done() <-chan struct{} { return ctx.done }
func (ctx *workWitnessedDeadline) expire()               { ctx.once.Do(func() { close(ctx.done) }) }
func (ctx *workWitnessedDeadline) Err() error {
	select {
	case <-ctx.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func workFinalResult(t *testing.T, output []byte) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 6 || string(result["profile"]) != `"work-command-result/0"` || string(result["state"]) != `"OK"` || string(result["errorCode"]) != "null" || string(result["observation"]) != "null" || string(result["proposal"]) == "null" {
		t.Fatalf("not an exclusive successful proposal envelope: %s", output)
	}
	return result
}
