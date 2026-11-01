package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-970).
const usageText = `corvint-postmerge-metrics report --policy FILE --records FILE --from TIME --until TIME
`

var commandSummaries = map[string]string{
	"report": "Report post-merge metrics for the records between --from and --until under a host policy",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-postmerge-metrics's command tree and help (CAP-V0-971).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-postmerge-metrics", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental post-merge metrics report"}
})

// serveHelp answers corvint-postmerge-metrics's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
