package gokernel

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

const (
	testCommitRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testTreeRevision   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func testIdentityOutput() []byte {
	return []byte("sha1\n" + testCommitRevision + "\n" + testTreeRevision + "\n")
}

// recordingRunner records every Git observation the probe issues. The two
// observations that make up one side of the nonmutation bracket are issued
// concurrently, so the recorder is mutex-guarded and callers compare each side
// as a set.
func recordingRunner(reply func(string) ([]byte, error)) (func(context.Context, string, int, ...string) ([]byte, error), func() []string) {
	var mutex sync.Mutex
	var calls []string
	run := func(_ context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
		mutex.Lock()
		calls = append(calls, arguments[0])
		mutex.Unlock()
		return reply(arguments[0])
	}
	observed := func() []string {
		mutex.Lock()
		defer mutex.Unlock()
		return slices.Clone(calls)
	}
	return run, observed
}

func sortedSide(calls []string, from, to int) []string {
	side := slices.Clone(calls[from:to])
	slices.Sort(side)
	return side
}

// TestProbeBracketsProfileReadBetweenBothObservations pins GPK-V0-007: the
// profile read is issued only after a complete identity+status observation, and
// a second complete identity+status observation is taken only after it returns.
// The bracket is what proves the probe left the repository unchanged, so it must
// stay strictly sequential even though each side's two reads are concurrent.
func TestProbeBracketsProfileReadBetweenBothObservations(t *testing.T) {
	run, observed := recordingRunner(func(command string) ([]byte, error) {
		switch command {
		case "rev-parse":
			return testIdentityOutput(), nil
		case "status", "ls-tree":
			return []byte{}, nil
		default:
			return nil, newError("test-command", "unexpected Git command")
		}
	})

	_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
	if err != nil {
		t.Fatal(err)
	}
	calls := observed()
	if len(calls) != 5 {
		t.Fatalf("Git observations = %v, want 5", calls)
	}
	side := []string{"rev-parse", "status"}
	if !slices.Equal(sortedSide(calls, 0, 2), side) {
		t.Fatalf("observation before profile read = %v, want %v", calls[0:2], side)
	}
	if calls[2] != "ls-tree" {
		t.Fatalf("bracketed read = %q, want ls-tree", calls[2])
	}
	if !slices.Equal(sortedSide(calls, 3, 5), side) {
		t.Fatalf("observation after profile read = %v, want %v", calls[3:5], side)
	}
}

func TestProfileFailureStopsBeforeSecondObservation(t *testing.T) {
	run, observed := recordingRunner(func(command string) ([]byte, error) {
		switch command {
		case "rev-parse":
			return testIdentityOutput(), nil
		case "status":
			return []byte{}, nil
		case "ls-tree":
			return nil, newError("profile-failed", "profile failed")
		default:
			return nil, newError("test-command", "unexpected Git command")
		}
	})

	_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
	if kernelCode(err) != "profile-failed" {
		t.Fatalf("error = %v, want profile-failed", err)
	}
	calls := observed()
	if len(calls) != 3 {
		t.Fatalf("Git calls after profile failure = %v, want 3", calls)
	}
	side := []string{"rev-parse", "status"}
	if !slices.Equal(sortedSide(calls, 0, 2), side) || calls[2] != "ls-tree" {
		t.Fatalf("Git calls after profile failure = %v", calls)
	}
}

// TestIdentityFailureOutranksStatusFailure pins the oracle's error precedence:
// the oracle reads the identity first, so when both reads of one observation
// fail the identity error is the one reported.
func TestIdentityFailureOutranksStatusFailure(t *testing.T) {
	run := func(_ context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "rev-parse":
			return nil, newError("identity-failed", "identity failed")
		case "status":
			return nil, newError("status-failed", "status failed")
		default:
			return nil, newError("test-command", "unexpected Git command")
		}
	}

	_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
	if kernelCode(err) != "identity-failed" {
		t.Fatalf("error = %v, want identity-failed", err)
	}
}

func TestProbeUsesOneAggregateDeadlineAcrossRetries(t *testing.T) {
	var mutex sync.Mutex
	deadline := time.Time{}
	statusCalls := 0
	run := func(ctx context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
		observed, ok := ctx.Deadline()
		if !ok {
			return nil, newError("test-deadline-missing", "Git probe context has no deadline")
		}
		mutex.Lock()
		if deadline.IsZero() {
			deadline = observed
		} else if !deadline.Equal(observed) {
			mutex.Unlock()
			return nil, newError("test-deadline-changed", "Git probe deadline changed across calls")
		}
		switch arguments[0] {
		case "rev-parse":
			mutex.Unlock()
			return testIdentityOutput(), nil
		case "ls-tree":
			mutex.Unlock()
			return []byte{}, nil
		case "status":
			statusCalls++
			call := statusCalls
			mutex.Unlock()
			if call == 1 {
				return []byte{}, nil
			}
			return []byte("?? changed\x00"), nil
		default:
			mutex.Unlock()
			return nil, newError("test-command", "unexpected Git command")
		}
	}

	_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
	if err != nil {
		t.Fatal(err)
	}
	mutex.Lock()
	gotStatusCalls := statusCalls
	mutex.Unlock()
	if gotStatusCalls != 4 {
		t.Fatalf("status calls = %d, want 4 across one retry", gotStatusCalls)
	}
}

func TestProbePersistentInstabilityStopsAfterThreeAttempts(t *testing.T) {
	var mutex sync.Mutex
	statusCalls := 0
	run := func(_ context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "rev-parse":
			return testIdentityOutput(), nil
		case "ls-tree":
			return []byte{}, nil
		case "status":
			mutex.Lock()
			statusCalls++
			call := statusCalls
			mutex.Unlock()
			if call%2 == 1 {
				return []byte{}, nil
			}
			return []byte("?? changed\x00"), nil
		default:
			return nil, newError("test-command", "unexpected Git command")
		}
	}

	_, err := probeRepositoryContext(context.Background(), t.TempDir(), run)
	if kernelCode(err) != "repository-state-unstable" {
		t.Fatalf("error = %v, want repository-state-unstable", err)
	}
	mutex.Lock()
	gotStatusCalls := statusCalls
	mutex.Unlock()
	if gotStatusCalls != 6 {
		t.Fatalf("status calls = %d, want 6", gotStatusCalls)
	}
}

// TestGitDeadlineIsTenSeconds pins the fixed probe bound by its literal value: the bound tests
// below derive their expectation from gitDeadline itself, so they cannot catch a change to it
// (V1-1414).
func TestGitDeadlineIsTenSeconds(t *testing.T) {
	if gitDeadline != 10*time.Second {
		t.Fatalf("gitDeadline = %s, want the frozen 10s", gitDeadline)
	}
}

// TestProbeBoundReplacesGitDeadline pins the V1-1391 seam: every probe entry point bounds its Git
// reads by WithProbeBound when set, and by the fixed gitDeadline otherwise.
func TestProbeBoundReplacesGitDeadline(t *testing.T) {
	probes := map[string]func(context.Context, gitRunner) error{
		"context": func(ctx context.Context, run gitRunner) error {
			_, err := probeRepositoryContext(ctx, t.TempDir(), run)
			return err
		},
		"around": func(ctx context.Context, run gitRunner) error {
			_, err := probeRepositoryAround(ctx, t.TempDir(), run, nil, func(context.Context, Observation) error { return nil })
			return err
		},
		"sharing": func(ctx context.Context, run gitRunner) error {
			_, err := probeRepositorySharing(ctx, t.TempDir(), run, func(context.Context, Observation) (func() error, error) { return nil, nil }, false)
			return err
		},
	}
	for name, probe := range probes {
		for _, lifted := range []bool{false, true} {
			ctx, bound := context.Background(), gitDeadline
			if lifted {
				ctx, bound = WithProbeBound(ctx, time.Hour), time.Hour
			}
			// The probe's deadline is its start plus the bound, and its start lies between these
			// two instants, so the check holds at any host load.
			start := time.Now()
			var mutex sync.Mutex
			var offending []string
			reads := 0
			run := func(ctx context.Context, _ string, _ int, arguments ...string) ([]byte, error) {
				deadline, ok := ctx.Deadline()
				if !ok {
					return nil, newError("test-deadline-missing", "Git probe context has no deadline")
				}
				mutex.Lock()
				reads++
				if deadline.Before(start.Add(bound)) || deadline.After(time.Now().Add(bound)) {
					offending = append(offending, fmt.Sprintf("%s left of a %s bound", time.Until(deadline), bound))
				}
				mutex.Unlock()
				switch arguments[0] {
				case "rev-parse":
					return testIdentityOutput(), nil
				case "status", "ls-tree":
					return []byte{}, nil
				}
				return nil, newError("test-command", "unexpected Git command")
			}
			if err := probe(ctx, run); err != nil {
				t.Fatalf("%s lifted=%v: %v", name, lifted, err)
			}
			if reads == 0 || len(offending) != 0 {
				t.Fatalf("%s lifted=%v: %d Git reads, deadlines outside the bound: %v", name, lifted, reads, offending)
			}
		}
	}
}
