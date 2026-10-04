package localcompletion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// This is a native Check/Seal capture-boundary fixture, not a complete CLI
// qualification. It pauses only after the real admitted stage owns its prefix.
func TestAggregateCaptureSignalHelper(t *testing.T) {
	root := os.Getenv("CORVINT_CAPTURE_SIGNAL_ROOT")
	if root == "" {
		return
	}
	key := os.Getenv("CORVINT_CAPTURE_SIGNAL_KEY")
	repo, err := open(root, key)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	options := dogfoodflow.CheckOptions{Root: root, Base: saved.Plan.Base, AggregatePublish: func(context.Context, string, dogfoodflow.AggregateCheckCapture) error {
		t.Fatal("interrupted capture published")
		return nil
	}}
	options.AggregateBegin = func(ctx context.Context, _ string, raw []byte) (*dogfoodflow.AggregateCheckStage, error) {
		snap, err := repo.snapshot(ctx)
		if err != nil {
			return nil, err
		}
		stage, err := repo.beginAggregateCheckCapture(ctx, saved, snap, raw)
		if err != nil {
			return nil, err
		}
		if _, err = stage.Stderr.Write([]byte("native strict-check capture prefix before signal\n")); err != nil {
			return nil, err
		}
		if err = os.WriteFile(os.Getenv("CORVINT_CAPTURE_SIGNAL_READY"), []byte("ready"), 0600); err != nil {
			return nil, err
		}
		<-ctx.Done()
		return stage, nil
	}
	var code int
	if os.Getenv("CORVINT_CAPTURE_SIGNAL_MODE") == "seal" {
		code, err = dogfoodflow.Seal(ctx, options, io.Discard, io.Discard)
	} else {
		code, err = dogfoodflow.Check(ctx, options, io.Discard, io.Discard)
	}
	if os.Getenv("CORVINT_CAPTURE_SIGNAL_RETRY") == "1" {
		if code != 2 || err != nil {
			t.Fatalf("retained lock retry: %d %v", code, err)
		}
		return
	}
	if err == nil {
		t.Fatalf("signal interruption disappeared: %d", code)
	}
}

func TestAggregateCaptureNativeParentSignals(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("native Owner unavailable")
	}
	for _, mode := range []string{"check", "seal"} {
		for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL} {
			t.Run(mode+"/"+sig.String(), func(t *testing.T) {
				repo, saved, receipt, report := aggregatePublicationFixture(t)
				for _, v := range []struct {
					role string
					raw  []byte
				}{{"outcome", receipt}, {"report", report}} {
					if err := repo.publishAggregateRole(context.Background(), saved, v.role, v.raw); err != nil {
						t.Fatal(err)
					}
				}
				dir := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
				ready := filepath.Join(t.TempDir(), "ready")
				command := exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalHelper$", "-test.v")
				command.Env = append(os.Environ(), "CORVINT_CAPTURE_SIGNAL_ROOT="+repo.auth.Root, "CORVINT_CAPTURE_SIGNAL_KEY="+repo.session, "CORVINT_CAPTURE_SIGNAL_MODE="+mode, "CORVINT_CAPTURE_SIGNAL_READY="+ready)
				var output bytes.Buffer
				command.Stdout = &output
				command.Stderr = &output
				owner, err := groupreap.Start(command)
				if err != nil {
					t.Fatal(err)
				}
				joined := false
				defer func() {
					if !joined {
						owner.Stop()
						bound, cancel := context.WithTimeout(context.Background(), 20*time.Second)
						defer cancel()
						result := owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done(), Expired: func() bool { return bound.Err() != nil }})
						if result.State != groupreap.Released {
							t.Errorf("owned fixture cleanup HOLD: %+v", result)
						}
					}
				}()
				deadline := time.Now().Add(15 * time.Second)
				for {
					if _, err := os.Stat(ready); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("no ready marker: %s", output.String())
					}
					time.Sleep(10 * time.Millisecond)
				}
				prefix, err := os.ReadFile(filepath.Join(dir, "check.stderr"))
				if err != nil || string(prefix) != "native strict-check capture prefix before signal\n" {
					t.Fatal("prefix absent at readiness", err)
				}
				if err = command.Process.Signal(sig); err != nil {
					t.Fatal(err)
				}
				bound, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				result := owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done(), Expired: func() bool { return bound.Err() != nil }})
				cancel()
				joined = true
				if result.State != groupreap.Released {
					t.Fatalf("native fixture HOLD: %+v", result)
				}
				if sig != syscall.SIGKILL && result.WaitErr != nil {
					t.Fatalf("normal signal return: %v %s", result.WaitErr, output.String())
				}
				after, err := os.ReadFile(filepath.Join(dir, "check.stderr"))
				if err != nil || !bytes.Equal(prefix, after) {
					t.Fatal("signal lost prefix", err)
				}
				if _, err = os.Stat(filepath.Join(dir, "check.capture.json")); !os.IsNotExist(err) {
					t.Fatal("interruption qualified capture")
				}
				if sig == syscall.SIGKILL {
					if _, err = os.Stat(filepath.Join(dir, "check.observed.json")); !os.IsNotExist(err) {
						t.Fatal("KILL claimed closed observation")
					}
					if _, err = os.Stat(filepath.Join(repo.auth.GitDir, "corvint/local-completion/operation.lock")); err != nil {
						t.Fatal("KILL lost lock", err)
					}
					retryCtx, retryCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer retryCancel()
					retry := exec.CommandContext(retryCtx, os.Args[0], "-test.run=^TestAggregateCaptureSignalHelper$")
					retry.WaitDelay = time.Second
					retry.Env = append(command.Env, "CORVINT_CAPTURE_SIGNAL_RETRY=1")
					if raw, err := retry.CombinedOutput(); err != nil {
						t.Fatalf("new-process refusal: %v %s", err, raw)
					}
				} else if _, err = os.Stat(filepath.Join(dir, "check.observed.json")); err != nil {
					t.Fatal("missing closed interruption observation", err)
				}
			})
		}
	}
}

func TestAggregateCapturePrefixSurvivesCancellationAndDrift(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline", "drift", "hold", "overflow", "close-failure"} {
		t.Run(mode, func(t *testing.T) {
			repo, saved, receipt, report := aggregatePublicationFixture(t)
			for _, v := range []struct {
				role string
				raw  []byte
			}{{"outcome", receipt}, {"report", report}} {
				if err := repo.publishAggregateRole(context.Background(), saved, v.role, v.raw); err != nil {
					t.Fatal(err)
				}
			}
			ctx, release, err := dogfoodoperation.Acquire(context.Background(), repo.auth.GitDir)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			var expire context.CancelFunc
			if mode == "deadline" {
				ctx, expire = context.WithDeadline(ctx, time.Now().Add(time.Second))
				defer expire()
			}
			snap, err := repo.snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			stage, err := repo.beginAggregateCheckCapture(ctx, saved, snap, report)
			if err != nil {
				t.Fatal(err)
			}
			prefix := []byte("actual strict-check prefix before failure\n")
			if _, err = stage.Stdout.Write(prefix); err != nil {
				t.Fatal(err)
			}
			dir := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
			if raw, err := os.ReadFile(filepath.Join(dir, "check.stdout")); err != nil || !bytes.Equal(raw, prefix) {
				t.Fatal("prefix not on disk before close", err)
			}
			var runErr error
			switch mode {
			case "cancel":
				cancel()
				runErr = context.Canceled
			case "deadline":
				<-ctx.Done()
				runErr = ctx.Err()
			case "drift":
				if err = os.WriteFile(repo.aggregateSourcePaths()["prior-report"], []byte("foreign drift"), 0600); err != nil {
					t.Fatal(err)
				}
				runErr = errors.New("aggregate-prior-evidence-drift")
			case "hold":
				runErr = dogfoodoperation.Hold(ctx, []byte(`{"state":"HOLD"}`), new(int))
			case "overflow":
				if _, err = stage.Stderr.Write(bytes.Repeat([]byte("x"), maxLogBytes+1)); err == nil {
					t.Fatal("overflow admitted")
				}
			case "close-failure":
				if err = stage.Stderr.(*aggregateCaptureWriter).file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			capture, closeErr := stage.Finish(2, runErr)
			if mode == "overflow" || mode == "close-failure" {
				if closeErr == nil {
					t.Fatal("capture IO failure hidden")
				}
			} else if closeErr != nil {
				t.Fatal(closeErr)
			}
			if !capture.Failed || !bytes.Equal(capture.Stdout, prefix) {
				t.Fatal("failed observation lost", capture)
			}
			if _, err = os.Stat(filepath.Join(dir, "check.observed.json")); err != nil {
				t.Fatal("missing failed identity", err)
			}
			if _, err = os.Stat(filepath.Join(dir, "check.capture.json")); !os.IsNotExist(err) {
				t.Fatal("diagnostic qualified success")
			}
			if saved.Terminal != nil {
				t.Fatal("terminal manufactured")
			}
			release()
			if mode == "hold" {
				if _, unlock, err := dogfoodoperation.Acquire(context.Background(), repo.auth.GitDir); err == nil || unlock != nil {
					t.Fatal("held capture lost lock")
				}
			}
			if _, err = repo.aggregateHistoryUsage(); err != nil {
				t.Fatal("capture escaped reservation", err)
			}
		})
	}
}

// TestValidatePlanRefusesFinalCheckInAnyForm pins LCP-V0-015: a selected check
// never runs the final check, whether by script name or by subverb.
func TestValidatePlanRefusesFinalCheckInAnyForm(t *testing.T) {
	cases := map[string]struct {
		argv []string
		want string
	}{
		"script name":       {[]string{"script/dogfood-check.sh", "BASE"}, "final-check-not-prerequisite"},
		"make target":       {[]string{"make", "dogfood-check"}, "final-check-not-prerequisite"},
		"check subverb":     {[]string{"/usr/local/bin/corvint", "dogfood", "check", "BASE"}, "final-check-not-prerequisite"},
		"seal subverb":      {[]string{"corvint", "--root", ".", "dogfood", "seal", "BASE"}, "final-check-not-prerequisite"},
		"change subverb":    {[]string{"corvint", "dogfood", "change", "BASE"}, ""},
		"non-adjacent pair": {[]string{"echo", "dogfood", "then", "check"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan := Plan{
				Base:    "0123456789abcdef0123456789abcdef01234567",
				Intents: []string{"docs/specs/example.md"},
				Checks:  []Check{{ID: "check", Argv: tc.argv, TimeoutSeconds: 60}},
			}
			got := ""
			if err := validatePlan(plan); err != nil {
				got = err.Error()
			}
			if got != tc.want {
				t.Fatalf("validatePlan(%v) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}

func aggregateStorageFixture(t *testing.T) (*repository, *state, *aggregateState, []byte) {
	t.Helper()
	root, key, plan := fixture(t, []string{"true"})
	repo := beginFixture(t, root, key, plan)
	for role, name := range repo.aggregateSourcePaths() {
		if role != "state-before" {
			if err := writeFile(name, []byte(role+" original bytes\n")); err != nil {
				t.Fatal(err)
			}
		}
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	receipt := []byte("prepared receipt fixture\n")
	old := func(role, kind string) *aggregateOld {
		raw, err := readFile(repo.aggregateSourcePaths()[role], maxArtifactBytes)
		if err != nil {
			t.Fatal(err)
		}
		return &aggregateOld{prefixedDigest(raw), int64(len(raw)), kind}
	}
	proposed := &aggregateState{Profile: tracerecordrepo.AggregateOutcomeProfile, BindingSHA256: "sha256:" + strings.Repeat("1", 64), ReceiptSHA256: prefixedDigest(receipt), LegacyFailureSHA256: "sha256:" + strings.Repeat("2", 64), Phase: "PREPARED", Issued: []aggregateIssued{}, ExpectedOld: aggregateExpectedOld{old("prior-outcome", "failed-recorder-output"), old("prior-report", "legacy-report")}}
	return repo, saved, proposed, receipt
}

func TestAggregatePreservationResumeEveryCopyBoundary(t *testing.T) {
	points := []string{"reservation-installed", "state-before-closed", "reservation-state-saved", "snapshot-manifest-installed", "snapshot-sealed-acknowledged"}
	for _, role := range aggregateSnapshotRoles {
		points = append(points, "payload-closed:"+role)
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			repo, saved, proposed, receipt := aggregateStorageFixture(t)
			original, err := readFile(repo.local("state.json"), maxStateBytes)
			if err != nil {
				t.Fatal(err)
			}
			repo.aggregateFault = func(at string) error {
				if at == point {
					return errors.New("injected-boundary")
				}
				return nil
			}
			if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err == nil || err.Error() != "injected-boundary" {
				t.Fatalf("fault not reached: %v", err)
			}
			reloaded, err := open(repo.auth.Root, repo.session)
			if err != nil {
				t.Fatal(err)
			}
			current, err := reloaded.load()
			if err != nil {
				t.Fatal(err)
			}
			if err := reloaded.preserveAggregate(context.Background(), current, proposed, receipt); err != nil {
				t.Fatal("resume", err)
			}
			current, err = reloaded.load()
			if err != nil {
				t.Fatal(err)
			}
			if current.AggregateOutcome.AttemptCount != 1 || len(current.AggregateOutcome.Snapshots) != 1 {
				t.Fatal("resume consumed ordinal")
			}
			sealed, err := reloaded.aggregatePreservation(current.AggregateOutcome)
			if err != nil || !sealed {
				t.Fatalf("closure sealed=%v err=%v", sealed, err)
			}
			directory := reloaded.aggregateSnapshotDirectory(current.AggregateOutcome.BindingSHA256, current.AggregateOutcome.ActiveSnapshotSHA256)
			preserved, err := readFile(filepath.Join(directory, "state-before.bin"), maxStateBytes)
			if err != nil || !bytes.Equal(preserved, original) {
				t.Fatal("pre-update state was not preserved", err)
			}
			before, err := readFile(reloaded.local("state.json"), maxStateBytes)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if _, err := reloaded.load(); err != nil {
					t.Fatal(err)
				}
			}
			after, err := readFile(reloaded.local("state.json"), maxStateBytes)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("read-only load changed state")
			}
		})
	}
}

func TestAggregatePreservationRejectsMissingPayloadAndCapacity(t *testing.T) {
	repo, saved, proposed, receipt := aggregateStorageFixture(t)
	if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err != nil {
		t.Fatal(err)
	}
	directory := repo.aggregateSnapshotDirectory(proposed.BindingSHA256, proposed.ActiveSnapshotSHA256)
	if err := os.Remove(filepath.Join(directory, "prior-report.bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.aggregatePreservation(proposed); err == nil {
		t.Fatal("missing sealed payload accepted")
	}
	repo, saved, proposed, receipt = aggregateStorageFixture(t)
	if err := os.MkdirAll(repo.aggregateDirectory(), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(repo.aggregateDirectory(), "retained-fault-stage"))
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(aggregateHistoryLimit); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err == nil || err.Error() != "aggregate-history-bound-exceeded" {
		t.Fatalf("capacity: %v", err)
	}
	if _, err := os.Stat(repo.aggregateReservationPath(1)); !os.IsNotExist(err) {
		t.Fatal("reservation written after capacity refusal", err)
	}
}

func TestAggregateLegacyAdmissionRequiresActualClosedWire(t *testing.T) {
	valid := []byte(`{"code": "admitted-path-limit", "error": "changed_paths exceeds 200 paths", "ok": false}` + "\n")
	if !strictLegacyAdmission(nil, valid, 2) {
		t.Fatal("actual legacy refusal was not admitted")
	}
	for _, raw := range []string{
		`{"code":"admitted-path-limit","error":"changed_paths exceeds 200 paths","ok":false,"extra":1}`,
		`{"code":"admitted-path-limit","error":"changed_paths exceeds 200 paths","ok":true}`,
		`{"code":"admitted-path-limit","error":"different failure","ok":false}`,
		`{"code":"admitted-path-limit","error":"changed_paths exceeds 200 paths","ok":false,"ok":false}`,
	} {
		if strictLegacyAdmission(nil, []byte(raw), 2) {
			t.Fatalf("admitted invalid wire %s", raw)
		}
	}
	if strictLegacyAdmission([]byte("x"), valid, 2) || strictLegacyAdmission(nil, valid, 1) {
		t.Fatal("admitted wrong streams or exit")
	}
}

func TestAggregatePreservationCancellationStopsBeforeNextWrite(t *testing.T) {
	for _, point := range []string{"", "reservation-installed", "state-before-closed", "payload-closed:prior-report"} {
		t.Run(point, func(t *testing.T) {
			repo, saved, proposed, receipt := aggregateStorageFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if point == "" {
				cancel()
			} else {
				repo.aggregateFault = func(at string) error {
					if at == point {
						cancel()
					}
					return nil
				}
			}
			if err := repo.preserveAggregate(ctx, saved, proposed, receipt); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel %s: %v", point, err)
			}
			if point == "" {
				if _, err := os.Stat(repo.aggregateReservationPath(1)); !os.IsNotExist(err) {
					t.Fatalf("reserved after cancellation: %v", err)
				}
			}
			if point == "reservation-installed" {
				reservation, err := repo.readAggregateReservation(1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(repo.aggregateSnapshotDirectory(reservation.BindingSHA256, reservation.SnapshotSHA256), "state-before.bin")); !os.IsNotExist(err) {
					t.Fatalf("copied after cancellation: %v", err)
				}
			}
		})
	}
	budget, err := gitrun.NewOperationBudget(64, time.Now().Add(-time.Second), false)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "expired")
	if err = writeAggregateExclusiveContext(gitrun.WithOperationBudget(context.Background(), budget), name, []byte("x")); err == nil || err.Error() != "aggregate-operation-deadline" {
		t.Fatalf("expired publication: %v", err)
	}
	if _, err = os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("published after deadline: %v", err)
	}
}

func TestAggregatePreservationProcessHelper(t *testing.T) {
	root := os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_ROOT")
	if root == "" {
		return
	}
	repo, err := open(root, os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	var proposed aggregateState
	if err = json.Unmarshal([]byte(os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_PROPOSAL")), &proposed); err != nil {
		t.Fatal(err)
	}
	point := os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_CRASH")
	repo.aggregateFault = func(at string) error {
		if at == point {
			os.Exit(88)
		}
		return nil
	}
	if role := os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_ROLE"); role != "" {
		raw, err := os.ReadFile(os.Getenv("CORVINT_AGGREGATE_STORAGE_TEST_INPUT"))
		if err != nil {
			t.Fatal(err)
		}
		if err = repo.publishAggregateRole(context.Background(), saved, role, raw); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err = repo.preserveAggregate(context.Background(), saved, &proposed, []byte("prepared receipt fixture\n")); err != nil {
		t.Fatal(err)
	}
}

func TestAggregatePreservationNewProcessCrashRecovery(t *testing.T) {
	points := []string{"reservation-installed", "state-before-closed", "reservation-state-saved", "snapshot-manifest-installed", "snapshot-sealed-acknowledged"}
	for _, role := range aggregateSnapshotRoles {
		points = append(points, "payload-closed:"+role)
	}
	for _, point := range points {
		t.Run(point, func(t *testing.T) {
			repo, _, proposed, _ := aggregateStorageFixture(t)
			original, err := os.ReadFile(repo.local("state.json"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(proposed)
			if err != nil {
				t.Fatal(err)
			}
			invoke := func(crash string) ([]byte, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAggregatePreservationProcessHelper$")
				child.Env = append(os.Environ(), "CORVINT_AGGREGATE_STORAGE_TEST_ROOT="+repo.auth.Root, "CORVINT_AGGREGATE_STORAGE_TEST_KEY="+repo.session, "CORVINT_AGGREGATE_STORAGE_TEST_PROPOSAL="+string(raw), "CORVINT_AGGREGATE_STORAGE_TEST_CRASH="+crash)
				child.WaitDelay = time.Second
				return child.CombinedOutput()
			}
			output, err := invoke(point)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 88 {
				t.Fatalf("crash boundary %s: %v %s", point, err, output)
			}
			output, err = invoke("")
			if err != nil {
				t.Fatalf("fresh process resume: %v %s", err, output)
			}
			saved, err := repo.load()
			if err != nil {
				t.Fatal(err)
			}
			a := saved.AggregateOutcome
			if a == nil || a.AttemptCount != 1 || a.PreservationState != "SEALED" {
				t.Fatalf("resume state: %+v", a)
			}
			if sealed, err := repo.aggregatePreservation(a); err != nil || !sealed {
				t.Fatalf("preserved closure: %v %v", sealed, err)
			}
			before, err := os.ReadFile(filepath.Join(repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256), "state-before.bin"))
			if err != nil || !bytes.Equal(before, original) {
				t.Fatalf("pre-crash state lost: %v", err)
			}
		})
	}
}

// Publication protocol tests use a real immutable receipt. They deliberately
// do not claim a strict-check or terminal witness; the CLI fixture owns that.
func aggregatePublicationFixture(t *testing.T) (*repository, *state, []byte, []byte) {
	t.Helper()
	return aggregatePublicationFixtureWithOldArtifacts(t, true, true)
}

func aggregatePublicationFixtureWithOldArtifacts(t *testing.T, outcomePresent, reportPresent bool) (*repository, *state, []byte, []byte) {
	t.Helper()
	root, key, plan := fixture(t, []string{"true"})
	for i := 0; i < 201; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%03d.go", i)), []byte(fmt.Sprintf("package fixture\nconst V%d = %d\n", i, i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	localGit(t, root, "add", ".")
	localGit(t, root, "commit", "-qm", "aggregate publication source")
	repo := beginFixture(t, root, key, plan)
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	target := localGit(t, root, "rev-parse", "HEAD")
	expected := tracerecordrepo.AggregateExpected{ObjectFormat: "sha1", Base: plan.Base, Target: target, Tree: localGit(t, root, "rev-parse", "HEAD^{tree}"), AdmissionPolicy: tracerecordrepo.AggregateAdmissionPolicy, Task: "Local completion " + saved.PlanDigest, Verification: []string{displayChecks(plan)}, Outcome: "passed"}
	receipt, err := tracerecordrepo.ProduceAggregateOutcome(context.Background(), root, expected)
	if err != nil {
		t.Fatal(err)
	}
	value, err := tracerecordrepo.ParseAggregateOutcome(receipt)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := tracerecordrepo.AggregateBinding(value)
	if err != nil {
		t.Fatal(err)
	}
	saved.ReportSet = &reportSet{PlanDigest: saved.PlanDigest, Target: target, Bindings: []artifact{}, Reports: []artifact{}, Observations: []observation{}, Digest: strings.Repeat("a", 64)}
	if err = repo.save(saved); err != nil {
		t.Fatal(err)
	}
	old := func(role, kind string, raw []byte, present bool) *aggregateOld {
		name := repo.aggregateSourcePaths()[role]
		if !present {
			if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			return nil
		}
		if err := writeFile(name, raw); err != nil {
			t.Fatal(err)
		}
		return &aggregateOld{prefixedDigest(raw), int64(len(raw)), kind}
	}
	proposed := &aggregateState{Profile: tracerecordrepo.AggregateOutcomeProfile, BindingSHA256: binding, ReceiptSHA256: prefixedDigest(receipt), LegacyFailureSHA256: "sha256:" + strings.Repeat("b", 64), Phase: "PREPARED", Issued: []aggregateIssued{}, ExpectedOld: aggregateExpectedOld{
		old("prior-outcome", "failed-recorder-output", []byte{}, outcomePresent),
		old("prior-report", "legacy-report", []byte("legacy report protocol fixture\n"), reportPresent),
	}}
	if err = repo.preserveAggregate(context.Background(), saved, proposed, receipt); err != nil {
		t.Fatal(err)
	}
	report, err := dogfoodflow.EncodeAggregateReport(dogfoodflow.AggregateReport{Profile: dogfoodflow.AggregateReportProfile, Base: plan.Base, Target: target, CompletionState: "complete", Steps: []dogfoodflow.AggregateReportStep{{Name: "local-outcome", Status: "PRODUCED", Reason: "none"}}, OCMStatus: json.RawMessage(`{}`), LocalOutcomeEvidenceSHA256: prefixedDigest(receipt), Anchor: dogfoodflow.AggregateReportAnchor{State: "NOT_OBSERVED"}, OCMLinkPlan: json.RawMessage(`null`), PacketCoverage: []json.RawMessage{}, Enrollment: dogfoodflow.AggregateEnrollment{Session: saved.Session, Generation: saved.Generation, PlanDigest: saved.PlanDigest, BindingSHA256: binding}, LocalOutcomeProfile: tracerecordrepo.AggregateOutcomeProfile})
	if err != nil {
		t.Fatal(err)
	}
	return repo, saved, receipt, report
}

func TestAggregatePublicationNewProcessCrashRecovery(t *testing.T) {
	for _, role := range []string{"outcome", "report"} {
		for _, boundary := range []string{"prepared-closed:", "stage-closed:", "issued-saved:", "artifact-renamed:", "phase-acknowledged:"} {
			t.Run(boundary+role, func(t *testing.T) {
				repo, saved, receipt, report := aggregatePublicationFixture(t)
				raw := receipt
				if role == "report" {
					if err := repo.publishAggregateRole(context.Background(), saved, "outcome", receipt); err != nil {
						t.Fatal(err)
					}
					raw = report
				}
				input := filepath.Join(t.TempDir(), "publication.json")
				if err := os.WriteFile(input, raw, 0600); err != nil {
					t.Fatal(err)
				}
				invoke := func(point string) ([]byte, error) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAggregatePreservationProcessHelper$")
					child.Env = append(os.Environ(), "CORVINT_AGGREGATE_STORAGE_TEST_ROOT="+repo.auth.Root, "CORVINT_AGGREGATE_STORAGE_TEST_KEY="+repo.session, "CORVINT_AGGREGATE_STORAGE_TEST_PROPOSAL={}", "CORVINT_AGGREGATE_STORAGE_TEST_ROLE="+role, "CORVINT_AGGREGATE_STORAGE_TEST_INPUT="+input, "CORVINT_AGGREGATE_STORAGE_TEST_CRASH="+point)
					child.WaitDelay = time.Second
					return child.CombinedOutput()
				}
				out, err := invoke(boundary + role)
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 88 {
					t.Fatalf("fault: %v %s", err, out)
				}
				before, err := os.ReadFile(repo.local("state.json"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err = repo.load(); err != nil {
					t.Fatalf("pending load: %v", err)
				}
				after, err := os.ReadFile(repo.local("state.json"))
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("load wrote pending state: %v", err)
				}
				out, err = invoke("")
				if err != nil {
					t.Fatalf("new process recovery: %v %s", err, out)
				}
				current, err := repo.load()
				if err != nil {
					t.Fatal(err)
				}
				rank, err := repo.aggregatePublishedPrefix(current)
				want := 1
				if role == "report" {
					want = 2
				}
				if err != nil || rank != want || current.AggregateOutcome.AttemptCount != 1 {
					t.Fatalf("prefix=%d want=%d err=%v", rank, want, err)
				}
			})
		}
	}
}

func TestAggregatePublicationRefusesForeignOldArtifact(t *testing.T) {
	repo, saved, receipt, _ := aggregatePublicationFixture(t)
	path := repo.aggregateSourcePaths()["prior-outcome"]
	foreign := []byte("unissued foreign artifact")
	if err := writeFile(path, foreign); err != nil {
		t.Fatal(err)
	}
	if err := repo.publishAggregateRole(context.Background(), saved, "outcome", receipt); err == nil || err.Error() != "aggregate-prior-evidence-drift" {
		t.Fatalf("foreign artifact accepted: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, foreign) {
		t.Fatalf("foreign artifact overwritten: %v", err)
	}
}

func TestAggregateHistoryExactCapacityAndOrphanConflict(t *testing.T) {
	t.Run("exact byte bound includes retained fault bytes", func(t *testing.T) {
		repo, _, _, _ := aggregateStorageFixture(t)
		used, err := repo.aggregateHistoryUsage()
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(repo.aggregateDirectory(), "retained-fault-stage")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = file.Truncate(aggregateHistoryLimit - used); err != nil {
			t.Fatal(err)
		}
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
		if actual, err := repo.aggregateHistoryUsage(); err != nil || actual != aggregateHistoryLimit {
			t.Fatalf("exact: %d %v", actual, err)
		}
		if err = os.Truncate(path, aggregateHistoryLimit-used+1); err != nil {
			t.Fatal(err)
		}
		if _, err = repo.aggregateHistoryUsage(); err == nil || err.Error() != "aggregate-history-bound-exceeded" {
			t.Fatalf("plus one: %v", err)
		}
	})
	t.Run("conflicting orphan keeps original reservation", func(t *testing.T) {
		repo, saved, proposed, receipt := aggregateStorageFixture(t)
		repo.aggregateFault = func(at string) error {
			if at == "reservation-installed" {
				return errors.New("fault")
			}
			return nil
		}
		if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err == nil {
			t.Fatal("fault not reached")
		}
		original, err := os.ReadFile(repo.aggregateReservationPath(1))
		if err != nil {
			t.Fatal(err)
		}
		if err = writeFile(repo.aggregateSourcePaths()["prior-outcome"], []byte("foreign")); err != nil {
			t.Fatal(err)
		}
		other, err := open(repo.auth.Root, repo.session)
		if err != nil {
			t.Fatal(err)
		}
		current, err := other.load()
		if err != nil {
			t.Fatal(err)
		}
		if err = other.preserveAggregate(context.Background(), current, proposed, receipt); err == nil || err.Error() != "aggregate-prior-evidence-drift" {
			t.Fatalf("adopted conflict: %v", err)
		}
		after, err := os.ReadFile(repo.aggregateReservationPath(1))
		if err != nil || !bytes.Equal(original, after) {
			t.Fatalf("replaced reservation: %v", err)
		}
		if _, err = os.Stat(repo.aggregateReservationPath(2)); !os.IsNotExist(err) {
			t.Fatalf("consumed new ordinal: %v", err)
		}
	})
	t.Run("ordinal 65 refuses before payload work", func(t *testing.T) {
		repo, saved, proposed, receipt := aggregateStorageFixture(t)
		saved.AggregateOutcome = proposed
		proposed.AttemptCount = 64
		if err := repo.reserveAggregateSnapshot(context.Background(), saved, proposed, receipt); err == nil || err.Error() != "aggregate-history-bound-exceeded" {
			t.Fatalf("ordinal 65: %v", err)
		}
		if _, err := os.Stat(repo.aggregateReservationPath(65)); !os.IsNotExist(err) {
			t.Fatalf("wrote ordinal 65: %v", err)
		}
	})
}

func TestAggregateReplacementPreservesTwoDistinctSnapshots(t *testing.T) {
	repo, saved, receipt, report := aggregatePublicationFixture(t)
	for _, item := range []struct {
		role string
		raw  []byte
	}{{"outcome", receipt}, {"report", report}} {
		if err := repo.publishAggregateRole(context.Background(), saved, item.role, item.raw); err != nil {
			t.Fatal(err)
		}
	}
	first := saved.AggregateOutcome.ActiveSnapshotSHA256
	directory := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, first)
	manifest, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	// A retained partial failed capture causes a new preservation admission.
	if err = writeAggregateExclusive(filepath.Join(directory, "check.stdout"), []byte("actual failed capture prefix")); err != nil {
		t.Fatal(err)
	}
	if err = repo.preserveAggregateReplacement(context.Background(), saved, receipt); err != nil {
		t.Fatal(err)
	}
	if saved.AggregateOutcome.AttemptCount != 2 || saved.AggregateOutcome.ActiveSnapshotSHA256 == first {
		t.Fatalf("replacement did not preserve new snapshot: %+v", saved.AggregateOutcome)
	}
	prior, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil || !bytes.Equal(prior, manifest) {
		t.Fatalf("old snapshot changed: %v", err)
	}
	if err = repo.validateAggregateLedger(saved.AggregateOutcome); err != nil {
		t.Fatal(err)
	}
	second := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
	raw, err := os.ReadFile(filepath.Join(second, "check-stdout.bin"))
	if err != nil || string(raw) != "actual failed capture prefix" {
		t.Fatalf("capture was not preserved: %v", err)
	}
}

// A fresh helper reloads disk state and follows the preservation/begin sequence
// used by the pending same-selector check. It does not qualify a strict check.
func TestAggregateCaptureRetryProcessHelper(t *testing.T) {
	root := os.Getenv("CORVINT_CAPTURE_RETRY_ROOT")
	if root == "" {
		return
	}
	repo, err := open(root, os.Getenv("CORVINT_CAPTURE_RETRY_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, release, err := dogfoodoperation.Acquire(context.Background(), repo.auth.GitDir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := readFile(repo.aggregateSourcePaths()["prior-outcome"], maxArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	report, err := readFile(repo.aggregateSourcePaths()["prior-report"], maxArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.preserveAggregateReplacement(ctx, saved, receipt); err != nil {
		t.Fatal(err)
	}
	snap, err := repo.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	point := os.Getenv("CORVINT_CAPTURE_RETRY_POINT")
	repo.aggregateFault = func(at string) error {
		if at == point {
			cancel()
		}
		return nil
	}
	stage, err := repo.beginAggregateCheckCapture(ctx, saved, snap, report)
	if point != "" {
		if !errors.Is(err, context.Canceled) || stage != nil {
			t.Fatalf("early cancellation: stage=%v err=%v", stage, err)
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		if _, err = stage.Finish(2, errors.New("unqualified retry witness")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAggregateCaptureMarkerOnlyNewProcessRetry(t *testing.T) {
	for _, point := range []string{"check-started-closed", "check-stdout-opened"} {
		t.Run(point, func(t *testing.T) {
			repo, saved, receipt, report := aggregatePublicationFixture(t)
			for _, item := range []struct {
				role string
				raw  []byte
			}{{"outcome", receipt}, {"report", report}} {
				if err := repo.publishAggregateRole(context.Background(), saved, item.role, item.raw); err != nil {
					t.Fatal(err)
				}
			}
			invoke := func(point string) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAggregateCaptureRetryProcessHelper$")
				cmd.Env = append(os.Environ(), "CORVINT_CAPTURE_RETRY_ROOT="+repo.auth.Root, "CORVINT_CAPTURE_RETRY_KEY="+repo.session, "CORVINT_CAPTURE_RETRY_POINT="+point)
				cmd.WaitDelay = time.Second
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("fresh process retry %s: %v %s", point, err, out)
				}
			}
			retained := map[string][]byte{}
			for ordinal := 1; ordinal <= 3; ordinal++ {
				boundary := point
				if ordinal == 3 {
					boundary = ""
				}
				invoke(boundary)
				current, err := repo.load()
				if err != nil {
					t.Fatal(err)
				}
				if current.AggregateOutcome.AttemptCount != ordinal || current.Terminal != nil {
					t.Fatalf("retry ordinal/terminal: %+v", current.AggregateOutcome)
				}
				dir := repo.aggregateSnapshotDirectory(current.AggregateOutcome.BindingSHA256, current.AggregateOutcome.ActiveSnapshotSHA256)
				name := filepath.Join(dir, "check.started.json")
				raw, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, exists := retained[name]; exists {
					t.Fatal("reused snapshot")
				}
				retained[name] = raw
				if ordinal < 3 {
					for _, stream := range []string{"check.stderr", "check.capture.json", "check.observed.json"} {
						if _, err := os.Stat(filepath.Join(dir, stream)); !os.IsNotExist(err) {
							t.Fatalf("early attempt created %s: %v", stream, err)
						}
					}
					_, err := os.Stat(filepath.Join(dir, "check.stdout"))
					if point == "check-started-closed" && !os.IsNotExist(err) {
						t.Fatal("stdout before cancellation", err)
					}
					if point == "check-stdout-opened" && err != nil {
						t.Fatal(err)
					}
				}
				for name, want := range retained {
					got, err := os.ReadFile(name)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatal("prior marker changed", err)
					}
				}
				for name := range retained {
					entries, err := os.ReadDir(filepath.Dir(name))
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range entries {
						if strings.HasPrefix(entry.Name(), ".aggregate-stage-") {
							t.Fatal("retry leaked an exclusive-install stage")
						}
					}
				}
				if _, err := repo.aggregateHistoryUsage(); err != nil {
					t.Fatal(err)
				}
				if err := repo.validateAggregateLedger(current.AggregateOutcome); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(repo.auth.GitDir, "corvint/local-completion/operation.lock")); !os.IsNotExist(err) {
					t.Fatal("normal cancellation retained lock", err)
				}
			}
		})
	}
}

func TestAggregateCaptureMarkerRefusalPreservesEvidence(t *testing.T) {
	for _, mode := range []string{"malformed", "foreign-session", "foreign-snapshot", "foreign-state", "foreign-report", "foreign-target", "qualified", "symlink", "capacity"} {
		t.Run(mode, func(t *testing.T) {
			repo, saved, receipt, report := aggregatePublicationFixture(t)
			for _, item := range []struct {
				role string
				raw  []byte
			}{{"outcome", receipt}, {"report", report}} {
				if err := repo.publishAggregateRole(context.Background(), saved, item.role, item.raw); err != nil {
					t.Fatal(err)
				}
			}
			ctx, release, err := dogfoodoperation.Acquire(context.Background(), repo.auth.GitDir)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			snap, err := repo.snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			cancelled, cancel := context.WithCancel(ctx)
			defer cancel()
			repo.aggregateFault = func(at string) error {
				if at == "check-started-closed" {
					cancel()
				}
				return nil
			}
			if _, err := repo.beginAggregateCheckCapture(cancelled, saved, snap, report); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			repo.aggregateFault = nil
			dir := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
			name := filepath.Join(dir, "check.started.json")
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err = json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "malformed":
				raw = []byte("{")
			case "foreign-session":
				value["session"] = "foreign"
			case "foreign-snapshot":
				value["snapshotSha256"] = "sha256:" + strings.Repeat("0", 64)
			case "foreign-state":
				value["stateSha256"] = "sha256:" + strings.Repeat("0", 64)
			case "foreign-report":
				value["reportSha256"] = "sha256:" + strings.Repeat("0", 64)
			case "foreign-target":
				value["target"] = strings.Repeat("0", 40)
			case "qualified":
				value["qualified"] = true
			case "symlink":
				target := filepath.Join(t.TempDir(), "marker")
				if err = os.WriteFile(target, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Remove(name); err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(target, name); err != nil {
					t.Fatal(err)
				}
			case "capacity":
				used, err := repo.aggregateHistoryUsage()
				if err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(repo.aggregateDirectory(), "retained-fault-stage"))
				if err != nil {
					t.Fatal(err)
				}
				if err = f.Truncate(aggregateHistoryLimit - used); err != nil {
					t.Fatal(err)
				}
				if err = f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "symlink" && mode != "capacity" {
				if mode != "malformed" {
					raw, err = aggregateCanonicalJSON(value)
					if err != nil {
						t.Fatal(err)
					}
				}
				if err = os.WriteFile(name, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(repo.local("state.json"))
			if err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = repo.preserveAggregateReplacement(ctx, saved, receipt)
			want := "aggregate-prior-evidence-drift"
			if mode == "capacity" {
				want = "aggregate-history-bound-exceeded"
			}
			if err == nil || err.Error() != want {
				t.Fatalf("%s admitted: %v", mode, err)
			}
			after, err := os.ReadFile(repo.local("state.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal mutated state", err)
			}
			afterEntries, err := os.ReadDir(dir)
			if err != nil || len(afterEntries) != len(entries) {
				t.Fatal("refusal leaked stage", err)
			}
			got, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(raw, got) {
				t.Fatal("refusal changed marker", err)
			}
			if _, err = os.Stat(repo.aggregateReservationPath(2)); !os.IsNotExist(err) {
				t.Fatal("refusal consumed ordinal", err)
			}
		})
	}
}

func TestAggregateStateAllMemberShapes(t *testing.T) {
	repo, saved, proposed, receipt := aggregateStorageFixture(t)
	if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(repo.local("state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.load(); err != nil {
		t.Fatalf("control: %v", err)
	}
	members := []string{"profile", "bindingSha256", "receiptSha256", "legacyFailureSha256", "attemptCount", "phase", "activeSnapshotSha256", "preservationState", "expectedOld", "issued", "snapshots"}
	for _, member := range members {
		for _, shape := range []string{"missing", "null", "wrong-type"} {
			t.Run(member+"/"+shape, func(t *testing.T) {
				var value map[string]any
				if err := json.Unmarshal(original, &value); err != nil {
					t.Fatal(err)
				}
				a := value["aggregateOutcome"].(map[string]any)
				switch shape {
				case "missing":
					delete(a, member)
				case "null":
					a[member] = nil
				case "wrong-type":
					a[member] = true
				}
				mutant, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(repo.local("state.json"), mutant, 0600); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.WriteFile(repo.local("state.json"), original, 0600); err != nil {
						t.Error(err)
					}
				}()
				if _, err = repo.load(); err == nil {
					t.Fatal("closed aggregate state mutation accepted")
				}
				after, err := os.ReadFile(repo.local("state.json"))
				if err != nil || !bytes.Equal(after, mutant) {
					t.Fatal("refusing reader mutated state")
				}
			})
		}
	}
}

func aggregateQualificationFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !e.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			result[rel] = prefixedDigest(raw)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAggregateQualificationOrphanGapAndMissingState(t *testing.T) {
	for _, shape := range []string{"nonconsecutive-orphan", "missing-original-state-before", "conflicting-original-reservation"} {
		t.Run(shape, func(t *testing.T) {
			repo, saved, proposed, receipt := aggregateStorageFixture(t)
			point := "reservation-installed"
			if shape == "missing-original-state-before" {
				point = "reservation-state-saved"
			}
			repo.aggregateFault = func(at string) error {
				if at == point {
					return errors.New("qualification-stop")
				}
				return nil
			}
			if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err == nil || err.Error() != "qualification-stop" {
				t.Fatalf("boundary: %v", err)
			}
			repo.aggregateFault = nil
			switch shape {
			case "nonconsecutive-orphan":
				if err := os.Rename(repo.aggregateReservationPath(1), repo.aggregateReservationPath(2)); err != nil {
					t.Fatal(err)
				}
			case "missing-original-state-before":
				if err := os.Remove(filepath.Join(repo.aggregateSnapshotDirectory(proposed.BindingSHA256, proposed.ActiveSnapshotSHA256), "state-before.bin")); err != nil {
					t.Fatal(err)
				}
			case "conflicting-original-reservation":
				raw, err := os.ReadFile(repo.aggregateReservationPath(1))
				if err != nil {
					t.Fatal(err)
				}
				var reservation aggregateReservation
				if err = json.Unmarshal(raw, &reservation); err != nil {
					t.Fatal(err)
				}
				reservation.MaximumBytes++
				raw, err = aggregateCanonicalJSON(reservation)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(repo.aggregateReservationPath(1), append(raw, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := aggregateQualificationFiles(t, repo.directory)
			other, err := open(repo.auth.Root, repo.session)
			if err != nil {
				t.Fatal(err)
			}
			current, err := other.load()
			if err == nil {
				err = other.preserveAggregate(context.Background(), current, proposed, receipt)
			}
			if err == nil {
				t.Fatal("unsafe recovery accepted")
			}
			if after := aggregateQualificationFiles(t, repo.directory); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed evidence: before=%v after=%v", before, after)
			}
		})
	}
}

func TestAggregateQualificationMixedCapacity(t *testing.T) {
	repo, saved, proposed, receipt := aggregateStorageFixture(t)
	if err := repo.preserveAggregate(context.Background(), saved, proposed, receipt); err != nil {
		t.Fatal(err)
	}
	// Preserve first sealed payloads; add a second genuine consecutive orphan
	// reservation and independently retained staging/HOLD diagnostics.
	original := aggregateQualificationFiles(t, repo.aggregateDirectory())
	if err := os.WriteFile(repo.aggregateSourcePaths()["check-stdout"], []byte("changed check attempt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	repo.aggregateFault = func(at string) error {
		if at == "reservation-installed" {
			return errors.New("qualification-orphan")
		}
		return nil
	}
	next := *proposed
	if err := repo.reserveAggregateSnapshot(context.Background(), saved, &next, receipt); err == nil || err.Error() != "qualification-orphan" {
		t.Fatalf("second orphan: %v", err)
	}
	stage := filepath.Join(repo.aggregateDirectory(), "retained-stage")
	hold := filepath.Join(repo.aggregateDirectory(), "retained-hold.json")
	if err := os.WriteFile(stage, []byte("retained stage bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hold, []byte(`{"state":"HOLD","qualified":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	used, err := repo.aggregateHistoryUsage()
	if err != nil {
		t.Fatal(err)
	}
	padding := filepath.Join(repo.aggregateDirectory(), "retained-fault-padding")
	f, err := os.Create(padding)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(aggregateHistoryLimit - used); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if total, err := repo.aggregateHistoryUsage(); err != nil || total != aggregateHistoryLimit {
		t.Fatalf("mixed exact=%d %v", total, err)
	}
	if err = os.Truncate(padding, aggregateHistoryLimit-used+1); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.aggregateHistoryUsage(); err == nil || err.Error() != "aggregate-history-bound-exceeded" {
		t.Fatalf("mixed +1: %v", err)
	}
	after := aggregateQualificationFiles(t, repo.aggregateDirectory())
	for path, digest := range original {
		if after[path] != digest {
			t.Fatalf("evicted/rewrote first snapshot %s", path)
		}
	}
	if _, err = os.Stat(repo.aggregateReservationPath(2)); err != nil {
		t.Fatal("original orphan lost", err)
	}
}

func TestAggregateQualificationPendingNativeReaders(t *testing.T) {
	current := os.Getenv("CORVINT_QUAL_CURRENT_BINARY")
	old := os.Getenv("CORVINT_QUAL_PREDECESSOR_BINARY")
	if current == "" || old == "" {
		t.Skip("requires pinned private current and original-main binaries")
	}
	evidenceRoot := os.Getenv("CORVINT_QUAL_READER_EVIDENCE")
	if evidenceRoot == "" {
		t.Fatal("missing fresh private per-reader evidence destination")
	}
	probe := func(t *testing.T, repo *repository, want aggregatePendingReaderBinding, phase string) {
		t.Helper()
		stateRaw, err := os.ReadFile(repo.local("state.json"))
		if err != nil {
			t.Fatal(err)
		}
		var observed struct {
			Aggregate *struct {
				Phase string `json:"phase"`
			} `json:"aggregateOutcome"`
		}
		if err := json.Unmarshal(stateRaw, &observed); err != nil {
			t.Fatal(err)
		}
		if phase == "legacy" {
			if observed.Aggregate != nil {
				t.Fatal("unexpected aggregate discriminator")
			}
		} else if observed.Aggregate == nil || observed.Aggregate.Phase != phase {
			t.Fatalf("fixture did not reach expected aggregate phase %s: %s", phase, stateRaw)
		}
		before := aggregateQualificationFiles(t, filepath.Join(repo.auth.GitDir, "corvint"))
		reportBefore := aggregateQualificationFiles(t, filepath.Join(repo.auth.Root, ".corvint"))
		for reader, binary := range []string{current, old} {
			for _, action := range []string{"status", "event"} {
				args := []string{"dogfood", action, "--session-key", repo.session}
				if action == "event" {
					args = []string{"dogfood", "event", "--host", "codex", "--host-version", "unknown", "--surface", "plugin", "--adapter-version", "0.1.0", "--event", "stop", "--input", "-", "--budget-bytes", "8000"}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				cmd := exec.CommandContext(ctx, binary, args...)
				cmd.Dir = repo.auth.Root
				cmd.Stdin = strings.NewReader(`{"sessionIdSha256":"` + repo.session + `"}`)
				cmd.WaitDelay = time.Second
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				deadlineErr := ctx.Err()
				cancel()
				exitCode := 0
				if err != nil {
					var exit *exec.ExitError
					if errors.As(err, &exit) {
						exitCode = exit.ExitCode()
					} else {
						exitCode = -1
					}
				}
				// Create-only artifacts prevent a rerun from replacing failed evidence.
				destination := filepath.Join(evidenceRoot, t.Name(), fmt.Sprintf("%d-%s", reader, action))
				if err := os.MkdirAll(destination, 0700); err != nil {
					t.Fatal(err)
				}
				meta, _ := json.Marshal(map[string]any{"binary": binary, "args": args, "phase": phase, "exit": exitCode, "runError": fmt.Sprint(err), "deadlineError": fmt.Sprint(deadlineErr), "binding": want})
				for name, raw := range map[string][]byte{"stdout": stdout.Bytes(), "stderr": stderr.Bytes(), "result.json": meta} {
					f, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						t.Fatal(err)
					}
					_, writeErr := f.Write(raw)
					closeErr := f.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatalf("retain reader evidence: %v %v", writeErr, closeErr)
					}
				}
				if deadlineErr != nil {
					t.Fatalf("reader deadline is not qualification: %v", deadlineErr)
				}
				if err := aggregatePendingReaderOracle(action, reader == 1, phase, want, exitCode, stdout.Bytes(), stderr.Bytes()); err != nil {
					t.Fatalf("reader %s/%s: %v (raw evidence %s)", binary, action, err, destination)
				}
				if !reflect.DeepEqual(before, aggregateQualificationFiles(t, filepath.Join(repo.auth.GitDir, "corvint"))) || !reflect.DeepEqual(reportBefore, aggregateQualificationFiles(t, filepath.Join(repo.auth.Root, ".corvint"))) {
					t.Fatalf("reader %s/%s changed private evidence", binary, action)
				}
			}
		}
	}
	points := []string{"reservation-installed", "state-before-closed", "reservation-state-saved", "snapshot-manifest-installed", "snapshot-sealed-acknowledged"}
	for _, role := range aggregateSnapshotRoles {
		points = append(points, "payload-closed:"+role)
	}
	for _, point := range points {
		t.Run("preservation/"+point, func(t *testing.T) {
			repo, saved, proposed, _ := aggregateStorageFixture(t)
			want := aggregatePendingReaderFixtureBinding(t, repo, saved)
			phase := "PREPARED"
			if point == "reservation-installed" || point == "state-before-closed" {
				phase = "legacy"
			}
			raw, err := json.Marshal(proposed)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAggregatePreservationProcessHelper$")
			cmd.Env = append(os.Environ(), "CORVINT_AGGREGATE_STORAGE_TEST_ROOT="+repo.auth.Root, "CORVINT_AGGREGATE_STORAGE_TEST_KEY="+repo.session, "CORVINT_AGGREGATE_STORAGE_TEST_PROPOSAL="+string(raw), "CORVINT_AGGREGATE_STORAGE_TEST_CRASH="+point)
			cmd.WaitDelay = time.Second
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 88 {
				t.Fatalf("boundary not reached: %v %s", err, out)
			}
			probe(t, repo, want, phase)
		})
	}
	for _, role := range []string{"outcome", "report"} {
		for _, boundary := range []string{"prepared-closed:", "stage-closed:", "issued-saved:", "artifact-renamed:", "phase-acknowledged:"} {
			t.Run("publication/"+boundary+role, func(t *testing.T) {
				repo, saved, receipt, report := aggregatePublicationFixture(t)
				want := aggregatePendingReaderFixtureBinding(t, repo, saved)
				phase := "PREPARED"
				if role == "report" || boundary == "phase-acknowledged:" {
					phase = "OUTCOME_PUBLISHED"
				}
				if role == "report" && boundary == "phase-acknowledged:" {
					phase = "REPORT_PUBLISHED"
				}
				raw := receipt
				if role == "report" {
					if err := repo.publishAggregateRole(context.Background(), saved, "outcome", receipt); err != nil {
						t.Fatal(err)
					}
					raw = report
				}
				input := filepath.Join(t.TempDir(), "publication.json")
				if err := os.WriteFile(input, raw, 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAggregatePreservationProcessHelper$")
				cmd.Env = append(os.Environ(), "CORVINT_AGGREGATE_STORAGE_TEST_ROOT="+repo.auth.Root, "CORVINT_AGGREGATE_STORAGE_TEST_KEY="+repo.session, "CORVINT_AGGREGATE_STORAGE_TEST_PROPOSAL={}", "CORVINT_AGGREGATE_STORAGE_TEST_ROLE="+role, "CORVINT_AGGREGATE_STORAGE_TEST_INPUT="+input, "CORVINT_AGGREGATE_STORAGE_TEST_CRASH="+boundary+role)
				cmd.WaitDelay = time.Second
				out, err := cmd.CombinedOutput()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 88 {
					t.Fatalf("boundary not reached: %v %s", err, out)
				}
				probe(t, repo, want, phase)
			})
		}
	}
}

func TestAggregateQualificationActualIOFailures(t *testing.T) {
	for _, boundary := range []string{"write", "close"} {
		t.Run(boundary, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.stdout")
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			writer := &aggregateCaptureWriter{file: file}
			prefix := []byte("retained observed prefix\n")
			if _, err = writer.Write(prefix); err != nil {
				t.Fatal(err)
			}
			// Closing the real descriptor injects EBADF at the actual next kernel
			// write/close boundary; no callback synthesizes the production error.
			if err = file.Close(); err != nil {
				t.Fatal(err)
			}
			if boundary == "write" {
				if n, err := writer.Write([]byte("must-not-append")); err == nil || n != 0 {
					t.Fatalf("closed-descriptor write=%d %v", n, err)
				}
			}
			raw, err := writer.close()
			if err == nil || !bytes.Equal(raw, prefix) {
				t.Fatalf("capture close must retain prefix and error: %q %v", raw, err)
			}
			disk, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(disk, prefix) {
				t.Fatalf("I/O refusal lost prior bytes: %v", err)
			}
		})
	}
	t.Run("rename", func(t *testing.T) {
		repo, saved, receipt, _ := aggregatePublicationFixture(t)
		destination := repo.aggregateSourcePaths()["prior-outcome"]
		before, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		parent := filepath.Dir(destination)
		info, err := os.Stat(parent)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(parent, info.Mode().Perm())
		reached := false
		repo.aggregateFault = func(at string) error {
			if at == "issued-saved:outcome" {
				reached = true
				return os.Chmod(parent, 0500)
			}
			return nil
		}
		err = repo.publishAggregateRole(context.Background(), saved, "outcome", receipt)
		var link *os.LinkError
		if !reached || !errors.As(err, &link) || link.Op != "rename" {
			t.Fatalf("actual rename boundary not refused: reached=%v err=%v", reached, err)
		}
		after, readErr := os.ReadFile(destination)
		if readErr != nil || !bytes.Equal(before, after) {
			t.Fatal("rename refusal changed prior artifact", readErr)
		}
		if saved.AggregateOutcome.Phase != "PREPARED" {
			t.Fatal("rename failure acknowledged publication")
		}
		if err := os.Chmod(parent, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		if sealed, err := repo.aggregatePreservation(saved.AggregateOutcome); err != nil || !sealed {
			t.Fatalf("rename refusal lost sealed source: %v %v", sealed, err)
		}
	})
}

func TestAggregateQualificationSelectors(t *testing.T) {
	repo, _, _, _ := aggregatePublicationFixture(t)
	for _, profile := range []string{"", "corvint-dogfood-aggregate-outcome/foreign"} {
		t.Run(profile, func(t *testing.T) {
			before := aggregateQualificationFiles(t, repo.directory)
			_, err := FinishWithAggregateProfile(context.Background(), repo.auth.Root, repo.session, profile, func(context.Context, string, []string, io.Writer, io.Writer) int {
				t.Fatal("refused selector invoked command")
				return 99
			})
			want := "aggregate-enrollment-required"
			if profile != "" {
				want = "aggregate-profile-unsupported"
			}
			if err == nil || err.Error() != want {
				t.Fatalf("selector refusal: %v want %s", err, want)
			}
			if !reflect.DeepEqual(before, aggregateQualificationFiles(t, repo.directory)) {
				t.Fatal("selector refusal changed history/state")
			}
		})
	}
}

func TestAggregateStateNestedMemberShapes(t *testing.T) {
	repo, saved, receipt, _ := aggregatePublicationFixture(t)
	if err := repo.publishAggregateRole(context.Background(), saved, "outcome", receipt); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(repo.local("state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.load(); err != nil {
		t.Fatal(err)
	}
	groups := map[string][]string{"expectedOld": {"outcome", "report"}, "outcome": {"sha256", "bytes", "kind"}, "report": {"sha256", "bytes", "kind"}, "issued": {"role", "sha256", "bytes"}, "snapshots": {"ordinal", "snapshotSha256", "reservedBytes"}}
	for group, members := range groups {
		for _, member := range members {
			for _, shape := range []string{"missing", "null", "wrong-type"} {
				t.Run(group+"/"+member+"/"+shape, func(t *testing.T) {
					var value map[string]any
					if err := json.Unmarshal(original, &value); err != nil {
						t.Fatal(err)
					}
					a := value["aggregateOutcome"].(map[string]any)
					var selected map[string]any
					switch group {
					case "expectedOld":
						selected = a[group].(map[string]any)
					case "outcome", "report":
						selected = a["expectedOld"].(map[string]any)[group].(map[string]any)
					case "issued", "snapshots":
						selected = a[group].([]any)[0].(map[string]any)
					}
					switch shape {
					case "missing":
						delete(selected, member)
					case "null":
						selected[member] = nil
					case "wrong-type":
						selected[member] = true
					}
					mutant, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					if err = os.WriteFile(repo.local("state.json"), mutant, 0600); err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := os.WriteFile(repo.local("state.json"), original, 0600); err != nil {
							t.Error(err)
						}
					}()
					if _, err = repo.load(); err == nil {
						t.Fatal("nested state substitution accepted")
					}
					after, err := os.ReadFile(repo.local("state.json"))
					if err != nil || !bytes.Equal(after, mutant) {
						t.Fatal("refusal wrote state")
					}
				})
			}
		}
	}
}

func TestAggregateQualificationCommittedMixedCapacityHelper(t *testing.T) {
	root := os.Getenv("CORVINT_QUAL_COMMITTED_ROOT")
	key := os.Getenv("CORVINT_QUAL_COMMITTED_KEY")
	if root == "" || key == "" {
		t.Skip("requires actual successful native Finish fixture")
	}
	repo, err := open(root, key)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.AggregateOutcome == nil || saved.AggregateOutcome.Phase != "COMMITTED" || saved.Terminal == nil || saved.Lifecycle != "satisfied" {
		t.Fatal("fixture is not actually committed")
	}
	original := aggregateQualificationFiles(t, repo.aggregateDirectory())
	stateBefore, err := os.ReadFile(repo.local("state.json"))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(repo.aggregateSourcePaths()["prior-outcome"])
	if err != nil {
		t.Fatal(err)
	}
	nextOrdinal := saved.AggregateOutcome.AttemptCount + 1
	if nextOrdinal != 2 {
		t.Fatalf("expected fresh actual native history, next=%d", nextOrdinal)
	}
	repo.aggregateFault = func(at string) error {
		if at == "reservation-installed" {
			return errors.New("qualification-consecutive-orphan")
		}
		return nil
	}
	proposed := *saved.AggregateOutcome
	if err := repo.reserveAggregateSnapshot(context.Background(), saved, &proposed, receipt); err == nil || err.Error() != "qualification-consecutive-orphan" {
		t.Fatalf("reserve actual orphan: %v", err)
	}
	stage := filepath.Join(repo.auth.Root, ".corvint", repo.aggregateStagePrefix(nextOrdinal)+"qualification")
	hold := filepath.Join(repo.aggregateDirectory(), "retained-qualification-hold.json")
	padding := filepath.Join(repo.aggregateDirectory(), "retained-qualification-padding")
	t.Cleanup(func() {
		for _, path := range []string{stage, hold, padding, repo.aggregateReservationPath(nextOrdinal)} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Error(err)
			}
		}
	})
	if err := os.WriteFile(stage, []byte("actual namespace retained publication stage\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hold, []byte(`{"state":"HOLD","qualified":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	used, err := repo.aggregateHistoryUsage()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(padding)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(aggregateHistoryLimit - used); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.aggregateHistoryUsage(); err != nil || got != aggregateHistoryLimit {
		t.Fatalf("committed mixed exact=%d %v", got, err)
	}
	if err = os.Truncate(padding, aggregateHistoryLimit-used+1); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.aggregateHistoryUsage(); err == nil || err.Error() != "aggregate-history-bound-exceeded" {
		t.Fatalf("committed mixed +1: %v", err)
	}
	after := aggregateQualificationFiles(t, repo.aggregateDirectory())
	for path, digest := range original {
		if after[path] != digest {
			t.Fatalf("changed committed history %s", path)
		}
	}
	stateAfter, err := os.ReadFile(repo.local("state.json"))
	if err != nil || !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("capacity probe changed committed state")
	}
	t.Logf("actual COMMITTED ordinal1 + consecutive orphan2 + owned publication stage + retained HOLD: used=%d exact=%d plus-one refused; no eviction", used, aggregateHistoryLimit)
}

type aggregatePendingReaderBinding struct{ Base, Target, Tree, Plan, Request string }

func aggregatePendingReaderFixtureBinding(t *testing.T, repo *repository, saved *state) aggregatePendingReaderBinding {
	t.Helper()
	request := sha256.Sum256([]byte(`{"sessionIdSha256":"` + repo.session + `"}`))
	return aggregatePendingReaderBinding{saved.Plan.Base, localGit(t, repo.auth.Root, "rev-parse", "HEAD"), localGit(t, repo.auth.Root, "rev-parse", "HEAD^{tree}"), saved.PlanDigest, fmt.Sprintf("%x", request)}
}

func aggregatePendingReaderOracle(action string, predecessor bool, phase string, want aggregatePendingReaderBinding, exit int, stdout, stderr []byte) error {
	if action != "status" && action != "event" {
		return fmt.Errorf("unknown action")
	}
	if phase != "legacy" && phase != "PREPARED" && phase != "OUTCOME_PUBLISHED" && phase != "REPORT_PUBLISHED" {
		return fmt.Errorf("unknown phase")
	}
	if predecessor && phase != "legacy" {
		code := "invalid-local-completion-schema"
		if action == "event" {
			code = "dogfood-event-unavailable"
		}
		var refusal struct {
			OK    *bool           `json:"ok"`
			Code  string          `json:"code"`
			Error json.RawMessage `json:"error"`
		}
		if exit != 2 || len(bytes.TrimSpace(stdout)) != 0 || json.Unmarshal(stderr, &refusal) != nil || refusal.OK == nil || *refusal.OK || refusal.Code != code {
			return fmt.Errorf("expected specific predecessor refusal %s", code)
		}
		if action == "status" {
			var detail struct{ Code, Message string }
			if json.Unmarshal(refusal.Error, &detail) != nil || detail.Code != code || detail.Message != code {
				return fmt.Errorf("invalid status refusal detail")
			}
		} else {
			var detail string
			if json.Unmarshal(refusal.Error, &detail) != nil || detail != code {
				return fmt.Errorf("invalid event refusal detail")
			}
		}
		// Event masks the cause. Its paired status probe on these unchanged files
		// must pass first; this event alone never proves aggregate-version refusal.
		return nil
	}
	var value struct {
		OK                                                  *bool `json:"ok"`
		Mutates                                             *bool `json:"mutates"`
		Profile, Tool, Claim, Support, Event, RequestSha256 string
		Policy                                              *struct {
			Satisfied                           *bool `json:"satisfied"`
			Lifecycle, Base, Target, PlanDigest string
			Unmet                               []string
		} `json:"policy"`
		Adapter    struct{ Host, HostVersion, Surface, AdapterVersion string }
		Repository struct{ CommitRevision, TreeRevision string }
		Completion struct{ Decision, Reason string }
	}
	if exit != 0 || len(bytes.TrimSpace(stderr)) != 0 || json.Unmarshal(stdout, &value) != nil || value.OK == nil || !*value.OK || value.Mutates == nil || *value.Mutates || value.Policy == nil || value.Policy.Satisfied == nil || *value.Policy.Satisfied {
		return fmt.Errorf("missing explicit successful unsatisfied policy")
	}
	p := value.Policy
	if want.Base == "" || want.Target == "" || want.Plan == "" || p.Lifecycle != "active" || p.Base != want.Base || p.Target != want.Target || p.PlanDigest != want.Plan {
		return fmt.Errorf("pending policy binding mismatch")
	}
	finalRequired := false
	for _, reason := range p.Unmet {
		if reason == "final-check-required" {
			finalRequired = true
		}
	}
	if !finalRequired {
		return fmt.Errorf("pending final-check requirement missing")
	}
	if action == "status" {
		if value.Profile != "corvint-local-completion/0" || value.Tool != "dogfood-status" || value.Claim != "caller-owned-selected-workflow-only" {
			return fmt.Errorf("status envelope mismatch")
		}
	} else if value.Profile != "corvint-dogfood-event/0" || value.Support != "FALLBACK" || value.Event != "stop" || value.RequestSha256 != want.Request || want.Request == "" || value.Adapter.Host != "codex" || value.Adapter.HostVersion != "unknown" || value.Adapter.Surface != "plugin" || value.Adapter.AdapterVersion != "0.1.0" || value.Repository.CommitRevision != want.Target || value.Repository.TreeRevision != want.Tree || want.Tree == "" || value.Completion.Decision != "block" || value.Completion.Reason != "local-policy-incomplete" {
		return fmt.Errorf("event envelope binding mismatch")
	}
	return nil
}

func TestAggregatePendingReaderOracle(t *testing.T) {
	want := aggregatePendingReaderBinding{"base", "target", "tree", "plan", "request"}
	for _, action := range []string{"status", "event"} {
		success := map[string]any{"ok": true, "mutates": false, "profile": "corvint-local-completion/0", "tool": "dogfood-status", "claim": "caller-owned-selected-workflow-only", "policy": map[string]any{"satisfied": false, "lifecycle": "active", "base": "base", "target": "target", "planDigest": "plan", "unmet": []string{"final-check-required"}}}
		if action == "event" {
			success["profile"] = "corvint-dogfood-event/0"
			success["support"], success["event"], success["requestSha256"] = "FALLBACK", "stop", "request"
			success["adapter"] = map[string]any{"host": "codex", "hostVersion": "unknown", "surface": "plugin", "adapterVersion": "0.1.0"}
			success["repository"] = map[string]any{"commitRevision": "target", "treeRevision": "tree"}
			success["completion"] = map[string]any{"decision": "block", "reason": "local-policy-incomplete"}
		}
		raw, _ := json.Marshal(success)
		for _, phase := range []string{"legacy", "PREPARED", "OUTCOME_PUBLISHED", "REPORT_PUBLISHED"} {
			if err := aggregatePendingReaderOracle(action, false, phase, want, 0, raw, nil); err != nil {
				t.Fatal(err)
			}
			err := aggregatePendingReaderOracle(action, true, phase, want, 0, raw, nil)
			if (err == nil) != (phase == "legacy") {
				t.Fatalf("predecessor acceptance at %s: %v", phase, err)
			}
		}
		mutants := map[string][]byte{"null": []byte("null"), "empty": []byte("{}"), "malformed": []byte("{")}
		for _, mutation := range []string{"missing-policy", "null-policy", "missing-satisfied", "null-satisfied", "true-satisfied", "lifecycle", "base", "target", "planDigest", "profile", "ok", "mutates", "unmet"} {
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			p := m["policy"].(map[string]any)
			switch mutation {
			case "missing-policy":
				delete(m, "policy")
			case "null-policy":
				m["policy"] = nil
			case "missing-satisfied":
				delete(p, "satisfied")
			case "null-satisfied":
				p["satisfied"] = nil
			case "true-satisfied":
				p["satisfied"] = true
			case "profile":
				m["profile"] = "wrong"
			case "ok", "mutates":
				delete(m, mutation)
			case "unmet":
				p["unmet"] = []string{}
			default:
				p[mutation] = "wrong"
			}
			mutants[mutation], _ = json.Marshal(m)
		}
		if action == "event" {
			for _, field := range []string{"requestSha256", "adapter", "repository", "completion"} {
				var m map[string]any
				_ = json.Unmarshal(raw, &m)
				delete(m, field)
				mutants[field], _ = json.Marshal(m)
			}
		}
		for name, mutant := range mutants {
			t.Run(action+"/"+name, func(t *testing.T) {
				if err := aggregatePendingReaderOracle(action, false, "PREPARED", want, 0, mutant, nil); err == nil {
					t.Fatal("accepted malformed/unbound response")
				}
			})
		}
		code := "invalid-local-completion-schema"
		var detail any = map[string]string{"code": code, "message": code}
		if action == "event" {
			code = "dogfood-event-unavailable"
			detail = code
		}
		refusal, _ := json.Marshal(map[string]any{"ok": false, "code": code, "error": detail})
		if err := aggregatePendingReaderOracle(action, true, "PREPARED", want, 2, nil, refusal); err != nil {
			t.Fatal(err)
		}
		if err := aggregatePendingReaderOracle(action, false, "PREPARED", want, 2, nil, refusal); err == nil {
			t.Fatal("current reader refusal qualified")
		}
		if err := aggregatePendingReaderOracle(action, true, "legacy", want, 2, nil, refusal); err == nil {
			t.Fatal("always-unavailable predecessor qualified")
		}
		for _, bad := range [][]byte{[]byte(`{"ok":false,"code":"usage","error":"usage"}`), []byte(`{"ok":false}`), []byte(`null`), []byte(`{`)} {
			if err := aggregatePendingReaderOracle(action, true, "PREPARED", want, 2, nil, bad); err == nil {
				t.Fatal("unrelated exit 2 qualified")
			}
		}
	}
}

// Active expectedOld is evidence to validate, not a value to reconstruct from
// history. Real absence stays valid, including after publication and replacement.
func TestAggregateExpectedOldMatchesActiveReservation(t *testing.T) {
	for _, outcomePresent := range []bool{false, true} {
		for _, reportPresent := range []bool{false, true} {
			t.Run(fmt.Sprintf("outcome-present-%t/report-present-%t", outcomePresent, reportPresent), func(t *testing.T) {
				repo, saved, receipt, report := aggregatePublicationFixtureWithOldArtifacts(t, outcomePresent, reportPresent)
				first, err := os.ReadFile(repo.aggregateReservationPath(1))
				if err != nil {
					t.Fatal(err)
				}
				if (saved.AggregateOutcome.ExpectedOld.Outcome != nil) != outcomePresent || (saved.AggregateOutcome.ExpectedOld.Report != nil) != reportPresent {
					t.Fatal("fixture did not capture actual original presence")
				}
				check := func(stage string) {
					t.Helper()
					t.Run(stage, func(t *testing.T) {
						before := aggregateQualificationFiles(t, repo.auth.Root)
						current, err := repo.load()
						if err != nil || !reflect.DeepEqual(current.AggregateOutcome.ExpectedOld, saved.AggregateOutcome.ExpectedOld) {
							t.Fatalf("valid active reservation refused or changed: %v", err)
						}
						wantPhase := map[string]string{"prepared": "PREPARED", "outcome-published": "OUTCOME_PUBLISHED", "report-published": "REPORT_PUBLISHED", "second-reservation": "REPORT_PUBLISHED"}[stage]
						if current.AggregateOutcome.Phase != wantPhase {
							t.Fatalf("fixture phase %s, want %s", current.AggregateOutcome.Phase, wantPhase)
						}
						if !reflect.DeepEqual(before, aggregateQualificationFiles(t, repo.auth.Root)) {
							t.Fatal("valid load wrote evidence")
						}
						original, err := os.ReadFile(repo.local("state.json"))
						if err != nil {
							t.Fatal(err)
						}
						for _, role := range []string{"outcome", "report"} {
							t.Run(role+"-presence-substitution", func(t *testing.T) {
								var value map[string]any
								if err := json.Unmarshal(original, &value); err != nil {
									t.Fatal(err)
								}
								old := value["aggregateOutcome"].(map[string]any)["expectedOld"].(map[string]any)
								if old[role] == nil {
									kind := "legacy-report"
									if role == "outcome" {
										kind = "failed-recorder-output"
									}
									old[role] = &aggregateOld{prefixedDigest([]byte("foreign")), 7, kind}
								} else {
									old[role] = nil
								}
								mutant, err := json.Marshal(value)
								if err != nil {
									t.Fatal(err)
								}
								if err = os.WriteFile(repo.local("state.json"), mutant, 0600); err != nil {
									t.Fatal(err)
								}
								defer func() {
									if err := os.WriteFile(repo.local("state.json"), original, 0600); err != nil {
										t.Error(err)
									}
								}()
								before := aggregateQualificationFiles(t, repo.auth.Root)
								if _, err := repo.load(); err == nil {
									t.Fatal("active expectedOld substitution accepted")
								}
								if !reflect.DeepEqual(before, aggregateQualificationFiles(t, repo.auth.Root)) {
									t.Fatal("refusal wrote evidence")
								}
							})
						}
					})
				}
				check("prepared")
				if err := repo.publishAggregateRole(context.Background(), saved, "outcome", receipt); err != nil {
					t.Fatal(err)
				}
				check("outcome-published")
				if err := repo.publishAggregateRole(context.Background(), saved, "report", report); err != nil {
					t.Fatal(err)
				}
				check("report-published")
				directory := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
				if err := writeAggregateExclusive(filepath.Join(directory, "check.stdout"), []byte("failed capture prefix")); err != nil {
					t.Fatal(err)
				}
				if err := repo.preserveAggregateReplacement(context.Background(), saved, receipt); err != nil {
					t.Fatal(err)
				}
				if saved.AggregateOutcome.AttemptCount != 2 || saved.AggregateOutcome.ExpectedOld.Outcome == nil || saved.AggregateOutcome.ExpectedOld.Report == nil {
					t.Fatal("missing second active reservation")
				}
				check("second-reservation")
				after, err := os.ReadFile(repo.aggregateReservationPath(1))
				if err != nil || !bytes.Equal(first, after) {
					t.Fatal("historical reservation changed", err)
				}
			})
		}
	}
}
