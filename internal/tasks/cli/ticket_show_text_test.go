package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0741_ShowFormatTextRendersBodyAndAcceptance pins V1-3353 and
// V1-3817: `ticket show --format text` prints the title, fields, body and
// numbered acceptance criteria as plain text with nothing JSON-escaped, the
// default stays the JSON envelope carrying the same text under record., and
// a refusal or a compact flag keeps the JSON form (TCL-V0-741).
func TestTCLV0741_ShowFormatTextRendersBodyAndAcceptance(t *testing.T) {
	r, _ := similarRepo(t)
	p := similarPayload("Packet ticket", `First line of the body.\nSecond line says \"quoted\".`)
	p = bytes.Replace(p, []byte(`"acceptanceCriteria":["it exists"]`), []byte(`"acceptanceCriteria":["The packet is \"readable\".","A test pins it."]`), 1)
	p = bytes.Replace(p, []byte(`"touchPaths":[]`), []byte(`"touchPaths":["internal/tasks/cli/"]`), 1)
	created := atm(t, r.Root, p, "ticket", "create", "--request-id", "packet-1", "--issued-at", "2026-11-01T12:00:00Z", "--payload-stdin")
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", created.stdout)
	}
	id := field(created.res.Items[0], "ticketId").Str
	local := id[strings.LastIndex(id, ":")+1:]

	text := func(args ...string) (int, string) {
		t.Helper()
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: r.Root, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: io.Discard})
		return code, out.String()
	}
	for _, form := range [][]string{{"--format", "text"}, {"--format=text"}} {
		code, got := text(append([]string{"ticket", "show", local}, form...)...)
		want := "# corvint-tasks ticket text (untrusted ticket content)\n" +
			id + "  Packet ticket\n" +
			"status: OPEN  priority: P2  kind: FEATURE  revision: 1\n" +
			"labels: (none)\n" +
			"touchPaths: internal/tasks/cli/\n" +
			"coverage: QUALIFIED\n" +
			"\nBody:\nFirst line of the body.\nSecond line says \"quoted\".\n" +
			"\nAcceptance criteria:\n1. The packet is \"readable\".\n2. A test pins it.\n"
		if code != 0 || got != want {
			t.Fatalf("%v: exit %d\n%s\nwant\n%s", form, code, got, want)
		}
	}
	// The default and --format json are the unchanged envelope.
	for _, args := range [][]string{{"ticket", "show", local}, {"ticket", "show", local, "--format", "json"}} {
		x := atm(t, r.Root, nil, args...)
		acs := field(field(x.res.Items[0], "record"), "acceptanceCriteria")
		if x.res.Outcome != wire.OutcomeOK || len(acs.Arr) != 2 || acs.Arr[0].Str != `The packet is "readable".` {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	// Refusals stay JSON: an unknown ticket, a bad format, and a compact flag with text.
	for _, args := range [][]string{
		{"ticket", "show", "NOPE-9999", "--format", "text"},
		{"ticket", "show", local, "--format", "yaml"},
		{"ticket", "show", local, "--format"},
		{"ticket", "show", local, "--format", "text", "--summary"},
		{"ticket", "show", local, "--fields", "record.body", "--format", "text"},
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v was accepted: %s", args, x.stdout)
		}
	}
}
