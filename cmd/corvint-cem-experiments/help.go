package main

import (
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis the command tree derives from (CAP-V0-930). Every verb parses the
// same flag set; each synopsis names the flags that verb reads.
const usageText = `corvint-cem-experiments plan --request FILE --out DIR --tasks-executable PATH --tasks-sha256 HEX [--repo DIR] [--stable-artifact-root DIR]
corvint-cem-experiments run --plan FILE --approve SHA256 --out DIR --experimental --trusted-local --tasks-executable PATH --tasks-sha256 HEX [--repo DIR] [--stable-artifact-root DIR]
corvint-cem-experiments verify --plan FILE --receipt FILE --tasks-executable PATH --tasks-sha256 HEX [--repo DIR] [--stable-artifact-root DIR]
corvint-cem-experiments gate --plan FILE --receipt FILE --tasks-executable PATH --tasks-sha256 HEX [--repo DIR] [--stable-artifact-root DIR]
`

var commandSummaries = map[string]string{
	"plan":   "Plan a CEM criterion experiment from a request JSON and print its plan digest",
	"run":    "Run the approved experiment plan as trusted local test execution and print its receipt digest",
	"verify": "Verify an experiment plan and receipt offline",
	"gate":   "Verify an experiment plan and receipt and apply the experiment gate",
}

var (
	commandTreeOnce  sync.Once
	commandTreeValue clitree.Tree
)

// commandTree is corvint-cem-experiments' command tree (corvint-command-tree/0).
func commandTree() clitree.Tree {
	commandTreeOnce.Do(func() {
		builder := clitree.NewBuilder("corvint-cem-experiments")
		builder.AddText(usageText, "")
		for path, summary := range commandSummaries {
			builder.Declare(strings.Fields(path), summary)
		}
		commandTreeValue = builder.Tree("corvint-cem-experiments help --json", "cmd/corvint-cem-experiments usageText",
			[]string{"cmd/corvint-cem-experiments"}, nil, nil)
	})
	return commandTreeValue
}

func rootHelp(tree clitree.Tree) string {
	var text strings.Builder
	text.WriteString("corvint-cem-experiments: the experimental CEM criterion experiment companion (CEX-V0)\n\nUsage:\n")
	for _, node := range tree.Nodes {
		text.WriteString("  " + strings.Join(node.Usage, "\n  ") + "\n")
	}
	text.WriteString("\nCommands:\n")
	for _, node := range tree.Nodes {
		text.WriteString("  " + strings.Join(node.Path, " ") + ": " + node.Summary + "\n")
	}
	text.WriteString("\nRun `corvint-cem-experiments COMMAND --help` for one command's usage, flags, required inputs and example.\n")
	text.WriteString("Every command of corvint-cem-experiments: " + tree.HelpJSON + "\n")
	return text.String()
}
