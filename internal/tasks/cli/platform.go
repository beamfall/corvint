package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// platformRefusal refuses every command except help and version on a
// platform Tasks does not support, before any store or worktree access
// (CTS-V0-025).
func platformRefusal(args []string) *wire.Result {
	if args[0] == "version" || args[0] == "--version" {
		return nil
	}
	err := wire.PlatformRefusal()
	if err == nil {
		return nil
	}
	return &wire.Result{Command: strings.Fields(verbPath(args)), Outcome: wire.OutcomeError, Codes: []string{wire.CodeUnsupported}, NotRetryable: true, Warnings: []string{
		err.Error() + "; nothing was read or written. The release spec lists only darwin and linux (CTS-V0-025).",
	}}
}
