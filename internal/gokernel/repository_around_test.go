package gokernel

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// TestProbeAroundSpawnsFourGitProcessesAndRefusesDrift pins proposed
// GPK-V0-076: one bracket around the caller's read is one complete
// identity-and-status observation, the read against it, and one complete
// observation -- four Git processes and no `ls-tree` -- where the dogfood
// event surface paid two probes of five. A closing observation that differs
// from the opening one is refused as ErrRepositoryDrift, never retried, and a
// read that fails returns at once, before the closing observation.
func TestProbeAroundSpawnsFourGitProcessesAndRefusesDrift(t *testing.T) {
	stable := func(command string) ([]byte, error) {
		switch command {
		case "rev-parse":
			return testIdentityOutput(), nil
		case "status":
			return []byte{}, nil
		}
		return nil, newError("test-command", "unexpected Git command")
	}
	bracketSides := func(t *testing.T, calls []string) {
		t.Helper()
		want := []string{"rev-parse", "status"}
		if len(calls) != 4 || !slices.Equal(sortedSide(calls, 0, 2), want) || !slices.Equal(sortedSide(calls, 2, 4), want) {
			t.Fatalf("GPK-V0-076: Git processes = %v, want two observations of %v", calls, want)
		}
	}
	t.Run("one-bracket", func(t *testing.T) {
		run, observed := recordingRunner(stable)
		reads := 0
		repository, err := probeRepositoryAround(context.Background(), ".", run, nil, func(_ context.Context, observation Observation) error {
			reads++
			if observation.TreeRevision != testTreeRevision || observation.CommitRevision != testCommitRevision || len(observation.DirtyPaths) != 0 {
				t.Fatalf("read observation = %+v", observation)
			}
			if len(observed()) != 2 {
				t.Fatalf("read issued after %v, want one complete observation", observed())
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		bracketSides(t, observed())
		if reads != 1 || repository.ProfileID != "" || repository.TreeRevision != testTreeRevision || repository.WorktreeState != "clean" {
			t.Fatalf("reads = %d, repository = %+v", reads, repository)
		}
		opening, err := Observation{ObjectFormat: "sha1", CommitRevision: testCommitRevision, TreeRevision: testTreeRevision}.Repository()
		if err != nil || opening != repository {
			t.Fatalf("opening observation as Repository = %+v (%v), closing = %+v", opening, err, repository)
		}
	})
	t.Run("drift-refused-not-retried", func(t *testing.T) {
		statuses := 0
		run, observed := recordingRunner(func(command string) ([]byte, error) {
			if command == "status" {
				statuses++
				if statuses == 2 {
					return []byte("?? notes.txt\x00"), nil
				}
			}
			return stable(command)
		})
		reads := 0
		_, err := probeRepositoryAround(context.Background(), ".", run, nil, func(context.Context, Observation) error {
			reads++
			return nil
		})
		if !errors.Is(err, ErrRepositoryDrift) {
			t.Fatalf("drift inside the bracket: err = %v", err)
		}
		bracketSides(t, observed())
		if reads != 1 {
			t.Fatalf("read ran %d times after drift, want once", reads)
		}
	})
	t.Run("read-error-precedes-closing", func(t *testing.T) {
		run, observed := recordingRunner(stable)
		refused := newError("read-refused", "the read refused")
		_, err := probeRepositoryAround(context.Background(), ".", run, nil, func(context.Context, Observation) error {
			return refused
		})
		if !errors.Is(err, refused) {
			t.Fatalf("read error = %v, want %v", err, refused)
		}
		if calls := observed(); len(calls) != 2 {
			t.Fatalf("a failed read took the closing observation: %v", calls)
		}
	})
}

// TestProbeAroundPrefetchingOverlapsTheOpeningStatusRead pins proposed LCP-V0-033: the
// prefetching bracket hands the opening identity to prefetch while the opening status read is
// still running (the status reply below waits for it), exactly once, never for the closing
// observation and never after a failed identity read, and spawns the same four Git processes.
func TestProbeAroundPrefetchingOverlapsTheOpeningStatusRead(t *testing.T) {
	t.Run("identity-before-status", func(t *testing.T) {
		prefetched := make(chan [3]string, 2)
		statuses := 0
		run, observed := recordingRunner(func(command string) ([]byte, error) {
			switch command {
			case "rev-parse":
				return testIdentityOutput(), nil
			case "status":
				statuses++
				if statuses == 1 {
					select {
					case <-time.After(5 * time.Second):
						return nil, newError("test-command", "the opening status read finished before prefetch started")
					case identity := <-prefetched:
						prefetched <- identity
					}
				}
				return []byte{}, nil
			}
			return nil, newError("test-command", "unexpected Git command")
		})
		_, err := probeRepositoryAround(context.Background(), ".", run, func(objectFormat, commit, tree string) {
			prefetched <- [3]string{objectFormat, commit, tree}
		}, func(context.Context, Observation) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if calls := observed(); len(calls) != 4 {
			t.Fatalf("LCP-V0-033: Git processes = %v, want the bracket's four", calls)
		}
		if got := <-prefetched; got != [3]string{"sha1", testCommitRevision, testTreeRevision} {
			t.Fatalf("prefetch identity = %v", got)
		}
		if len(prefetched) != 0 {
			t.Fatal("prefetch ran for the closing observation too")
		}
	})
	t.Run("failed-identity-never-prefetches", func(t *testing.T) {
		run, _ := recordingRunner(func(command string) ([]byte, error) {
			if command == "rev-parse" {
				return []byte("sha1\nnot-an-object\n"), nil
			}
			return []byte{}, nil
		})
		calls := 0
		_, err := probeRepositoryAround(context.Background(), ".", run, func(string, string, string) { calls++ }, func(context.Context, Observation) error { return nil })
		if err == nil || calls != 0 {
			t.Fatalf("malformed identity: err = %v, prefetch calls = %d", err, calls)
		}
	})
}
