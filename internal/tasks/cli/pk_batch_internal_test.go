// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// TCL-V0-7404: the precheck uses the same admission as a retry claim.
func TestTCLV07404RestoredScopeAndRequestBinding(t *testing.T) {
	id, err := wire.ParseTicketID("test", "ticket:acme:main:AT-0001")
	if err != nil {
		t.Fatal(err)
	}
	rec := &ticket.Record{TicketID: id, AcceptanceRevision: "1"}
	prior := &snapshot.Attempt{TicketID: id, TicketRevision: "1", Generation: "1", Phase: "RELEASED", Scope: &snapshot.Scope{Source: "DECLARED", Resources: []ticket.Resource{{Class: "PATH", Key: "src/"}}}}
	bad, restored := transaction.RestoreScope(rec, map[string]*snapshot.Attempt{"a": prior}, []string{"src/a.go", "other/b.go"})
	if restored == nil || strings.Join(restored.Paths, " ") != "src/a.go" || strings.Join(bad, " ") != "other/b.go" {
		t.Fatalf("bad=%v restored=%+v", bad, restored)
	}
	row := &closeRow{local: "AT-0001", hit: batchHit{ticketID: id.Raw}, g: &gatedRow{candidate: "aaa", base: "bbb", scope: &claimScope{source: scopeDeclared, keys: []string{"src/"}}}}
	first := row.gatedStepID(50, "claim")
	if first != row.gatedStepID(50, "claim") {
		t.Fatal("unstable")
	}
	row.g.candidate = "ccc"
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("candidate omitted")
	}
	row.g.candidate = "aaa"
	row.g.scope.source = scopeDiff
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("scope source omitted")
	}
	row.g.scope.source = scopeDeclared
	row.g.scope.keys = []string{"docs/"}
	if first == row.gatedStepID(50, "claim") {
		t.Fatal("scope keys omitted")
	}
	prior.TicketRevision = "2"
	if _, restored := transaction.RestoreScope(rec, map[string]*snapshot.Attempt{"a": prior}, []string{"src/a.go"}); restored != nil {
		t.Fatal("stale acceptance restored")
	}
}
