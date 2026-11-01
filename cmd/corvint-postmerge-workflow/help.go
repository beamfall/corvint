package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-970).
const usageText = `corvint-postmerge-workflow replay --change ID --dry-run --fixture FILE --policy FILE [--root REPO]
`

var commandSummaries = map[string]string{
	"replay": "Replay one historical change through recording connectors and print the replay report",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-postmerge-workflow's command tree and help (CAP-V0-971).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-postmerge-workflow", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental local post-merge replay companion, not a live writer"}
})

// serveHelp answers corvint-postmerge-workflow's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
