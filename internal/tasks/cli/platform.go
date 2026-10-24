package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// platformRefusal refuses every command except help and version on a
// platform Tasks does not support, before any store or worktree access
// (CTS-V0-025).
func platformRefusal(args []string) *wire.Result {
	if args[0] == "version" || args[0] == "--version" || parserHelp(args) {
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

// parserHelp reports a help request that commandHelp leaves to the command's
// own parser, which answers it without reading or writing state: --help
// anywhere in a ticket mutation argv (CAL-V0-170) and `pool --help`. A
// --help that is a flag's value, or that the parser refuses, is no help
// request and stays refused.
func parserHelp(args []string) bool {
	if len(args) == 2 && args[0] == "pool" && args[1] == "--help" {
		return true
	}
	if len(args) < 2 || args[0] != "ticket" {
		return false
	}
	if _, ok := mutationVerbs[args[1]]; !ok {
		return false
	}
	flags, res := parseMutateFlags(args[:2], args[2:])
	return res == nil && !flags.batch && flags.help
}
