package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
)

// commandHelpText is every corvint-test synopsis, one node per line. The command tree, every
// node's --help and `corvint-test help --json` derive from it, and the capabilities check fails
// when a dispatched subcommand or flag is missing from it (CAP-V0-040 to CAP-V0-047).
const commandHelpText = `corvint-test help [--json | --tree]
corvint-test e2e catalog --rev REV [--root DIR] [--manifest FILE] [--out FILE]
corvint-test e2e harvest --rev REV [--root DIR] [--manifest FILE] [--source [ALIAS:]DIR]... [--out FILE]
corvint-test e2e obligations --base REV --head REV [--root DIR] [--manifest FILE] [--coverage FILE] [--ledger FILE] [--module-graph] [--out FILE]
    [--endpoint-source [ALIAS:]DIR]... [--rails-routes [ALIAS:]FILE]... [--openapi [ALIAS:]FILE]... [--remedy-summary [--remedy-commits N]]
corvint-test e2e compile --base REV --head REV --out DIR [--root DIR] [--manifest FILE] [--coverage FILE] [--ledger FILE] [--module-graph]
    [--endpoint-source [ALIAS:]DIR]... [--rails-routes [ALIAS:]FILE]... [--openapi [ALIAS:]FILE]...
corvint-test e2e prove --base REV --head REV --out DIR [--root DIR] [--manifest FILE] [--coverage FILE] [--ledger FILE] [--module-graph]
    [--repeats N] [--node-modules DIR] [--node-bin FILE] [--package-json FILE] [--lockfile FILE]
    [--hosted-test FILE [--hosted-title TITLE]] [--record-coverage] [--oracle-acceptance FILE] [--endpoint-source [ALIAS:]DIR]... [--rails-routes [ALIAS:]FILE]... [--openapi [ALIAS:]FILE]...
corvint-test e2e shadow --window FILE [--root DIR] [--manifest FILE] [--out FILE]
corvint-test e2e how-accept --answer FILE [--write PATH] [--evidence FILE] [--oracle FILE]
corvint-test e2e how-evidence --answer FILE --prove FILE --validity FILE [--root DIR]
corvint-test e2e discriminate --base REV --head REV --app-dir DIR --test-command JSON [--root DIR] [--tests-dir DIR]
    [--rebuild-command JSON] [--verdict exit|provider] [--max-mutants N] [--wall-time D] [--run-timeout D] [--rebuild-timeout D]
    [--dir-match any|leading] [--exclude-dir NAME]... [--keep-dir NAME]... [--map FILE] [--out FILE]
corvint-test runner runners
corvint-test runner profiles
corvint-test runner plan --executable PATH --executable-sha256 SHA256 --request FILE [--tools FILE] [--out FILE] [--experimental] [--trusted-local]
corvint-test runner run --executable PATH --executable-sha256 SHA256 --plan FILE --approve SHA256 --out FILE --experimental --trusted-local [--tools FILE]
corvint-test falsify plan --experimental
corvint-test falsify execute --experimental --approve-plan DIGEST
corvint-test falsify execute-receipt --experimental --approve-plan DIGEST
corvint-test falsify verify-receipt --experimental --expected-plan DIGEST --expected-tool SHA256
corvint-test minimize plan --experimental
corvint-test minimize execute --experimental --approve-plan DIGEST
corvint-test accept plan --plan FILE
corvint-test accept accept --plan FILE --approve-plan DIGEST --new PATH... --repeat N --environment LABEL [--temp-root DIR]
`

// commandSummaries are the one-line summaries of the groups and their commands.
var commandSummaries = map[string]string{
	"help":                    "Print this binary's usage, or with --json or --tree its command tree",
	"e2e":                     "Compile and prove change-scoped E2E scenarios from admitted, revision-pinned blocks (CCE-V0)",
	"e2e catalog":             "Write the admitted block catalog of the application map at one revision",
	"e2e harvest":             "Harvest candidate blocks from the application map at one revision",
	"e2e obligations":         "List the E2E obligations a base..head change reaches",
	"e2e compile":             "Compile the change's scenarios into an output directory",
	"e2e prove":               "Compile and prove the change's scenarios, or prove one hosted Playwright test",
	"e2e shadow":              "Evaluate a frozen shadow window (CCE-V0-011)",
	"e2e how-accept":          "Accept a site-knowledge stage 7 draft as a candidate test (SKH-V0 stage 8)",
	"e2e how-evidence":        "Bind prove and validity reports to a site-knowledge answer (SKH-V0 stage 8)",
	"e2e discriminate":        "Mutate changed Ruby/TS/JS application hunks and record which external tests kill each mutant (CCE-V0-170..176, 236..238)",
	"runner":                  "Folded corvint-test-runner: plan and run pinned test runners (CTF-V0, TRE-V0)",
	"runner runners":          "List the runner profile IDs as JSON; no executable pin is needed",
	"runner profiles":         "Alias of runners",
	"runner plan":             "Derive a runner plan from a bounded request",
	"runner run":              "Execute an approved runner plan",
	"falsify":                 "Folded corvint-behavior-falsify: plan and execute behavior falsification (CTF-V0)",
	"falsify plan":            "Plan a falsification from the request JSON on stdin",
	"falsify execute":         "Execute exactly the approved falsification plan read on stdin",
	"falsify execute-receipt": "Execute the approved plan read on stdin and write its receipt",
	"falsify verify-receipt":  "Verify the falsification receipt on stdin offline",
	"minimize":                "Folded corvint-playwright-minimize: plan and execute Playwright test minimization (CTF-V0)",
	"minimize plan":           "Plan a minimization from the live request JSON on stdin",
	"minimize execute":        "Execute exactly the approved minimization plan read on stdin",
	"accept":                  "Folded corvint-tests-accept: plan and accept new tests (CTF-V0)",
	"accept plan":             "Validate an acceptance request and print its digest",
	"accept accept":           "Run an approved acceptance request against the new test files",
}

// commandTreeInternal are the dispatched words no synopsis names.
var commandTreeInternal = []clitree.Internal{
	{Word: "worker", Reason: "accept worker the accept group re-enters this executable as (CTF-V0-006)"},
	{Word: "-help", Reason: "single-dash help spelling the runner group accepts"},
	{Word: "--gradle-user-home", Reason: "argument of the pinned Gradle invocation the runner group compares, not a corvint-test option"},
}

var (
	commandTreeOnce  sync.Once
	commandTreeValue clitree.Tree
)

// commandTree is corvint-test's command tree (corvint-command-tree/0).
func commandTree() clitree.Tree {
	commandTreeOnce.Do(func() {
		builder := clitree.NewBuilder("corvint-test")
		builder.AddText(commandHelpText, "")
		for path, summary := range commandSummaries {
			builder.Declare(strings.Fields(path), summary)
		}
		commandTreeValue = builder.Tree("corvint-test help --json", "cmd/corvint-test commandHelpText",
			[]string{"cmd/corvint-test", "internal/testrunner/cli", "internal/behaviorfalsify/cli",
				"internal/playwrightminimize/cli", "internal/testacceptance/cli"}, nil, commandTreeInternal)
	})
	return commandTreeValue
}

func isHelpFlag(arg string) bool { return arg == "--help" || arg == "-h" || arg == "-help" }

// helpRequest answers `help`, `help --json` (or `help --tree`), `help PATH...`, and any invocation whose leading
// words name a node and whose arguments carry --help or -h, before any group parses them, so each
// node prints its own help and exits 0 (CAP-V0-042). It reports false for every other invocation,
// and for a lone --help and the e2e group, whose help run prints (CCE-V0-118).
func helpRequest(args []string, stdout io.Writer) (bool, error) {
	tree := commandTree()
	// --tree is the command-tree spelling corvint itself uses (CAP-V0-950).
	if len(args) == 2 && args[0] == "help" && (args[1] == "--json" || args[1] == "--tree") {
		encoded, err := clitree.Encode(tree)
		if err != nil {
			return true, err
		}
		_, err = stdout.Write(encoded)
		return true, err
	}
	var words []string
	switch {
	case len(args) == 0:
		return false, nil
	case args[0] == "help":
		words = slices.DeleteFunc(slices.Clone(args[1:]), isHelpFlag)
		if len(words) < len(args)-1 && len(words) == 0 {
			words = []string{"help"}
		}
	case args[0] == "e2e" || len(args) == 1 && isHelpFlag(args[0]):
		// run prints the group usage and each e2e step's CCE-V0-118 help from the workflow
		// catalog; `help e2e ...` still prints the tree node.
		return false, nil
	default:
		asked := false
		for _, arg := range args {
			if arg == "--" {
				break
			}
			asked = asked || isHelpFlag(arg)
		}
		if !asked {
			return false, nil
		}
		words = clitree.LeadingWords(args, nil)
	}
	if len(words) == 0 {
		_, err := fmt.Fprint(stdout, rootHelp(tree))
		return true, err
	}
	node, ok := tree.Find(words)
	if !ok || (args[0] == "help" && len(node.Path) != len(words)) {
		return true, fmt.Errorf("unknown command %q; run `corvint-test help`", strings.Join(words, " "))
	}
	_, err := fmt.Fprint(stdout, clitree.Render(tree, node))
	return true, err
}

func rootHelp(tree clitree.Tree) string {
	var text strings.Builder
	text.WriteString("corvint-test: the experimental test companion (CCE-V0, CTF-V0)\n\nUsage:\n")
	text.WriteString(usage + "\n\nCommands:\n")
	for _, node := range tree.Nodes {
		text.WriteString("  " + strings.Join(node.Path, " ") + ": " + node.Summary + "\n")
	}
	text.WriteString("\nRun `corvint-test COMMAND --help` for one command's usage, flags, required inputs and example.\n")
	text.WriteString("Every command of corvint-test: " + tree.HelpJSON + "\n")
	return text.String()
}
