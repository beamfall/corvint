package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-intake validate --root PATH --base FULL_OID --head FULL_OID < candidate.json
corvint-intake reader-check --root PATH --raw FILE --output FILE
`

var commandSummaries = map[string]string{
	"validate":     "Validate a reader candidate on stdin against trusted base and head pins and print the author input",
	"reader-check": "Preflight the reader boundary for a raw file and a fresh intake file",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-intake's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-intake", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the experimental local intake validator; output never grants authority; host confinement and reader extraction are NOT_OBSERVED"}
})

// serveHelp answers corvint-intake's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
