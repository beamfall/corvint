package main

import (
	"io"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

var commandSummaries = map[string]string{
	"check":    "Report whether a newer official release of the component exists; no request leaves the host without --allow-network",
	"apply":    "Install the newer official release with an atomic switch and keep the previous binaries for rollback",
	"rollback": "Restore the previous binaries the last apply kept",
}

var commandExamples = map[string]string{
	"check":    "corvint-update check --component core --allow-network",
	"apply":    "corvint-update apply --component tasks --allow-network",
	"rollback": "corvint-update rollback --component core",
}

// versionFlags are answered by the companion contract after the tree (CAP-V0-700).
var versionFlags = []string{"--version"}

// companionHelp is corvint-update's command tree and help (CAP-V0-726). Its subcommand words are
// matched by comparison, not a switch, so the dispatch scan found none and it stayed on the
// ratchet; the root help keeps the authored usage the companion contract printed.
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-update", usage, commandSummaries, commandExamples, versionFlags, nil),
		Intro: func(w io.Writer) { io.WriteString(w, strings.TrimRight(usage, "\n")+"\n") }}
})

// serveHelp answers corvint-update's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
