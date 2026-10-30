//go:build unix

package workqueuev0_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// These witnesses exercise specified wire bindings, not installed-binary
// authentication, source acquisition, or a general-purpose observation API.
func TestIndependentWireTupleInvalidation(t *testing.T) {
	t.Run("WQO-V0-009 queue source binding", func(t *testing.T) {
		t.Run("WQO-V0-040 immutable tuple invalidation", func(t *testing.T) {
			goldens := identityGoldens(t)
			for _, dimension := range []string{"mapping", "fixed-operation", "adapter-path", "source-commit", "source-tree", "source-materialization", "checkpoint", "lifecycle", "route", "capacity", "collision", "touch-path"} {
				t.Run(dimension, func(t *testing.T) {
					policy, err := workqueue.ParsePolicy([]byte(goldens["policy"].Wire))
					if err != nil {
						t.Fatal(err)
					}
					snapshot, err := workqueue.ParseSnapshot([]byte(goldens["snapshot"].Wire))
					if err != nil {
						t.Fatal(err)
					}
					checkpoint, err := workqueue.ParseCheckpoint([]byte(goldens["checkpoint"].Wire))
					if err != nil {
						t.Fatal(err)
					}
					if err := workqueue.ValidateCheckpoint(checkpoint, policy, snapshot, snapshot.RepositorySource); err != nil {
						t.Fatalf("original independent tuple rejected: %v", err)
					}
					before := checkpoint.Canonical()
					oldSnapshot, oldQueueSource := snapshot.ID, workqueue.QueueSourceIdentity(policy, snapshot)
					switch dimension {
					case "mapping":
						policy.MappingVersion = "v2"
					case "fixed-operation":
						policy.Operations.Snapshot = []string{"snapshot", "fixed-v2"}
					case "adapter-path":
						policy.AdapterPath = "script/adapter-v2"
					case "source-commit":
						snapshot.RepositorySource.Commit = strings.Repeat("a", 40)
					case "source-tree":
						snapshot.RepositorySource.Tree = strings.Repeat("b", 40)
					case "source-materialization":
						snapshot.RepositorySource.MaterializationSHA256 = strings.Repeat("c", 64)
					case "checkpoint":
						snapshot.Checkpoint.Version = "v2"
					case "lifecycle":
						snapshot.Tickets[0].Lifecycle = "HELD"
						snapshot.DetailRequestTicketVersionIDs = []string{}
					case "route":
						snapshot.Tickets[0].RouteAlternatives[0].Requires = []string{"capability:corvint:worklist:new"}
					case "capacity":
						snapshot.CapacityClasses[0].AvailableUnits = 1
					case "collision":
						snapshot.Tickets[0].CollisionGroupIDs = []string{"collision:corvint:worklist:new"}
					case "touch-path":
						snapshot.Tickets[0].TouchPaths = []string{"src/new.go"}
					}
					policy.RefreshIdentity()
					if _, err := workqueue.ParsePolicy(policy.Canonical()); err != nil {
						t.Fatalf("changed wire policy is not valid: %v", err)
					}
					snapshot.PolicyID = policy.ID
					workqueue.RefreshRepositorySource(&snapshot.RepositorySource)
					workqueue.RefreshSnapshot(snapshot)
					if got := workqueue.ValidateSnapshot(snapshot); got.State != workqueue.StateValidated {
						t.Fatalf("changed wire snapshot is not valid: %+v", got)
					}
					if snapshot.ID == oldSnapshot || workqueue.QueueSourceIdentity(policy, snapshot) == oldQueueSource {
						t.Fatal("changed bound dimension reused an old snapshot or queue-source ID")
					}
					if err := workqueue.ValidateCheckpoint(checkpoint, policy, snapshot, snapshot.RepositorySource); err == nil {
						t.Fatal("old checkpoint accepted for a changed tuple")
					}
					if !bytes.Equal(before, checkpoint.Canonical()) {
						t.Fatal("historical checkpoint changed")
					}
				})
			}
		})
	})
}

func TestIndependentFutureVocabularyRefusal(t *testing.T) {
	t.Run("WQO-V0-040", func(t *testing.T) {
		goldens := identityGoldens(t)
		policy, err := workqueue.ParsePolicy([]byte(goldens["policy"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		policy.AdapterProfile = "repository-work-queue-adapter/1"
		policy.RefreshIdentity()
		if _, err := workqueue.ParsePolicy(policy.Canonical()); err == nil {
			t.Fatal("unsupported future adapter profile accepted after rehashing")
		}
		snapshot, err := workqueue.ParseSnapshot([]byte(goldens["snapshot"].Wire))
		if err != nil {
			t.Fatal(err)
		}
		snapshot.Tickets[0].Lifecycle = "FUTURE_READY"
		workqueue.RefreshSnapshot(snapshot)
		if _, err := workqueue.ParseSnapshot(snapshot.Canonical()); err == nil {
			t.Fatal("unsupported future lifecycle accepted after rehashing")
		}
	})
}
