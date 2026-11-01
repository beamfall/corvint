// Package capcatalog derives the machine-readable Corvint command catalog
// (corvint-capability-catalog/0) from the command registry and help text the
// binary already carries, and holds the task-indexed agent guide that ships in
// the binary (CAP-V0-015 to CAP-V0-021, docs/specs/capability-catalog-v0.md).
//
// Every field is read from the registry or the help text. A field the registry
// cannot answer is null and named in the command's unknowns with a reason; it
// is never filled with a hand-written claim.
package capcatalog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Schema names the catalog document.
const Schema = "corvint-capability-catalog/0"

// RequiredFlagsPartial is the only requiredFlagsCoverage value (CAP-V0-022):
// the binary declares no structured required-option table for most usage
// forms, so requiredFlags is a lower bound read from Usage synopses.
const RequiredFlagsPartial = "partial"

//go:embed AGENT-GUIDE.md
var guide string

//go:embed catalog.json
var snapshot []byte

// Guide returns the task-indexed agent guide shipped in the binary.
func Guide() string { return guide }

// Snapshot returns the committed catalog bytes, which a test keeps equal to
// `corvint help --json` (CAP-V0-019), so a binary that does not link the
// command registry can serve the same catalog.
func Snapshot() []byte { return slices.Clone(snapshot) }

// Registry is the binary's own command surface: the dispatch list, the root
// help, each command's help text and its code-declared operation tables.
type Registry struct {
	RootHelp   string
	Commands   []string
	Topics     map[string]string
	Operations map[string][]string
	// NestedOperations holds code-declared tables one level below an
	// operation: NestedOperations["docs"]["corpus"] lists the corpus operations.
	NestedOperations map[string]map[string][]string
	// NestedHelpTopics are the two-word `corvint help COMMAND SUBCOMMAND`
	// topics the help dispatcher accepts, such as "docs corpus".
	NestedHelpTopics []string
	// Workflows are the code-declared multi-command chains (CAP-V0-030).
	Workflows []WorkflowSource
	// HiddenCommands are the dispatched verbs root help leaves unlisted, each with a
	// root-help-style line: a gloss and its owning spec in trailing parentheses (CAP-V0-650).
	HiddenCommands map[string]string
}

// Unknown names a field the registry cannot answer and why.
type Unknown struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Command is one top-level verb.
type Command struct {
	Name                  string              `json:"name"`
	HelpTopic             string              `json:"helpTopic"`
	Purpose               *string             `json:"purpose"`
	Maturity              *string             `json:"maturity"`
	OwningSpec            *string             `json:"owningSpec"`
	FreezeContract        *string             `json:"freezeContract"`
	Operations            []string            `json:"operations"`
	NestedOperations      map[string][]string `json:"nestedOperations"`
	RequiredFlags         map[string][]string `json:"requiredFlags"`
	Flags                 []string            `json:"flags"`
	SchemaIDs             []string            `json:"schemaIds"`
	Inputs                *[]string           `json:"inputs"`
	Outputs               *[]string           `json:"outputs"`
	ExitCodesMentioned    []int               `json:"exitCodesMentioned"`
	Writes                *string             `json:"writes"`
	SelfObservationLedger string              `json:"selfObservationLedger"`
	WriteStatements       []string            `json:"writeStatements"`
	Unknowns              []Unknown           `json:"unknowns"`
}

// Catalog is the corvint-capability-catalog/0 document.
type Catalog struct {
	Schema     string            `json:"schema"`
	Source     string            `json:"source"`
	Derivation map[string]string `json:"derivation"`
	// RequiredFlagsCoverage tells a consumer that requiredFlags is partial,
	// so an omitted form or option is not evidence that none is required.
	RequiredFlagsCoverage string    `json:"requiredFlagsCoverage"`
	GlobalFlags           []string  `json:"globalFlags"`
	GlobalSchemaIDs       []string  `json:"globalSchemaIds"`
	NestedHelpTopics      []string  `json:"nestedHelpTopics"`
	Guide                 GuideRef  `json:"guide"`
	Commands              []Command `json:"commands"`
	// Workflows are the ordered multi-command chains, additive under /0 (CAP-V0-030).
	Workflows []Workflow `json:"workflows"`
	// Profiling is the PRB-V0-009 profile-knob member, supplied by the binary and carried unchanged.
	Profiling map[string]any `json:"profiling,omitempty"`
	// CommandTrees names each binary's machine-readable command tree: every subcommand at every
	// depth with its usage, flags, required inputs and example (CAP-V0-045).
	CommandTrees []CommandTreeRef `json:"commandTrees,omitempty"`
}

// CommandTreeRef is one binary's command tree: the command printing it and the committed snapshot
// of that output.
type CommandTreeRef struct {
	Binary   string `json:"binary"`
	Command  string `json:"command"`
	Snapshot string `json:"snapshot"`
}

// GuideRef binds the shipped guide by digest.
type GuideRef struct {
	Topic  string `json:"topic"`
	SHA256 string `json:"sha256"`
}

var derivation = map[string]string{
	"globalFlags":           "every --option the root help Global options block names, including the --json modifier of --version",
	"globalSchemaIds":       "every NAME/VERSION schema identifier the root help Global options block names",
	"nestedOperations":      "the code-declared operation tables one level below an operation, such as docs corpus",
	"nestedHelpTopics":      "the two-word corvint help COMMAND SUBCOMMAND topics the help dispatcher accepts",
	"requiredFlags":         "per usage form (the command and its catalogued operation words), the --options every Usage block line of that form names outside [ ] and ( ); a form whose lines require none is omitted, and options a help text states as required only in prose are not derived; see requiredFlagsCoverage",
	"requiredFlagsCoverage": "always partial: requiredFlags reads only Usage synopses, and the binary declares no structured required-option table for most forms, so an option that help prose or the parser requires (one of two alternatives, or one required only in some mode) is missing; an absent form or option does not mean none is required, so treat requiredFlags as a lower bound and read corvint help COMMAND before relying on an omission",
	"purpose":               "the command's label in the root help Commands block",
	"maturity":              "core under the root help Core header, experimental under the Experimental header",
	"owningSpec":            "the parenthesized spec code ending the command's root help line",
	"freezeContract":        "the spec codes named in the root help Core header",
	"operations":            "the code-declared subcommand tables plus the lowercase word after the command in each help usage line",
	"flags":                 "every --option token in the command's help text, without the global --root and --help",
	"schemaIds":             "every NAME/VERSION schema identifier the command's help text names; input and output are not separated",
	"exitCodesMentioned":    "each exit status the help text states after the word exit; incomplete by construction",
	"writes":                "none when the root help Support boundary lists the command as reading without mutating, local-state when it lists the command (in at least one mode) as writing local CEM artifacts or local state, mode-dependent when it lists both",
	"selfObservationLedger": "best-effort-append when the root help Support boundary lists the command as appending to .corvint/self-observations.jsonl",
	"writeStatements":       "sentences of the command's help text that mention writing, mutation or read-only behaviour",
	"commandTrees":          "each binary with subcommands, the command printing its corvint-command-tree/0 document (every subcommand at every depth with its usage, flags, required inputs and example) and the committed snapshot of it",
	"workflows":             "the code-declared multi-command chains in adoption order; each step's usage, inputs, commit, produces, options and example are the sections of the help the step's command prints for --help, its flags, requiredFlags and schemaIds are read from that help as for commands, requires names earlier steps, chain lists the required steps, and a step with a null command is a repository state to write or commit",
}

const ioUnknownReason = "COMMAND --help does not separate input from output schemas; schemaIds lists every schema identifier it names"

var (
	flagPattern     = regexp.MustCompile(`--[a-z][a-z0-9]*(?:-[a-z0-9]+)*`)
	schemaPattern   = regexp.MustCompile(`[a-z][a-z0-9]*(?:[-.][a-z0-9]+)*/[0-9]+(?:\.[0-9]+)?`)
	exitPattern     = regexp.MustCompile(`(?i)\bexit(?:s|ing)?(?:\s+(?:status|code|with))*\s*([0-9])((?:\s*(?:,|/|or|and)\s*[0-9]\b)*)`)
	digitPattern    = regexp.MustCompile(`[0-9]`)
	specCodePattern = regexp.MustCompile(`[A-Z][A-Z0-9]*-V[0-9]+`)
	writePattern    = regexp.MustCompile(`(?i)\b(writes?|written|writing|mutat\w*|read-only)\b`)
)

type rootEntry struct {
	purpose, maturity, spec string
}

// Build derives the catalog. It reads nothing but its argument.
func Build(registry Registry) Catalog {
	entries, freeze := parseRootCommands(registry.RootHelp)
	for name, line := range registry.HiddenCommands {
		if _, listed := entries[name]; !listed {
			entries[name] = parseRootLabel(line, "experimental")
		}
	}
	readOnly, writers, ledger := parseSupportBoundary(registry.RootHelp)
	catalog := Catalog{
		Schema:     Schema,
		Source:     "corvint help: the topLevelCommands dispatch list, the root help and each COMMAND --help topic",
		Derivation: derivation,
		// Never derived from prose: help sentences that call an option required
		// are not parsed, so the list stays a lower bound (CAP-V0-023).
		RequiredFlagsCoverage: RequiredFlagsPartial,
		GlobalFlags:           flags(globalBlock(registry.RootHelp), true),
		GlobalSchemaIDs:       schemaIDs(globalBlock(registry.RootHelp)),
		NestedHelpTopics:      sortedTopics(registry.NestedHelpTopics),
		Guide:                 GuideRef{Topic: "corvint help guide", SHA256: digest(guide)},
		Commands:              make([]Command, 0, len(registry.Commands)),
		Workflows:             make([]Workflow, 0, len(registry.Workflows)),
	}
	for _, source := range registry.Workflows {
		catalog.Workflows = append(catalog.Workflows, BuildWorkflow(source))
	}
	for _, name := range registry.Commands {
		text := registry.Topics[name]
		command := Command{
			Name:                  name,
			HelpTopic:             "corvint help " + name,
			Operations:            operations(name, text, registry.Operations[name]),
			NestedOperations:      nestedOperations(registry.NestedOperations[name]),
			Flags:                 flags(text, false),
			SchemaIDs:             schemaIDs(text),
			ExitCodesMentioned:    exitCodes(text),
			SelfObservationLedger: "not-listed",
			WriteStatements:       writeStatements(text),
			Unknowns:              []Unknown{},
		}
		command.RequiredFlags = requiredFlags(command, text)
		if entry, ok := entries[name]; ok {
			command.Purpose, command.Maturity = stringPointer(entry.purpose), stringPointer(entry.maturity)
			if entry.spec != "" {
				command.OwningSpec = stringPointer(entry.spec)
			}
			if entry.maturity == "core" && freeze != "" {
				command.FreezeContract = stringPointer(freeze)
			}
		} else {
			command.unknown("purpose", "the root help Commands block has no line for this command")
			command.unknown("maturity", "the root help Commands block has no line for this command")
		}
		if command.OwningSpec == nil {
			reason := "the root help line names no owning spec"
			if entry, ok := entries[name]; ok && entry.maturity == "core" {
				reason = "Core verbs name no per-verb spec in the root help; freezeContract names the compatibility freeze"
			}
			command.unknown("owningSpec", reason)
		}
		command.unknown("inputs", ioUnknownReason)
		command.unknown("outputs", ioUnknownReason)
		if len(command.ExitCodesMentioned) == 0 {
			command.unknown("exitCodesMentioned", "the help text states no exit status")
		}
		switch {
		case readOnly[name] && writers[name]:
			command.Writes = stringPointer("mode-dependent")
		case readOnly[name]:
			command.Writes = stringPointer("none")
		case writers[name]:
			command.Writes = stringPointer("local-state")
		default:
			command.unknown("writes", "the root help Support boundary does not classify this command; see writeStatements")
		}
		if ledger[name] {
			command.SelfObservationLedger = "best-effort-append"
		}
		catalog.Commands = append(catalog.Commands, command)
	}
	return catalog
}

// Encode renders the catalog as deterministic two-space indented JSON with a
// terminal LF.
func Encode(catalog Catalog) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(catalog); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (command *Command) unknown(field, reason string) {
	command.Unknowns = append(command.Unknowns, Unknown{Field: field, Reason: reason})
}

func stringPointer(value string) *string { return &value }

func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// block returns the lines after the line equal to header up to the next blank line.
func block(text, header string) []string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if line != header {
			continue
		}
		var body []string
		for _, next := range lines[index+1:] {
			if strings.TrimSpace(next) == "" {
				break
			}
			body = append(body, next)
		}
		return body
	}
	return nil
}

func parseRootCommands(rootHelp string) (map[string]rootEntry, string) {
	entries := map[string]rootEntry{}
	maturity, freeze := "", ""
	for _, line := range block(rootHelp, "Commands:") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, "    ") {
			switch {
			case strings.HasPrefix(trimmed, "Core"):
				maturity = "core"
				freeze = strings.Join(specCodePattern.FindAllString(trimmed, -1), " ")
			case strings.HasPrefix(trimmed, "Experimental"):
				maturity = "experimental"
			default:
				maturity = ""
			}
			continue
		}
		names, label, found := strings.Cut(trimmed, "  ")
		if !found || maturity == "" {
			continue
		}
		entry := parseRootLabel(label, maturity)
		// A family line ("features | overview | review  gloss") labels each of its verbs.
		for _, family := range strings.Split(names, " | ") {
			command := strings.Fields(family)[0]
			if _, seen := entries[command]; !seen {
				entries[command] = entry
			}
		}
	}
	return entries, freeze
}

// parseRootLabel splits a root help gloss from its trailing owning spec in parentheses.
func parseRootLabel(label, maturity string) rootEntry {
	label = strings.TrimSpace(label)
	spec := ""
	if open := strings.LastIndex(label, " ("); open >= 0 && strings.HasSuffix(label, ")") {
		spec = label[open+2 : len(label)-1]
		label = label[:open]
	}
	return rootEntry{purpose: label, maturity: maturity, spec: spec}
}

func globalBlock(rootHelp string) string {
	return strings.Join(block(rootHelp, "Global options:"), "\n")
}

func nestedOperations(declared map[string][]string) map[string][]string {
	nested := map[string][]string{}
	for operation, names := range declared {
		sorted := slices.Clone(names)
		slices.Sort(sorted)
		nested[operation] = slices.Compact(sorted)
	}
	return nested
}

// parseSupportBoundary reads the root help Support boundary sentences that
// list read-only commands, local-state writers and self-observation ledger
// appenders. An item counts only when its first word is a command name, so an
// operation listed after its command ("cem begin, prepare") stays with it.
func parseSupportBoundary(rootHelp string) (readOnly, writers, ledger map[string]bool) {
	readOnly, writers, ledger = map[string]bool{}, map[string]bool{}, map[string]bool{}
	text := strings.Join(strings.Fields(strings.Join(block(rootHelp, "Support boundary:"), " ")), " ")
	for _, sentence := range strings.SplitAfter(text, ". ") {
		for _, clause := range strings.Split(sentence, ";") {
			switch {
			case strings.Contains(clause, " read without mutating"):
				addItems(readOnly, clause[:strings.Index(clause, " read without mutating")])
			case strings.Contains(clause, " best-effort append"):
				addItems(ledger, clause[:strings.Index(clause, " best-effort append")])
			case strings.Contains(clause, " write local"):
				for _, part := range strings.Split(clause, " write local") {
					addItems(writers, lastList(part))
				}
			}
		}
	}
	return readOnly, writers, ledger
}

// lastList drops a leading phrase that ended the previous list ("CEM artifacts")
// by keeping the text after the last "; " or the whole part.
func lastList(part string) string {
	part = strings.TrimSpace(part)
	for _, prefix := range []string{"CEM artifacts", "state"} {
		part = strings.TrimSpace(strings.TrimPrefix(part, prefix))
	}
	return part
}

func addItems(set map[string]bool, list string) {
	list = strings.ReplaceAll(list, ", and ", ", ")
	list = strings.ReplaceAll(list, " and ", ", ")
	for _, item := range strings.Split(list, ",") {
		if fields := strings.Fields(item); len(fields) > 0 {
			set[fields[0]] = true
		}
	}
}

func operations(name, text string, declared []string) []string {
	found := slices.Clone(declared)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "corvint" {
			continue
		}
		fields = fields[1:]
		if len(fields) >= 2 && fields[0] == "[--root" && fields[1] == "PATH]" {
			fields = fields[2:]
		}
		if len(fields) >= 2 && fields[0] == name && operationWord(fields[1]) {
			found = append(found, fields[1])
		}
	}
	slices.Sort(found)
	found = slices.Compact(found)
	if found == nil {
		found = []string{}
	}
	return found
}

func sortedTopics(topics []string) []string {
	sorted := slices.Clone(topics)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	if sorted == nil {
		sorted = []string{}
	}
	return sorted
}

// FormKey is the usage form a command line names: the command followed by the
// leading words of words that are its catalogued operation and nested
// operation, such as "cem verify" or "docs corpus build".
func FormKey(command Command, words []string) string {
	key := command.Name
	if len(words) > 0 && slices.Contains(command.Operations, words[0]) {
		key += " " + words[0]
		if nested, ok := command.NestedOperations[words[0]]; ok && len(words) > 1 && slices.Contains(nested, words[1]) {
			key += " " + words[1]
		}
	}
	return key
}

// requiredFlags derives, per usage form, the options every line of that form
// in the command's Usage blocks names outside brackets and parentheses.
func requiredFlags(command Command, text string) map[string][]string {
	forms := map[string][]string{}
	seen := map[string]bool{}
	for _, form := range usageForms(text) {
		fields := strings.Fields(form)[1:]
		if len(fields) >= 2 && fields[0] == "[--root" && fields[1] == "PATH]" {
			fields = fields[2:]
		}
		if len(fields) == 0 || fields[0] != command.Name {
			continue
		}
		key := FormKey(command, fields[1:])
		required := topLevelFlags(strings.Join(fields[1:], " "))
		if !seen[key] {
			seen[key], forms[key] = true, required
			continue
		}
		forms[key] = slices.DeleteFunc(forms[key], func(flag string) bool { return !slices.Contains(required, flag) })
	}
	for key, required := range forms {
		if len(required) == 0 {
			delete(forms, key)
		}
	}
	return forms
}

// usageForms returns each "corvint ..." line of every Usage: block, with its
// indented continuation lines joined.
func usageForms(text string) []string {
	var forms []string
	inUsage := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case line == "Usage:":
			inUsage = true
		case !inUsage:
		case trimmed == "":
			inUsage = false
		case strings.HasPrefix(trimmed, "corvint "):
			forms = append(forms, trimmed)
		case len(forms) > 0 && strings.HasPrefix(line, " "):
			forms[len(forms)-1] += " " + trimmed
		default:
			inUsage = false
		}
	}
	return forms
}

// topLevelFlags lists the --option tokens of a usage form outside every
// bracket and parenthesis, and before any "|" alternative at that level.
func topLevelFlags(form string) []string {
	var outside strings.Builder
	depth := 0
	for _, character := range form {
		switch {
		case character == '[' || character == '(':
			depth++
		case (character == ']' || character == ')') && depth > 0:
			depth--
		case depth == 0:
			outside.WriteRune(character)
		}
	}
	found := []string{}
	for _, token := range strings.Fields(outside.String()) {
		if token == "|" {
			return []string{}
		}
		flag, _, _ := strings.Cut(token, "=")
		if before, _, alternative := strings.Cut(flag, "|"); alternative {
			flag = before
		}
		if flagPattern.MatchString(flag) && flagPattern.FindString(flag) == flag && flag != "--root" && flag != "--help" {
			found = append(found, flag)
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

func operationWord(word string) bool {
	if word == "" || word[0] < 'a' || word[0] > 'z' {
		return false
	}
	for _, character := range word {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

// flags lists the --option tokens of text; a command's list leaves out the
// global --root and --help, which every command accepts.
func flags(text string, global bool) []string {
	found := []string{}
	for _, flag := range flagPattern.FindAllString(text, -1) {
		if global || (flag != "--root" && flag != "--help") {
			found = append(found, flag)
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// schemaIDs keeps a NAME/VERSION token only when it stands alone: not part of
// a path or a longer word on either side.
func schemaIDs(text string) []string {
	found := []string{}
	for _, span := range schemaPattern.FindAllStringIndex(text, -1) {
		before, after := byte(' '), byte(' ')
		if span[0] > 0 {
			before = text[span[0]-1]
		}
		if span[1] < len(text) {
			after = text[span[1]]
		}
		if strings.IndexByte(pathCharacters+".", before) >= 0 || strings.IndexByte(pathCharacters, after) >= 0 {
			continue
		}
		found = append(found, text[span[0]:span[1]])
	}
	slices.Sort(found)
	return slices.Compact(found)
}

const pathCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_/-"

func exitCodes(text string) []int {
	found := []int{}
	for _, match := range exitPattern.FindAllStringSubmatch(text, -1) {
		for _, digit := range append([]string{match[1]}, digitPattern.FindAllString(match[2], -1)...) {
			code, _ := strconv.Atoi(digit)
			found = append(found, code)
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

func writeStatements(text string) []string {
	found := []string{}
	for _, paragraph := range strings.Split(text, "\n\n") {
		joined := strings.Join(strings.Fields(paragraph), " ")
		for _, sentence := range splitSentences(joined) {
			if writePattern.MatchString(sentence) && !strings.HasPrefix(sentence, "corvint ") {
				found = append(found, sentence)
			}
		}
	}
	return slices.Compact(found)
}

func splitSentences(text string) []string {
	var sentences []string
	for text != "" {
		end := strings.Index(text, ". ")
		if end < 0 {
			sentences = append(sentences, text)
			break
		}
		sentences = append(sentences, text[:end+1])
		text = text[end+2:]
	}
	return sentences
}
