package cli_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-025, CAL-V0-170: a help request only a command's own parser
// recognizes still answers help on every platform, with no store.
func TestCTSV0025_ParserHelpAnswersOnEveryPlatform(t *testing.T) {
	for _, args := range [][]string{
		{"ticket", "create", "--payload-stdin", "--help"},
		{"ticket", "create", "--help", "--payload-stdin"},
		{"ticket", "create", "--payload-stdin", "--help", "--verbose"},
		{"pool", "--help"},
	} {
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: t.TempDir(), Args: args, Stdin: unreadHelpInput{}, Stdout: &out})
		r, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out.Bytes())
		}
		for _, c := range r.Codes {
			if c == wire.CodeUnsupported {
				t.Fatalf("%v refused as unsupported: %s", args, out.Bytes())
			}
		}
		if args[0] == "ticket" && (code != 0 || field(r.Items[0], "operation").Str != "CREATE") {
			t.Fatalf("%v: code %d %s", args, code, out.Bytes())
		}
	}
}
