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
	Schema          string            `json:"schema"`
	Source          string            `json:"source"`
	Derivation      map[string]string `json:"derivation"`
	GlobalFlags     []string          `json:"globalFlags"`
	GlobalSchemaIDs []string          `json:"globalSchemaIds"`
	Guide           GuideRef          `json:"guide"`
	Commands        []Command         `json:"commands"`
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
	readOnly, writers, ledger := parseSupportBoundary(registry.RootHelp)
	catalog := Catalog{
		Schema:          Schema,
		Source:          "corvint help: the topLevelCommands dispatch list, the root help and each COMMAND --help topic",
		Derivation:      derivation,
		GlobalFlags:     flags(globalBlock(registry.RootHelp), true),
		GlobalSchemaIDs: schemaIDs(globalBlock(registry.RootHelp)),
		Guide:           GuideRef{Topic: "corvint help guide", SHA256: digest(guide)},
		Commands:        make([]Command, 0, len(registry.Commands)),
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
		label = strings.TrimSpace(label)
		spec := ""
		if open := strings.LastIndex(label, " ("); open >= 0 && strings.HasSuffix(label, ")") {
			spec = label[open+2 : len(label)-1]
			label = label[:open]
		}
		command := strings.Fields(names)[0]
		if _, seen := entries[command]; !seen {
			entries[command] = rootEntry{purpose: label, maturity: maturity, spec: spec}
		}
	}
	return entries, freeze
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
