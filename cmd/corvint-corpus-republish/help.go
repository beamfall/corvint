package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-972).
const usageText = `corvint-corpus-republish build --trusted-local --root ROOT --request FILE --policy FILE --expected-policy-sha256 SHA --output-dir DIR [--documentation-root DIR] [--previous-result FILE] [--previous-artifact FILE] [--previous-sidecar FILE]
corvint-corpus-republish queue --trusted-local --root ROOT --request FILE --policy FILE --expected-policy-sha256 SHA --pending FILE [--documentation-root DIR] [--previous-result FILE] [--previous-artifact FILE] [--previous-sidecar FILE]
`

var commandSummaries = map[string]string{
	"build": "Build a republished corpus into a fresh private output directory",
	"queue": "Queue a republish request in the private single-writer pending state",
}

var commandExamples = map[string]string{}

// companionHelp is corvint-corpus-republish's command tree and help (CAP-V0-973).
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{Tree: clitree.CompanionTree("corvint-corpus-republish", usageText, commandSummaries, commandExamples, nil, nil),
		Title: "the trusted-local corpus republish companion"}
})

// serveHelp answers corvint-corpus-republish's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
