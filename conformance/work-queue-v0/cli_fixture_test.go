//go:build unix

package workqueuev0_test

import (
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// Independent Python standard-library fixture bytes are frozen before evaluating
// the consumer. This proves populated input validity, not production eligibility.
func TestCLIIndependentPopulatedInputs(t *testing.T) {
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile("testdata/cli_" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	policy, err := workqueue.ParsePolicy(read("policy"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workqueue.ParseSnapshot(read("snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	details, err := workqueue.ParseDetails(read("details"))
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := workqueue.ParseCheckpoint(read("verify"))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := workqueue.ParseEnvelope(read("envelope"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("WQO-V0-008 populated snapshot facts", func(t *testing.T) {
		if got := workqueue.ValidateSnapshot(snapshot); got.State != workqueue.StateValidated || len(got.Unknowns) != 0 {
			t.Fatalf("independent populated snapshot invalid: %+v", got)
		}
		if len(snapshot.Tickets) != 4 || snapshot.Scope.TicketCount != 4 || len(snapshot.Leases) != 1 {
			t.Fatal("fixture lost its ticket/active lease coverage")
		}
		if len(snapshot.CapacityClasses) != 2 || len(envelope.Available) != 2 || len(envelope.Capabilities) != 2 {
			t.Fatal("fixture lost its repository/caller capacity or route coverage")
		}
		if len(snapshot.Tickets[0].RouteAlternatives) != 2 || len(snapshot.Tickets[0].CapacityUses) != 2 {
			t.Fatal("fixture lost its multi-route/multi-resource witness")
		}
		if snapshot.Tickets[0].TouchPaths[0] != snapshot.Tickets[1].TouchPaths[0] || snapshot.Tickets[0].CollisionGroupIDs[0] != snapshot.Tickets[2].CollisionGroupIDs[0] {
			t.Fatal("fixture lost its derived-path and adapter collision bases")
		}
	})
	t.Run("WQO-V0-012 detail coverage checkpoint binding", func(t *testing.T) {
		limit, err := strconv.Atoi(policy.DetailLimit)
		if err != nil {
			t.Fatal(err)
		}
		var expected []string
		for _, ticket := range snapshot.Tickets {
			if ticket.Lifecycle == "READY" && len(expected) < limit {
				expected = append(expected, ticket.TicketVersionID)
			}
		}
		sort.Strings(expected)
		var actual []string
		for _, detail := range details.Details {
			actual = append(actual, detail.TicketVersionID)
		}
		sort.Strings(actual)
		if len(expected) != 3 || !reflect.DeepEqual(expected, snapshot.DetailRequestTicketVersionIDs) || !reflect.DeepEqual(expected, actual) {
			t.Fatalf("detail requests/documents must exactly match the READY prefix: expected=%v requested=%v actual=%v", expected, snapshot.DetailRequestTicketVersionIDs, actual)
		}
		if got := workqueue.ValidateDetailCoverage(snapshot, details); got.State != workqueue.StateValidated || len(got.Unknowns) != 0 {
			t.Fatalf("independent digest-bound details invalid: %+v", got)
		}
		if len(details.Details) != 3 {
			t.Fatal("fixture lost populated hostile-prose details")
		}
		if err := workqueue.ValidateCheckpoint(checkpoint, policy, snapshot, snapshot.RepositorySource); err != nil {
			t.Fatal(err)
		}
	})
}
