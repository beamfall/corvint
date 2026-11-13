// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/clitree"
	flowscmd "github.com/Beamfall/corvint/internal/command/flows"
	"github.com/Beamfall/corvint/internal/mcp/docsserver"
)

// rootHelp is `corvint-docs --help`. The corpus join is prose, not a synopsis, because the
// words after --corpus=FILE are the core's verbs, not this binary's.
const rootHelp = `usage: corvint-docs [--root PATH] docs SUBCOMMAND [OPTIONS]
       corvint-docs [--root PATH] flows [SUBCOMMAND] [OPTIONS]
       corvint-docs [--root PATH] how [OPTIONS] GOAL
       corvint-docs mcp --root ABSOLUTE_ROOT [--corpus-artifact PATH]
       corvint-docs help docs | docs check | docs corpus | docs corpus OPERATION | docs flows | docs maintain | flows | how | mcp | docs-tool
       corvint-docs help --json
Corvint's documentation app (decision 0594): drafting pinned documentation, the documentation
corpus, generated-block maintenance and flow documentation, and the application-flow group
(flows, and how, its alias of flows appmap how; decision 0595), and the documentation MCP server
(mcp; MTT-V1-004). Every subcommand is experimental.
` + "`corvint docs ...`, `corvint flows ...`, `corvint how ...` and `corvint --corpus=FILE ...` forward to\n" +
	"this executable unchanged, so the two spellings print the same bytes; only\n" +
	"`flows test-evidence` stays in corvint.\n" +
	`With --corpus=FILE [--corpus-depth=N] before a core read (impact, affected or
test-validity), corvint-docs runs that read in the corvint executable beside it or on PATH and
joins the corpus receipt onto its output.
Required: a Git repository at --root (default .) for every verb but the corpus operations that
read only their input files.
Writes: docs maintain --apply and docs flows finalize write the pages they name; flows record,
flows docs maintain --apply and the flows ingest verbs write the files they name; docs draft and
docs consume best-effort append unsupported-* failures to .corvint/self-observations.jsonl
(SOL-V0-007); every other verb reads.
Example: corvint-docs docs corpus search --artifact corpus.json --query TERM
`

// docsToolHelp documents the protocol verb corvint-mcp's docs bridge runs.
const docsToolHelp = `Answer one MCP documentation tool call for corvint-mcp (protocol verb).

Usage:
  corvint-docs docs-tool --root ABSOLUTE_ROOT TOOL < arguments.json

TOOL is corvint.docs_draft or corvint.docs_consume. stdin carries the tool's JSON arguments
(at most 192 KiB); stdout is one corvint-docs-tool/1 document with exactly one of
"structured" (with "text"), "failure" (a refusal the server reports as a tool error) or
"error" (a transport failure). A refusal still exits 0: a non-zero exit means the process,
not the call, failed. ` + "`corvint-docs mcp` and `corvint-docs-mcp`" + ` run this verb for every
docs_draft and docs_consume call; nothing else needs it.

Example:
  corvint-docs docs-tool --root /absolute/repo corvint.docs_draft < arguments.json
`

// helpBooleanFlags take no value, so a `--help` after one is still a help request.
var helpBooleanFlags = map[string]bool{"--apply": true, "--check": true, "--dry-run": true, "--owner-stdin": true, "--trusted-project": true, "--verify-rederive": true, "--timings": true}

// nestedHelpTopics are the `help docs X` topics with authored text of their own.
var nestedHelpTopics = [][2]string{
	{"docs corpus", "docs-corpus"},
	{"docs check", "docs-check"},
	{"docs flows", "docs-flows"},
	{"docs maintain", "docs-maintain"},
}

// parseHelpInvocation recognises every help form corvint recognised for its docs group, with
// this binary's root and docs-tool topics: `--help`, `-h` or `help` alone; `help --json` and
// `help --tree`; `help docs [corpus [OPERATION]|flows|maintain]`; `help flows`, `help how`,
// `help mcp` and `help docs-tool`; `help flows appmap SUB` and a `--help` or `-h` anywhere before
// `--` after a `flows appmap SUB` command (AMAP-V0-975); and a `--help` after a docs, flows, how or
// mcp verb. An unknown help topic is an error; anything else is not help.
func parseHelpInvocation(arguments []string) (string, bool, error) {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", false, nil
			}
			index += 2
		} else {
			index++
		}
	}
	rest := arguments[index:]
	switch {
	case len(rest) == 0:
		return "", false, nil
	case rest[0] == "--help", rest[0] == "-h", len(rest) == 1 && rest[0] == "help":
		return "root", true, nil
	case len(rest) == 2 && rest[0] == "help" && (rest[1] == "--json" || rest[1] == "--tree"):
		return "tree-json", true, nil
	case rest[0] == "help":
		words := strings.Fields(strings.Join(rest[1:], " "))
		if topic := helpWordsTopic(words); topic != "" {
			return topic, true, nil
		}
		if topic := appmapNodeTopic(words); topic != "" && topic == "node:"+strings.Join(words, " ") {
			return topic, true, nil
		}
		return "", true, unknownHelpTopic(rest[1:])
	}
	if topic := appmapNodeTopic(rest); topic != "" && helpTokenBeforeEnd(rest) {
		return topic, true, nil
	}
	topic := commandHelpTopic(rest)
	if topic == "" || !helpFlagRequested(rest[1:]) {
		return "", false, nil
	}
	return topic, true, nil
}

// helpWordsTopic names the topic of `help WORDS...`, or "" for an unknown path.
func helpWordsTopic(words []string) string {
	switch {
	case len(words) == 1 && (words[0] == "docs" || words[0] == "flows" || words[0] == "how" || words[0] == "mcp"):
		return words[0]
	case len(words) == 1 && words[0] == toolVerb:
		return "docs-tool"
	case len(words) == 2 && words[0] == "docs":
		if words[1] == "corpus" {
			return "docs-corpus"
		}
		if slices.Contains(docsSubcommands, words[1]) {
			return nestedTopicOr(words[0], words[1], "docs")
		}
	case len(words) == 3 && words[0] == "docs" && words[1] == "corpus":
		if _, ok := lookupCorpusOperation(words[2]); ok {
			return "docs-corpus-" + words[2]
		}
	}
	return ""
}

func nestedTopicOr(command, subcommand, fallback string) string {
	for _, nested := range nestedHelpTopics {
		if nested[0] == command+" "+subcommand {
			return nested[1]
		}
	}
	return fallback
}

// commandHelpTopic names the help topic of a verb invocation, or "" when the first word is
// not a verb of this binary or its nested choice is invalid.
func commandHelpTopic(rest []string) string {
	switch {
	case rest[0] == toolVerb:
		return "docs-tool"
	case rest[0] == "flows", rest[0] == "how", rest[0] == "mcp":
		return rest[0]
	case rest[0] != "docs":
		return ""
	case len(rest) > 1 && rest[1] == "corpus":
		return corpusHelpTopic(rest[2:])
	case len(rest) > 1 && slices.Contains(docsSubcommands, rest[1]):
		return nestedTopicOr(rest[0], rest[1], "docs")
	case len(rest) > 1 && !argparseOptionLike(rest[1]):
		return ""
	}
	return "docs"
}

// appmapNodeTopic names the help topic `node:flows appmap SUB` when the leading words of rest are
// exactly a `flows appmap SUB` command of this binary's tree, or "" (AMAP-V0-975).
func appmapNodeTopic(rest []string) string {
	words := clitree.LeadingWords(rest, nil)
	if len(words) < 3 || words[0] != "flows" || words[1] != "appmap" {
		return ""
	}
	if node, ok := commandTree().Find(words); !ok || len(node.Path) != len(words) {
		return ""
	}
	return "node:" + strings.Join(words, " ")
}

// helpTokenBeforeEnd reports whether `--help` or `-h` appears anywhere before `--` (AMAP-V0-975).
func helpTokenBeforeEnd(tokens []string) bool {
	for _, token := range tokens {
		switch token {
		case "--":
			return false
		case "--help", "-h":
			return true
		}
	}
	return false
}

func helpFlagRequested(tokens []string) bool {
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case token == "--":
			return false
		case token == "--help", token == "-h":
			return true
		case !argparseOptionLike(token), strings.Contains(token, "="), helpBooleanFlags[token]:
			continue
		case index+1 < len(tokens) && !argparseOptionLike(tokens[index+1]):
			index++
		default:
			return false
		}
	}
	return false
}

func unknownHelpTopic(topic []string) error {
	quoted := []string{pythonRepr("docs"), pythonRepr("flows"), pythonRepr("how"), pythonRepr("mcp"), pythonRepr(toolVerb)}
	for _, nested := range nestedHelpTopics {
		quoted = append(quoted, pythonRepr(nested[0]))
	}
	return argumentError("unknown help topic: " + pythonRepr(strings.Join(topic, " ")) + " (choose from " + strings.Join(quoted, ", ") + ")")
}

// helpText is the text one topic prints.
func helpText(topic string) string {
	if topic == "tree-json" {
		return commandTreeJSON()
	}
	if words, ok := strings.CutPrefix(topic, "node:"); ok {
		node, _ := commandTree().Find(strings.Fields(words))
		return clitree.Render(commandTree(), node)
	}
	return topicText(topic)
}

func topicText(topic string) string {
	if operation, ok := strings.CutPrefix(topic, "docs-corpus-"); ok {
		return docsCorpusOperationHelp(operation)
	}
	switch topic {
	case "root":
		return rootHelp
	case "docs":
		return docsHelp + "\n" + docsMaintainHelp + "\n" + docsFlowsHelp
	case "docs-corpus":
		return docsCorpusHelp()
	case "docs-check":
		return docsCheckHelp
	case "docs-flows":
		return docsFlowsHelp
	case "docs-maintain":
		return docsMaintainHelp
	case "docs-tool":
		return docsToolHelp
	case "flows":
		return flowsHelp
	case "how":
		return flowscmd.HowHelp
	case "mcp":
		return docsserver.Usage
	}
	panic("unknown help topic " + topic)
}

// helpTopics lists every authored topic that documents a command path, in a stable order.
func helpTopics() []string {
	topics := []string{"docs", "docs-check", "docs-corpus", "docs-flows", "docs-maintain", "docs-tool", "flows", "how", "mcp"}
	for _, operation := range corpusOperations {
		topics = append(topics, "docs-corpus-"+operation.name)
	}
	return topics
}

// helpTopicPath is the command path a help topic documents.
func helpTopicPath(topic string) []string {
	if operation, ok := strings.CutPrefix(topic, "docs-corpus-"); ok {
		return []string{"docs", "corpus", operation}
	}
	switch topic {
	case "docs-corpus":
		return []string{"docs", "corpus"}
	case "docs-check":
		return []string{"docs", "check"}
	case "docs-flows":
		return []string{"docs", "flows"}
	case "docs-maintain":
		return []string{"docs", "maintain"}
	case "docs-tool":
		return []string{toolVerb}
	}
	return []string{topic}
}

// topicSummary is a topic's first prose line: the sentence before its synopsis.
func topicSummary(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		lowered := strings.ToLower(line)
		if line == "" || strings.HasPrefix(lowered, "usage") || strings.HasPrefix(line, binaryName) {
			continue
		}
		return strings.TrimSuffix(line, ".")
	}
	return ""
}

// commandTreeInternal are the dispatched words no synopsis names (CAP-V0-043): the spaced
// spellings of the audit operations, and the core verbs the corpus join recognises after
// --corpus=FILE, which belong to corvint, not to this binary.
var commandTreeInternal = []clitree.Internal{
	{Word: "audit", Reason: "spaced spelling of docs corpus audit-sample and audit-report"},
	{Word: "sample", Reason: "spaced spelling of docs corpus audit-sample"},
	{Word: "report", Reason: "spaced spelling of docs corpus audit-report"},
	{Word: "impact", Reason: "core read the --corpus=FILE join runs in corvint"},
	{Word: "affected", Reason: "core read the --corpus=FILE join runs in corvint"},
	{Word: "test-validity", Reason: "core read the --corpus=FILE join runs in corvint"},
}

// commandTree is this binary's command tree, derived from the help topics it prints, so no
// node can exist without help (CAP-V0-040, CAP-V0-041).
var commandTree = sync.OnceValue(func() clitree.Tree {
	builder := clitree.NewBuilder(binaryName)
	for _, topic := range helpTopics() {
		text := topicText(topic)
		builder.Declare(helpTopicPath(topic), topicSummary(text))
		builder.AddText(text, "")
	}
	return builder.Tree(binaryName+" help --json", "cmd/"+binaryName+" help topics", []string{"cmd/" + binaryName},
		[]string{"--root", "--version", "--json", "--tree", "--corpus", "--corpus-depth"}, commandTreeInternal)
})

// commandTreeJSON is `corvint-docs help --json`, the corvint-command-tree/0 document.
func commandTreeJSON() string {
	encoded, err := clitree.Encode(commandTree())
	if err != nil {
		panic("command tree encoding: " + err.Error())
	}
	return string(encoded)
}
