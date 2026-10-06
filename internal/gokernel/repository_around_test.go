package gokernel

import (
	"context"
	"errors"
	"slices"
	"testing"
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
		repository, err := probeRepositoryAround(context.Background(), ".", run, func(_ context.Context, observation Observation) error {
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
		_, err := probeRepositoryAround(context.Background(), ".", run, func(context.Context, Observation) error {
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
		_, err := probeRepositoryAround(context.Background(), ".", run, func(context.Context, Observation) error {
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
