package main

import (
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// capabilityRegistry assembles the binary's own command surface for the
// corvint-capability-catalog/0 document (CAP-V0-015): the dispatch list, root
// help, each command's help topics and the code-declared operation tables.
func capabilityRegistry() capcatalog.Registry {
	registry := capcatalog.Registry{
		RootHelp:         rootHelp,
		Commands:         slices.Clone(topLevelCommands),
		Topics:           map[string]string{},
		Operations:       map[string][]string{},
		NestedOperations: map[string]map[string][]string{},
		Workflows:        []capcatalog.WorkflowSource{e2eAdoptionWorkflow},
		HiddenCommands:   hiddenCommands,
	}
	for _, command := range topLevelCommands {
		topic, _ := publicHelpTopic(command)
		registry.Topics[command] = helpText(topic)
	}
	corpus := make([]string, 0, len(corpusOperations))
	docsTopics := []string{registry.Topics["docs"], helpText("docs-corpus"), helpText("docs-flows"), helpText("docs-maintain")}
	for _, operation := range corpusOperations {
		corpus = append(corpus, operation.name)
		docsTopics = append(docsTopics, docsCorpusOperationHelp(operation.name))
	}
	registry.Topics["docs"] = strings.Join(docsTopics, "\n")
	registry.NestedOperations["docs"] = map[string][]string{"corpus": corpus}
	registry.Operations["docs"] = slices.Clone(docsSubcommands)
	registry.Operations["lease"] = slices.Clone(leaseActions)
	for name := range flowSubcommands {
		registry.Operations["flows"] = append(registry.Operations["flows"], name)
	}
	for _, nested := range nestedHelpTopics {
		registry.NestedHelpTopics = append(registry.NestedHelpTopics, nested[0])
	}
	for action := range cemHelpActions {
		registry.NestedHelpTopics = append(registry.NestedHelpTopics, "cem "+action)
	}
	for command, choices := range helpSubcommands {
		for choice := range choices {
			registry.Operations[command] = append(registry.Operations[command], choice)
		}
	}
	return registry
}

// capabilityHelpTopics are `corvint help --json`, `corvint help --tree` and `corvint help guide`: help topics, not
// commands, so the unknown-topic refusal keeps listing commands only (CAP-V0-015, CAP-V0-017).
var capabilityHelpTopics = map[string]string{"--json": "catalog-json", "--tree": "tree-json", "guide": "guide"}

// capabilityOrPublicHelpTopic resolves the one word after `corvint help`.
func capabilityOrPublicHelpTopic(word string) string {
	if topic, ok := capabilityHelpTopics[word]; ok {
		return topic
	}
	topic, _ := publicHelpTopic(word)
	return topic
}

// capabilityHelpText is helpText's fallback for the capability topics.
func capabilityHelpText(topic string) string {
	switch topic {
	case "catalog-json":
		return capabilityCatalogJSON()
	case "guide":
		return capcatalog.Guide()
	case "tree-json":
		return commandTreeJSON()
	default:
		panic("unknown help topic")
	}
}

// capabilityCatalogJSON is `corvint help --json` (CAP-V0-015, CAP-V0-016); it carries the
// profiling member unchanged (PRB-V0-009).
func capabilityCatalogJSON() string {
	catalog := capcatalog.Build(capabilityRegistry())
	catalog.Profiling = profileKnobCatalog()
	catalog.CommandTrees = commandTreeRefs
	encoded, err := capcatalog.Encode(catalog)
	if err != nil {
		panic("capability catalog encoding: " + err.Error())
	}
	return string(encoded)
}

// catalogCommandHelpPrefix marks `corvint help --json COMMAND`, the one-command
// slice of the catalog (CAP-V0-950).
const catalogCommandHelpPrefix = "catalog-command:"

// catalogCommandJSON is `corvint help --json COMMAND`: the command's entry of
// `corvint help --json`, at most capcatalog.MaxCommandSliceBytes.
func catalogCommandJSON(command string) string {
	encoded, _, ok := capcatalog.Slice([]byte(capabilityCatalogJSON()), command)
	if !ok {
		panic("capability catalog slice: " + command)
	}
	return string(encoded)
}

// unknownCatalogCommand refuses `corvint help --json NAME` for a NAME that is
// no top-level command, naming every command.
func unknownCatalogCommand(name string) error {
	quoted := make([]string, 0, len(topLevelCommands))
	for _, command := range topLevelCommands {
		quoted = append(quoted, pythonRepr(command))
	}
	return argumentError("unknown command for help --json: " + pythonRepr(name) + " (choose from " + strings.Join(quoted, ", ") + ")")
}
