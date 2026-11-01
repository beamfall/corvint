package main

import (
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clitree"
)

// nodeHelpPrefix marks the help topic of one subcommand below a command's own topic, such as
// `corvint flows appmap build --help` (CAP-V0-042).
const nodeHelpPrefix = "node:"

// commandTreeInternal are the dispatched words and options no synopsis names because no operator
// or agent types them: hook and worker entry points the binary or its installed hooks invoke, test
// fixtures, hook event values, and alternative spellings (CAP-V0-043).
var commandTreeInternal = []clitree.Internal{
	{Word: "native-hook", Reason: "host hook entry point the installed adapter invokes"},
	{Word: "--qualified-lifecycle", Reason: "native-hook option the installed adapter passes"},
	{Word: "--qualified-direct-lifecycle", Reason: "native-hook option the installed adapter passes"},
	{Word: "authority-event", Reason: "host hook event forwarded by the installed adapter"},
	{Word: "--native-output", Reason: "authority-event option the installed adapter passes"},
	{Word: "qualified-event", Reason: "host hook event forwarded by the installed adapter"},
	{Word: "dogfood-outcome-worker", Reason: "worker process dogfood starts for itself"},
	{Word: "dogfood-verifier-worker", Reason: "worker process dogfood starts for itself"},
	{Word: "--aggregate-over-limit", Reason: "dogfood-record option dogfood change passes for itself (DCW-V0-374)"},
	{Word: "dogfood-observe", Reason: "self-observation ledger writer the dogfood hooks invoke (SOL-V0-001)"},
	{Word: "--step", Reason: "dogfood-observe option"},
	{Word: "--status", Reason: "dogfood-observe option"},
	{Word: "frontier-next", Reason: "experimental frontier/2 stub with no admitted deployment adapter"},
	{Word: "--enrollment", Reason: "frontier-next option"},
	{Word: "plan-fixture", Reason: "work test fixture entry point"},
	{Word: "--executor", Reason: "work plan-fixture option"},
	{Word: "--observations", Reason: "work plan-fixture option"},
	{Word: "pre-compact", Reason: "claude-code EVENT value of adapter"},
	{Word: "post-compact", Reason: "claude-code EVENT value of adapter"},
	{Word: "audit", Reason: "spaced spelling of docs corpus audit-sample and audit-report"},
	{Word: "sample", Reason: "spaced spelling of docs corpus audit-sample"},
}

// commandTreeRefs are the command trees `corvint help --json` links: corvint's own and each
// companion binary's with subcommands. The capabilities check reads the snapshots (CAP-V0-045).
var commandTreeRefs = []capcatalog.CommandTreeRef{
	{Binary: "corvint", Command: "corvint help --tree", Snapshot: "internal/capcatalog/trees/corvint.json"},
	{Binary: "corvint-cem-candidate", Command: "corvint-cem-candidate help --json", Snapshot: "internal/capcatalog/trees/corvint-cem-candidate.json"},
	{Binary: "corvint-cem-experiments", Command: "corvint-cem-experiments help --json", Snapshot: "internal/capcatalog/trees/corvint-cem-experiments.json"},
	{Binary: "corvint-tasks", Command: "corvint-tasks help --json", Snapshot: "internal/capcatalog/trees/corvint-tasks.json"},
	{Binary: "corvint-test", Command: "corvint-test help --json", Snapshot: "internal/capcatalog/trees/corvint-test.json"},
}

// commandTreeJSON is `corvint help --tree`, the canonical corvint-command-tree/0 document.
func commandTreeJSON() string {
	encoded, err := clitree.Encode(commandTree())
	if err != nil {
		panic("command tree encoding: " + err.Error())
	}
	return string(encoded)
}

// helpTopics lists every authored help topic that documents commands, in a stable order: each
// public command, each cem action, and each nested docs and harness topic.
func helpTopics() []string {
	topics := []string{}
	for _, command := range topLevelCommands {
		topic, _ := publicHelpTopic(command)
		if topic != "harness-event" {
			topics = append(topics, topic)
		}
	}
	for _, action := range slices.Sorted(func(yield func(string) bool) {
		for action := range cemHelpActions {
			if !yield(action) {
				return
			}
		}
	}) {
		topics = append(topics, cemActionHelpPrefix+action)
	}
	topics = append(topics, "harness-event", "docs-corpus", "docs-flows", "docs-maintain", flowsAppmapBuildHelpTopic)
	for _, operation := range corpusOperations {
		topics = append(topics, "docs-corpus-"+operation.name)
	}
	return topics
}

// helpTopicPath is the command path a help topic documents.
func helpTopicPath(topic string) []string {
	if action, ok := strings.CutPrefix(topic, cemActionHelpPrefix); ok {
		return []string{"cem", action}
	}
	if operation, ok := strings.CutPrefix(topic, "docs-corpus-"); ok {
		return []string{"docs", "corpus", operation}
	}
	switch topic {
	case "docs-corpus":
		return []string{"docs", "corpus"}
	case "docs-flows":
		return []string{"docs", "flows"}
	case "docs-maintain":
		return []string{"docs", "maintain"}
	case "harness-event":
		return []string{"harness", "event"}
	case flowsAppmapBuildHelpTopic:
		return []string{"flows", "appmap", "build"}
	}
	return []string{topic}
}

// topicSummary is a topic's first prose line: the sentence before its synopsis.
func topicSummary(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		lowered := strings.ToLower(line)
		if line == "" || strings.HasPrefix(lowered, "usage") || strings.HasPrefix(line, "corvint") {
			continue
		}
		return strings.TrimSuffix(line, ".")
	}
	return ""
}

// commandTree is corvint's command tree, derived from the help topics the binary prints, so no
// node can exist without help (CAP-V0-040, CAP-V0-041).
func commandTree() clitree.Tree {
	commandTreeOnce.Do(func() { commandTreeValue = buildCommandTree() })
	return commandTreeValue
}

var (
	commandTreeOnce  sync.Once
	commandTreeValue clitree.Tree
)

func buildCommandTree() clitree.Tree {
	builder := clitree.NewBuilder("corvint")
	for _, topic := range helpTopics() {
		text := topicText(topic)
		summary := topicSummary(text)
		builder.Declare(helpTopicPath(topic), summary)
		builder.AddText(text, "")
	}
	return builder.Tree("corvint help --tree", "cmd/corvint help topics", []string{"cmd/corvint"},
		[]string{"--root", "--version", "--json"}, commandTreeInternal)
}

// findTreeNode returns the node whose path is exactly words.
func findTreeNode(words []string) (clitree.Node, bool) {
	node, ok := commandTree().Find(words)
	return node, ok && len(node.Path) == len(words)
}

// nodeHelpTopic returns the node topic for an invocation whose leading words name a subcommand
// deeper than the resolved topic documents, or "" when the topic already is that command's own.
func nodeHelpTopic(rest []string, topic string) string {
	node, ok := commandTree().Find(clitree.LeadingWords(rest, nil))
	if !ok || len(node.Path) <= len(helpTopicPath(topic)) {
		return ""
	}
	return nodeHelpPrefix + strings.Join(node.Path, " ")
}

func nodeHelpText(path string) string {
	node, ok := findTreeNode(strings.Fields(path))
	if !ok {
		panic("unknown command tree node")
	}
	return clitree.Render(commandTree(), node) + parentTopicPointer(node.Path)
}

// parentTopicPointer names the authored help topic that documents a node's command in full, so a
// subcommand's derived help still leads to its prose.
func parentTopicPointer(path []string) string {
	best := []string{}
	for _, topic := range helpTopics() {
		if documented := helpTopicPath(topic); len(documented) > len(best) && len(documented) < len(path) &&
			slices.Equal(documented, path[:len(documented)]) {
			best = documented
		}
	}
	if len(best) == 0 {
		return ""
	}
	return "Full topic: corvint help " + strings.Join(best, " ") + "\n"
}

// helpExampleTrailer gives a command topic its required inputs and, when its text has none, one
// example, both derived from its tree node (CAP-V0-044).
func helpExampleTrailer(topic string) string {
	switch topic {
	case "root", "catalog-json", "tree-json", "guide":
		return ""
	}
	node, ok := findTreeNode(helpTopicPath(topic))
	if !ok {
		return ""
	}
	trailer := "\nRequired: " + strings.Join(orNone(node.Required), ", ") + "\n"
	if !authoredExample.MatchString(topicText(topic)) {
		trailer += "\nExample:\n  " + node.Example + "\n"
	}
	return trailer
}

// authoredExample matches a topic's own Example or Examples heading.
var authoredExample = regexp.MustCompile(`(?m)^\s*Examples?:`)

func orNone(items []string) []string {
	if len(items) == 0 {
		return []string{"none"}
	}
	return items
}
