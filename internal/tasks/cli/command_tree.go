package cli

import (
	"regexp"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// commandTreeInternal are the dispatched words and options of corvint-tasks
// that no command synopsis names (CAP-V0-043).
var commandTreeInternal = []clitree.Internal{
	{Word: "lane-leader", Reason: "lane leader process the dispatcher starts for itself (cmd/corvint-tasks)"},
	{Word: "--directory", Reason: "lane-leader option the dispatcher passes"},
	{Word: "--capsule", Reason: "lane-leader option the dispatcher passes"},
	{Word: "--supervise", Reason: "run option the detached launcher passes to its own supervisor process"},
}

// CommandTree is the corvint-tasks command tree (corvint-command-tree/0),
// derived from the command help each verb prints, so no node can exist
// without help (CAP-V0-040, CAP-V0-046). `corvint-tasks help --json` prints it.
func CommandTree() clitree.Tree {
	commandTreeOnce.Do(func() { commandTreeValue = buildCommandTree() })
	return commandTreeValue
}

var (
	commandTreeOnce  sync.Once
	commandTreeValue clitree.Tree
)

// usageRemark is the parenthesized prose a usage line carries after three or
// more spaces, such as "   (stdin when absent)".
var usageRemark = regexp.MustCompile(`\s{3,}\(.*$`)

func buildCommandTree() clitree.Tree {
	builder := clitree.NewBuilder("corvint-tasks")
	for _, verb := range append(append([]string{}, ReadVerbs...), OmittedVerbs...) {
		path := strings.Fields(verb)
		builder.Declare(path, "")
		result := fullCommandHelp(path)
		if result == nil {
			if verb == "help" {
				builder.AddForm("corvint-tasks help [--json]", "")
			}
			continue
		}
		usage, _ := result.Items[0].Obj.Get("usage")
		for _, form := range strings.Split(usage.Str, "; ") {
			builder.AddForm(usageRemark.ReplaceAllString(strings.TrimSpace(form), ""), "")
		}
	}
	return builder.Tree("corvint-tasks help --json", "corvint-tasks command help (internal/tasks/cli)",
		[]string{"cmd/corvint-tasks", "internal/tasks/cli"}, []string{"--version", "--verbose"}, commandTreeInternal)
}

// CommandTreeJSON is the canonical tree document `corvint-tasks help --json`
// prints.
func CommandTreeJSON() []byte {
	encoded, err := clitree.Encode(CommandTree())
	if err != nil {
		panic("command tree encoding: " + err.Error())
	}
	return encoded
}

// treeHelp adds the node's required inputs and one example to a command's
// help object (CAP-V0-044).
func treeHelp(cmd []string, o *wire.Object) {
	node, ok := CommandTree().Find(cmd)
	if !ok || len(node.Path) != len(cmd) {
		return
	}
	required := node.Required
	if required == nil {
		required = []string{}
	}
	o.Set("required", wire.Strings(required))
	o.Set("example", wire.String(node.Example))
	if hasRequestIDFlag(node.Flags) {
		o.Set("requestId", wire.String(requestIDHelp))
	}
}
