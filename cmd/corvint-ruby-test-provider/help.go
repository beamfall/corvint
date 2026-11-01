package main

import (
	"io"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-970).
const usageText = `corvint-ruby-test-provider rspec --dir DIR [--exec-prefix ARG]... [--rspec ARG]... [--spec PATH]... [--formatter stock|corvint-rspec-json/0] [--junit] [--retain] [--timeout DURATION]
`

var commandSummaries = map[string]string{
	"rspec": "Run RSpec once with the JSON formatter and print the corvint-ruby-test-provider/0 document",
}

// printFlags writes the rspec flag defaults.
func printFlags(w io.Writer) {
	flags, _ := newFlags(w)
	flags.PrintDefaults()
}

// helpSpellings are the binary's own help words, which the tree help answers first except -help.
var helpSpellings = []clitree.Internal{
	{Word: "help", Reason: "alternative spelling of --help that the binary already accepted; its tree help answers it"},
	{Word: "-help", Reason: "single-dash alternative spelling of --help that the binary's own dispatch still answers"},
}

// companionHelp is corvint-ruby-test-provider's command tree and help (CAP-V0-971): the root help
// keeps the authored usage and flag defaults, and rspec's help ends with its flag defaults.
var companionHelp = sync.OnceValue(func() clitree.Help {
	return clitree.Help{
		Tree: clitree.CompanionTree("corvint-ruby-test-provider", usageText, commandSummaries, nil, nil, helpSpellings),
		Intro: func(w io.Writer) {
			io.WriteString(w, usage)
			printFlags(w)
		},
		Detail: func(_ []string, w io.Writer) {
			io.WriteString(w, "\n")
			printFlags(w)
		},
	}
})
