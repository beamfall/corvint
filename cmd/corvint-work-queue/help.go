package main

import (
	"io"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis text the command tree derives from (CAP-V0-726).
const usageText = `usage: corvint-work-queue snapshot|details|verify
`

var commandSummaries = map[string]string{
	"snapshot": "Print the canonical work-queue snapshot of the repository containing the working directory",
	"details":  "Print the canonical details document for the same snapshot",
	"verify":   "Print the canonical checkpoint document that verifies the same snapshot",
}

var commandExamples = map[string]string{
	"snapshot": "corvint-work-queue snapshot",
	"details":  "corvint-work-queue details",
	"verify":   "corvint-work-queue verify",
}

// companionHelp is corvint-work-queue's command tree and help (CAP-V0-726). Its operations are
// dispatched by internal/worklistadapter, so that package is a dispatch source beside cmd/.
// Help answers before runProducer and acquires nothing, so the closed operation set of
// WQO-V0-007 is unchanged.
var companionHelp = sync.OnceValue(func() clitree.Help {
	const binary = "corvint-work-queue"
	builder := clitree.NewBuilder(binary)
	builder.AddText(usageText, "")
	for path, summary := range commandSummaries {
		builder.Declare(strings.Fields(path), summary)
	}
	for path, example := range commandExamples {
		builder.Example(strings.Fields(path), example)
	}
	tree := builder.Tree(binary+" help --json", "cmd/"+binary+" usageText",
		[]string{"cmd/" + binary, "internal/worklistadapter"}, nil, nil)
	return clitree.Help{Tree: tree, Title: "the read-only work-queue observation producer"}
})

// serveHelp answers corvint-work-queue's help invocations before its own dispatch. An unknown
// verb is left to that dispatch, whose `unsupported operation` refusal WQO-V0-007 and WQO-V0-030
// pin, so the shared unknown-verb envelope does not apply here (ACX-V0-904).
func serveHelp(args []string, stdout, stderr io.Writer) (int, bool) {
	help := companionHelp()
	if _, unknown := clitree.UnknownVerb(help.Tree, args); unknown {
		if handled, _ := clitree.Answer(help.Tree, args, help.Root(), io.Discard); !handled {
			return 0, false
		}
	}
	return help.Serve(args, stdout, stderr)
}
