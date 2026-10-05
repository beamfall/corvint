package importer_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/importer"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func prereqExport(block, local string) []byte {
	return issue502Export(block, func(o *wire.Object) {
		o.Set("executionPrerequisites", wire.Array(obj("gateId", wire.Null(), "obligation", str("COMPLETED"), "stages", wire.Strings([]string{"integrate"}), "ticketId", str(fixture.TicketID(local)))))
	})
}

// TestCALV0099_ImportAcceptsAndPreservesPrerequisites: an export item may
// carry executionPrerequisites (validated against the store like
// dependencies); a re-import whose export omits the key keeps the record's
// existing set; a record without the key stays byte-free of it.
func TestCALV0099_ImportAcceptsAndPreservesPrerequisites(t *testing.T) {
	p := fixture.Ticket("AT-02")
	first := issue502Plan(t, prereqExport("one\n", "AT-02"), p)
	if len(first.ExecutionPrerequisites) != 1 || first.ExecutionPrerequisites[0].TicketID.Raw != p.TicketID.Raw {
		t.Fatalf("import dropped the key: %+v", first.ExecutionPrerequisites)
	}
	next := issue502Plan(t, issue502Export("one, changed\n", nil), first, p)
	if next.Revision != "2" || len(next.ExecutionPrerequisites) != 1 {
		t.Fatalf("re-import dropped the key: %+v", next)
	}
	if plain := issue502Plan(t, issue502Export("one\n", nil)); bytes.Contains(plain.Encode(), []byte("executionPrerequisites")) {
		t.Fatal("a legacy import gained the key")
	}

	exp, err := importer.Decode(prereqExport("one\n", "AT-99"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importer.Plan(exp, issue502Store(t, p), "operator", issue502Now); wire.CodeOf(err) != wire.CodeDependencyMissing {
		t.Fatalf("missing prerequisite imported: %v", err)
	}
	bad := issue502Export("one\n", func(o *wire.Object) { o.Set("executionPrerequisites", wire.Array()) })
	if exp, err := importer.Decode(bad); err == nil {
		_, err = importer.Plan(exp, issue502Store(t, p), "operator", issue502Now)
		if wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("empty prerequisites imported: %v", err)
		}
	} else if wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("empty prerequisites: %v", err)
	}
}
