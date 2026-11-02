package cli_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTicketShowOn360TicketJournaledFixture pins V1-3903: a journaled fixture
// store of 360 tickets is built by fixture.CommitPosts, which keeps each
// receipt within the reader's inline post cap, so ticket show reads it OK.
func TestTicketShowOn360TicketJournaledFixture(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	var recs []*ticket.Record
	posts := map[string][]byte{}
	for i := 0; i < 360; i++ {
		rec := fixture.Ticket(fmt.Sprintf("S%03d", i))
		recs = append(recs, rec)
		posts["intent/tickets/"+rec.TicketID.Local+".json"] = rec.Encode()
	}
	fixture.WriteIntent(t, r, recs...)
	fixture.CommitPosts(t, r, "MUTATION", "", posts)
	names, err := os.ReadDir(filepath.Join(r.StateDir, "receipts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", n.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := snapshot.DecodeReceipt(raw); err != nil {
			t.Fatalf("fixture receipt %s does not decode: %v", n.Name(), err)
		}
	}
	for _, local := range []string{"S000", "S359"} {
		var out bytes.Buffer
		cli.Run(cli.Env{Cwd: r.Root, Args: []string{"ticket", "show", local}, Stdout: &out, Stderr: io.Discard})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil || res.Outcome != wire.OutcomeOK || len(res.Codes) != 0 {
			t.Fatalf("ticket show %s on a 360-ticket store: %s", local, out.Bytes())
		}
	}
}
