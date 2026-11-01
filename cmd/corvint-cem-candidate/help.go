package main

import (
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// usageText is the synopsis the command tree derives from (CAP-V0-930).
const usageText = `corvint-cem-candidate verify --experimental --repository REPO --map FILE --expected-base OID --target OID --artifacts DIR
corvint-cem-candidate assemble --experimental --request FILE --out-dir DIR
corvint-cem-candidate assemble-stable --experimental --request FILE --out-dir DIR
`

var commandSummaries = map[string]string{
	"verify":          "Verify a cem/1.0-experimental.1 candidate map against independently supplied revisions",
	"assemble":        "Check a closed candidate assembly request; retired and cem/1.0 sources refuse without writing",
	"assemble-stable": "Assemble a cem/1.0 stable bundle from a plain or Tasks-bound request and print its re-verify argv",
}

var (
	commandTreeOnce  sync.Once
	commandTreeValue clitree.Tree
)

// commandTree is corvint-cem-candidate's command tree (corvint-command-tree/0).
func commandTree() clitree.Tree {
	commandTreeOnce.Do(func() {
		builder := clitree.NewBuilder("corvint-cem-candidate")
		builder.AddText(usageText, "")
		for path, summary := range commandSummaries {
			builder.Declare(strings.Fields(path), summary)
		}
		commandTreeValue = builder.Tree("corvint-cem-candidate help --json", "cmd/corvint-cem-candidate usageText",
			[]string{"cmd/corvint-cem-candidate"}, nil, nil)
	})
	return commandTreeValue
}

func rootHelp(tree clitree.Tree) string {
	var text strings.Builder
	text.WriteString("corvint-cem-candidate: the experimental CEM candidate companion (CEM-V1)\n\nUsage:\n")
	for _, node := range tree.Nodes {
		text.WriteString("  " + strings.Join(node.Usage, "\n  ") + "\n")
	}
	text.WriteString("\nCommands:\n")
	for _, node := range tree.Nodes {
		text.WriteString("  " + strings.Join(node.Path, " ") + ": " + node.Summary + "\n")
	}
	text.WriteString("\nRun `corvint-cem-candidate COMMAND --help` for one command's usage, flags, required inputs and example.\n")
	text.WriteString("Every command of corvint-cem-candidate: " + tree.HelpJSON + "\n")
	return text.String()
}
