package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-postmerge-workflow replay --change ID --dry-run --fixture FILE --policy FILE [--root REPO]
corvint-postmerge-workflow encode-id VALUE
`

var commandSummaries = map[string]string{
	"replay":    "Replay one historical change through recording connectors and print the replay report",
	"encode-id": "Print VALUE, such as a branch name, encoded as a workflow identifier (PMR-V0-960)",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-postmerge-workflow's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-postmerge-workflow", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental local post-merge replay companion, not a live writer"}
})

// serveHelp answers corvint-postmerge-workflow's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
