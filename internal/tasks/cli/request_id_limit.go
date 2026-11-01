package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// requestIDHelp is the requestId member every command with --request-id
// carries in its verbose help (TCT-V0-991).
var requestIDHelp = fmt.Sprintf("--request-id is the idempotency key: at most %d bytes; a longer one is refused LIMIT_EXCEEDED naming the flag before any store access, including the base of multi-target and --batch forms (TCT-V0-991)", wire.MaxRequestIDBytes)

// requestIDLimitRefusal refuses a --request-id over wire.MaxRequestIDBytes
// before any command parser reads the store, so every command reports the
// same diagnostic (TCT-V0-991).
func requestIDLimitRefusal(args []string) *wire.Result {
	for i, a := range args {
		var v string
		switch {
		case a == "--request-id" && i+1 < len(args):
			v = args[i+1]
		case strings.HasPrefix(a, "--request-id="):
			v = strings.TrimPrefix(a, "--request-id=")
		default:
			continue
		}
		if n := len(v); n > wire.MaxRequestIDBytes {
			return errorResult(strings.Fields(verbPath(args)), wire.Errorf(wire.CodeLimitExceeded, "--request-id",
				"--request-id is %d bytes; it may be at most %d bytes (TCT-V0-991)", n, wire.MaxRequestIDBytes))
		}
	}
	return nil
}

// hasRequestIDFlag reports whether a command-tree node takes --request-id.
func hasRequestIDFlag(flags []string) bool { return slices.Contains(flags, "--request-id") }
