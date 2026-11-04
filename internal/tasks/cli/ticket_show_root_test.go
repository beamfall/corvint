// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCL-V0-926: show-local --root resolves before any read or output projection.
func TestTCLV0926TicketShowRootAfterVerb(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id, local := tcdTicket(t, root, "root-selected-ticket", "")
	parent := filepath.Dir(root)
	for _, args := range [][]string{
		{"ticket", "show", local, "--root", root},
		{"ticket", "show", "--root", root, id},
		{"ticket", "show", local, "--root=" + root, "--json"},
		{"ticket", "show", local, "--root", filepath.Base(root), "--fields", "record.ticketId"},
		{"ticket", "show", local, "--summary", "--root", root},
	} {
		x := atm(t, parent, nil, args...)
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !bytes.Contains(x.stdout, []byte(id)) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	// The test runner's envelope helper cannot decode a successful text response.
	var output bytes.Buffer
	code := cli.Run(cli.Env{Cwd: parent, Args: []string{"ticket", "show", local, "--root", root, "--format", "text"}, Stdout: &output})
	stdout := output.Bytes()
	if code != 0 || !bytes.Contains(stdout, []byte("root-selected-ticket")) {
		t.Fatalf("text --root: exit %d: %s", code, stdout)
	}
	for _, tail := range [][]string{{"--root"}, {"--root="}, {"--root", "--json"}} {
		x := atm(t, parent, nil, append([]string{"ticket", "show", local}, tail...)...)
		if x.code != 1 || x.res.Outcome != wire.OutcomeError || x.res.Snapshot != nil ||
			len(x.res.Codes) != 1 || x.res.Codes[0] != wire.CodeMalformed {
			t.Fatalf("%v: %s", tail, x.stdout)
		}
	}
}
