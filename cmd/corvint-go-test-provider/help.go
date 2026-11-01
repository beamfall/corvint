package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-970).
const usageText = `corvint-go-test-provider session --experimental --trusted-local --foreground --authority-bundle FILE [--interval DURATION] [--debounce DURATION] [--scope PATTERN]... [--watch FILE]... [--retain]
corvint-go-test-provider test-run [--dir DIR] [--timeout DURATION] [PACKAGE...]
`

var commandSummaries = map[string]string{
	"session":  "Run the experimental foreground live-test loop and print one JSON line per state transition",
	"test-run": "Run go test -json once and print the corvint-go-test-run/0 document with repository identity and test file blob IDs",
}

// companionHelp is corvint-go-test-provider's command tree and help (CAP-V0-971). The provider
// mode without a subcommand word is a root form, and the attachment argument the provider passes
// to itself is an internal entry point.
var companionHelp = sync.OnceValue(func() clitree.Help {
	internal := []clitree.Internal{{Word: attachmentArgument,
		Reason: "worker the provider re-executes itself with to attach the live parent verifier; not an operator entry point"}}
	return clitree.Help{
		Tree:      clitree.CompanionTree("corvint-go-test-provider", usageText, commandSummaries, nil, nil, internal),
		Title:     "the experimental Go live-test provider",
		RootForms: []string{"corvint-go-test-provider --experimental --trusted-local --authority-bundle FILE"},
		Detail: func(path []string, w io.Writer) {
			if len(path) == 1 && path[0] == "test-run" {
				io.WriteString(w, "\n"+testRunUsage)
			}
		},
	}
})

// serveHelp answers corvint-go-test-provider's help invocations before its own dispatch.
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	return companionHelp().Serve(args, stdout, stderr)
}
