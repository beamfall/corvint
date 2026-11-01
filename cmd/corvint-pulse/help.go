package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-pulse [--root ROOT] serve --stdio
`

var commandSummaries = map[string]string{
	"serve": "Serve the Pulse protocol over stdin and stdout for the repository at --root, which is required and comes first",
}

var commandExamples = map[string]string{"serve": "corvint-pulse --root . serve --stdio"}

// companionHelp is corvint-pulse's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-pulse", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental Pulse stdio server"}
})

// serveHelp answers corvint-pulse's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
