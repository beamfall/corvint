package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-dashboard-snapshot snapshot --root ROOT [--source ADAPTER=PATH]... [--conformance --generated-at TIME]
corvint-dashboard-snapshot roadmap --root ROOT --tasks BINARY --store DIR --generated-at TIME [--receipts-dir DIR] [--docs-state FILE] [--requirements FILE] [--atm BINARY]
corvint-dashboard-snapshot cem-stable --root ROOT --cem PATH --ocm PATH --expected-base OID --target OID --artifact-root DIR
`

var commandSummaries = map[string]string{
	"snapshot":   "Compile the dashboard snapshot of a repository from its source adapters; --conformance pins --generated-at",
	"roadmap":    "Compile the roadmap snapshot from a Tasks store, receipts, docs state and requirements; --atm is the legacy spelling of --tasks",
	"cem-stable": "Compile the dashboard view of a cem/1.0 stable map and its OCM against an expected base and target",
}

// versionFlags are answered by the companion contract before the tree (CAP-V0-700).
var versionFlags = []string{"--version"}

// companionHelp is corvint-dashboard-snapshot's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-dashboard-snapshot", usageText, commandSummaries, nil, versionFlags, nil),
		Title: "the local dashboard snapshot compiler"}
})

// serveHelp answers corvint-dashboard-snapshot's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
