package gokernel

import (
	"context"
	"slices"
	"testing"
)

// TestSharedBracketSpawnsFourGitProcessesUnlessTheProfileIsEmitted pins the
// proposed GPK-V0-058 spawn count: the shared bracket issues exactly one
// complete observation, the read, and one complete observation -- four Git
// processes -- and adds the `ls-tree` read between the two observations only
// for the caller that emits the profile.
func TestSharedBracketSpawnsFourGitProcessesUnlessTheProfileIsEmitted(t *testing.T) {
	reply := func(command string) ([]byte, error) {
		switch command {
		case "rev-parse":
			return testIdentityOutput(), nil
		case "status", "ls-tree":
			return []byte{}, nil
		}
		return nil, newError("test-command", "unexpected Git command")
	}
	cases := map[string]struct {
		wantProfile bool
		want        int
		middle      []string
	}{
		"profile-not-emitted": {false, 4, nil},
		"profile-emitted":     {true, 5, []string{"ls-tree"}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			run, observed := recordingRunner(reply)
			var reads, finishes int
			read := func(_ context.Context, observation Observation) (func() error, error) {
				reads++
				if observation.TreeRevision != testTreeRevision || len(observation.DirtyPaths) != 0 {
					t.Fatalf("read observation = %+v", observation)
				}
				if len(observed()) != 2 {
					t.Fatalf("read issued after %v, want one complete observation", observed())
				}
				return func() error { finishes++; return nil }, nil
			}
			repository, err := probeRepositorySharing(context.Background(), ".", run, read, test.wantProfile)
			if err != nil {
				t.Fatal(err)
			}
			calls := observed()
			if len(calls) != test.want || reads != 1 || finishes != 1 {
				t.Fatalf("GPK-V0-058: %d Git processes, %d reads, %d finishes: %v", len(calls), reads, finishes, calls)
			}
			want := []string{"rev-parse", "status"}
			if !slices.Equal(sortedSide(calls, 0, 2), want) || !slices.Equal(sortedSide(calls, len(calls)-2, len(calls)), want) {
				t.Fatalf("bracket sides = %v", calls)
			}
			if !slices.Equal(calls[2:len(calls)-2], test.middle) {
				t.Fatalf("read stage = %v, want %v", calls[2:len(calls)-2], test.middle)
			}
			if (repository.ProfileID != "") != test.wantProfile || repository.TreeRevision != testTreeRevision {
				t.Fatalf("repository = %+v", repository)
			}
		})
	}
}

// TestSharedBracketRerunsTheReadWhenTheObservationMoves pins the retry: an
// identity that changes between the two observations re-runs the read against
// the new opening observation, exactly as the concurrent path re-runs its probe.
func TestSharedBracketRerunsTheReadWhenTheObservationMoves(t *testing.T) {
	identities := 0
	run, _ := recordingRunner(func(command string) ([]byte, error) {
		switch command {
		case "rev-parse":
			identities++
			if identities == 2 {
				return []byte("sha1\n" + testTreeRevision + "\n" + testCommitRevision + "\n"), nil
			}
			return testIdentityOutput(), nil
		case "status":
			return []byte{}, nil
		}
		return nil, newError("test-command", "unexpected Git command")
	})
	reads, finishes := 0, 0
	read := func(context.Context, Observation) (func() error, error) {
		reads++
		return func() error { finishes++; return nil }, nil
	}
	if _, err := probeRepositorySharing(context.Background(), ".", run, read, false); err != nil {
		t.Fatal(err)
	}
	if reads != 2 || finishes != 2 {
		t.Fatalf("GPK-V0-058: read ran %d times and finished %d times across one retry, want 2 and 2", reads, finishes)
	}
}
