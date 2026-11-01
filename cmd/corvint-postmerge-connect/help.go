package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-970).
const usageText = `corvint-postmerge-connect plan --experimental --root REPO --fixture FILE --policy FILE --input FILE [--mode dry-run]
corvint-postmerge-connect read-intake --experimental --root REPO --fixture FILE --policy FILE --input FILE --output FILE --author-root DIR [--mode dry-run]
corvint-postmerge-connect write --experimental --root REPO --fixture FILE --policy FILE --input FILE [--mode dry-run|recording|live] [--output FILE] [--author-root DIR]
`

var commandSummaries = map[string]string{
	"plan":        "Print the typed write plan for one input; writes no file",
	"read-intake": "Read the source item through the fixture and save the intake file outside the authoring worktree",
	"write":       "Apply the plan in dry-run, recording or local-fixture live mode",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-postmerge-connect's command tree and help (CAP-V0-971).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-postmerge-connect", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental post-merge tracker and forge connector (local fixtures only)"}
})

// serveHelp answers corvint-postmerge-connect's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
