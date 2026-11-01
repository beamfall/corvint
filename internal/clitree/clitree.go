// Package clitree is the machine-readable command tree of a Corvint binary
// (corvint-command-tree/0): every subcommand node at every depth with its usage
// synopses, flags, required inputs and one example. A binary derives its tree
// from the help synopses it already prints and the operation tables its
// dispatch reads, so a node cannot exist without help, and the drift check in
// tools/capabilities-check compares the tree with the literals the dispatch
// code matches (CAP-V0-040 to CAP-V0-049, docs/specs/capability-catalog-v0.md).
package clitree

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// Schema names the tree document.
const Schema = "corvint-command-tree/0"

// Node is one subcommand at any depth. Path holds the words after the binary.
type Node struct {
	Path     []string `json:"path"`
	Command  string   `json:"command"`
	Summary  string   `json:"summary"`
	Usage    []string `json:"usage"`
	Flags    []string `json:"flags"`
	Required []string `json:"required"`
	Example  string   `json:"example"`
	Help     string   `json:"help"`
}

// Internal is a dispatched word that is an internal entry point (a worker the
// binary starts itself, a hook a host calls), listed with the reason it has no
// public help node.
type Internal struct {
	Word   string `json:"word"`
	Reason string `json:"reason"`
}

// Tree is one binary's command tree.
type Tree struct {
	Schema   string     `json:"schema"`
	Binary   string     `json:"binary"`
	HelpJSON string     `json:"helpJson"`
	Source   string     `json:"source"`
	Sources  []string   `json:"dispatchSources"`
	Global   []string   `json:"globalFlags"`
	Internal []Internal `json:"internalEntryPoints"`
	Nodes    []Node     `json:"nodes"`
}

// AllFlags is the union of the global flags, every node's flags and every
// listed internal option.
func (t Tree) AllFlags() map[string]bool {
	all := map[string]bool{"--help": true, "-h": true}
	for _, flag := range t.Global {
		all[flag] = true
	}
	for _, internal := range t.Internal {
		if strings.HasPrefix(internal.Word, "-") {
			all[internal.Word] = true
		}
	}
	for _, node := range t.Nodes {
		for _, flag := range node.Flags {
			all[flag] = true
		}
	}
	return all
}

var flagToken = regexp.MustCompile(`^-{1,2}[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*`)

// OperationWord reports whether a usage token is a literal subcommand word:
// lowercase letters, digits and hyphens, starting with a letter.
func OperationWord(word string) bool {
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

// Builder accumulates usage forms into nodes.
type Builder struct {
	binary   string
	forms    map[string][]string
	summary  map[string]string
	example  map[string]string
	declared map[string]bool
}

// NewBuilder starts a tree for binary.
func NewBuilder(binary string) *Builder {
	return &Builder{binary: binary, forms: map[string][]string{}, summary: map[string]string{},
		example: map[string]string{}, declared: map[string]bool{}}
}

// Declare adds a node for path even when no usage form names it yet; Problems
// then reports it until its help gains a synopsis.
func (b *Builder) Declare(path []string, summary string) {
	key := strings.Join(path, " ")
	b.declared[key] = true
	if summary != "" && b.summary[key] == "" {
		b.summary[key] = summary
	}
}

// Example sets the example of the node at path instead of the derived one.
func (b *Builder) Example(path []string, example string) {
	b.example[strings.Join(path, " ")] = example
}

// AddText adds every synopsis line of text: a line whose first token is the
// binary, with indented continuation lines joined. A line starting with
// "usage:" is read the same way.
func (b *Builder) AddText(text, summary string) {
	for _, form := range Synopses(b.binary, text) {
		b.AddForm(form, summary)
	}
}

// AddForm adds one synopsis line. A token whose alternatives are all operation
// words (`a|b` or `<a|b>`) at a path position expands into one form per word.
func (b *Builder) AddForm(form, summary string) {
	for _, expanded := range expand(b.binary, form) {
		path := FormPath(b.binary, expanded)
		if len(path) == 0 {
			continue
		}
		key := strings.Join(path, " ")
		if !slices.Contains(b.forms[key], expanded) {
			b.forms[key] = append(b.forms[key], expanded)
		}
		if summary != "" && b.summary[key] == "" {
			b.summary[key] = summary
		}
		for depth := 1; depth < len(path); depth++ {
			b.declared[strings.Join(path[:depth], " ")] = true
		}
	}
}

// Synopses returns each synopsis line of text that starts with the binary
// (after an optional "usage:" or "Usage:" label), joined with its indented
// continuation lines.
func Synopses(binary, text string) []string {
	var forms []string
	open := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		lowered := strings.ToLower(trimmed)
		if rest, ok := strings.CutPrefix(lowered, "usage:"); ok {
			trimmed = strings.TrimSpace(trimmed[len(trimmed)-len(rest):])
			if trimmed == "" {
				open = false
				continue
			}
		}
		if label, rest, ok := strings.Cut(trimmed, ": usage: "); ok && strings.HasPrefix(label, binary) {
			trimmed = rest
		}
		switch {
		case strings.HasPrefix(trimmed, binary+" "):
			forms = append(forms, trimmed)
			open = true
		case open && trimmed != "" && strings.HasPrefix(line, "    ") && !strings.Contains(trimmed, ": "):
			forms[len(forms)-1] += " " + trimmed
		default:
			open = false
		}
	}
	return forms
}

// tokens splits a form, keeping a bracket or angle group with spaces as one
// token only where it is a single word.
func tokens(form string) []string { return strings.Fields(form) }

// skipPreamble returns the index of the first token after the binary and any
// leading bracketed option group such as [--root PATH].
func skipPreamble(fields []string) int {
	index := 1
	for index < len(fields) && strings.HasPrefix(fields[index], "[-") {
		depth := 0
		for index < len(fields) {
			depth += strings.Count(fields[index], "[") - strings.Count(fields[index], "]")
			index++
			if depth <= 0 {
				break
			}
		}
	}
	return index
}

// FormPath returns the leading operation words of a synopsis after the binary
// and its preamble.
func FormPath(binary, form string) []string {
	fields := tokens(form)
	if len(fields) == 0 || fields[0] != binary {
		return nil
	}
	var path []string
	for _, field := range fields[skipPreamble(fields):] {
		if !OperationWord(field) {
			break
		}
		path = append(path, field)
	}
	return path
}

func alternatives(token string) ([]string, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">")
	if strings.HasPrefix(token, "<") != strings.HasSuffix(token, ">") || !strings.Contains(inner, "|") {
		return nil, false
	}
	words := strings.Split(inner, "|")
	for _, word := range words {
		if !OperationWord(word) {
			return nil, false
		}
	}
	return words, true
}

// spacedAlternatives is a parenthesized choice of operation words written with
// spaces, such as `(init | adopt)`.
var spacedAlternatives = regexp.MustCompile(`\(\s*([a-z][a-z0-9-]*(?:\s*\|\s*[a-z][a-z0-9-]*)+)\s*\)`)

func expand(binary, form string) []string {
	form = spacedAlternatives.ReplaceAllStringFunc(form, func(group string) string {
		return "<" + strings.Join(strings.Fields(strings.ReplaceAll(strings.Trim(group, "()"), "|", " ")), "|") + ">"
	})
	fields := tokens(form)
	if len(fields) == 0 || fields[0] != binary {
		return nil
	}
	for index := skipPreamble(fields); index < len(fields); index++ {
		if OperationWord(fields[index]) {
			continue
		}
		words, ok := alternatives(fields[index])
		if !ok {
			break
		}
		var out []string
		for _, word := range words {
			copied := slices.Clone(fields)
			copied[index] = word
			out = append(out, expand(binary, strings.Join(copied, " "))...)
		}
		return out
	}
	return []string{form}
}

// Tree assembles the nodes in path order. A node without its own synopsis
// carries the synopses of its descendants.
func (b *Builder) Tree(helpJSON, source string, sources, global []string, internal []Internal) Tree {
	keys := map[string]bool{}
	for key := range b.forms {
		keys[key] = true
	}
	for key := range b.declared {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	slices.Sort(ordered)
	tree := Tree{Schema: Schema, Binary: b.binary, HelpJSON: helpJSON, Source: source,
		Sources: slices.Clone(sources), Internal: slices.Clone(internal), Nodes: []Node{}}
	if tree.Sources == nil {
		tree.Sources = []string{}
	}
	if tree.Internal == nil {
		tree.Internal = []Internal{}
	}
	tree.Global = slices.Sorted(slices.Values(global))
	if tree.Global == nil {
		tree.Global = []string{}
	}
	for _, key := range ordered {
		path := strings.Fields(key)
		own := b.forms[key]
		usage := slices.Clone(own)
		if len(usage) == 0 {
			for _, other := range ordered {
				if strings.HasPrefix(other, key+" ") {
					usage = append(usage, b.forms[other]...)
				}
			}
		}
		node := Node{Path: path, Command: b.binary + " " + key, Summary: b.summary[key], Usage: usage,
			Flags: Flags(usage), Required: required(b.binary, own, len(path)), Example: b.example[key],
			Help: b.binary + " " + key + " --help"}
		if node.Usage == nil {
			node.Usage = []string{}
		}
		if node.Example == "" && len(usage) > 0 {
			node.Example = MinimalExample(usage[0])
		}
		tree.Nodes = append(tree.Nodes, node)
	}
	return tree
}

// Flags lists every option token the forms name, without --help and -h.
func Flags(forms []string) []string {
	found := []string{}
	for _, form := range forms {
		for _, field := range strings.Fields(form) {
			field = strings.TrimLeft(field, "[(<{|")
			flag := flagToken.FindString(field)
			if flag == "" || flag == "--help" || flag == "-h" || !strings.HasPrefix(field, flag) {
				continue
			}
			found = append(found, flag)
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// outside returns the tokens of form outside every bracket, parenthesis and
// brace group, after the node path.
func outside(binary, form string, depth int) []string {
	fields := tokens(form)
	start := skipPreamble(fields) + depth
	if start > len(fields) {
		return nil
	}
	var kept []string
	level := 0
	for _, field := range fields[start:] {
		opened := strings.Count(field, "[") + strings.Count(field, "(") + strings.Count(field, "{")
		closed := strings.Count(field, "]") + strings.Count(field, ")") + strings.Count(field, "}")
		if level == 0 && opened == 0 {
			kept = append(kept, field)
		}
		level += opened - closed
		if level < 0 {
			level = 0
		}
	}
	return kept
}

// required lists the inputs every own form names outside optional groups: an
// option with its value placeholder, or a positional placeholder.
func required(binary string, forms []string, depth int) []string {
	var common []string
	for index, form := range forms {
		var found []string
		fields := outside(binary, form, depth)
		for position := 0; position < len(fields); position++ {
			field := fields[position]
			switch {
			case field == "|" || field == "..." || field == "<" || field == ">":
				continue
			case strings.HasPrefix(field, "-"):
				if position+1 < len(fields) && !strings.HasPrefix(fields[position+1], "-") && fields[position+1] != "|" && !OperationWord(fields[position+1]) {
					field += " " + fields[position+1]
					position++
				}
				found = append(found, field)
			case !OperationWord(field):
				found = append(found, field)
			}
		}
		if index == 0 {
			common = found
			continue
		}
		common = slices.DeleteFunc(common, func(item string) bool { return !slices.Contains(found, item) })
	}
	if common == nil {
		return []string{}
	}
	return common
}

// MinimalExample is the smallest invocation a synopsis allows: optional
// [ ] groups and prose ( ) groups are dropped, a required ( a | b ) group
// keeps its first alternative, and an a|b token keeps a.
func MinimalExample(form string) string {
	fields := tokens(form)
	var kept []string
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		switch {
		case strings.HasPrefix(field, "["):
			index = closeGroup(fields, index, '[', ']')
		case strings.HasPrefix(field, "("):
			end := closeGroup(fields, index, '(', ')')
			group := strings.Join(fields[index:end+1], " ")
			group = strings.TrimSuffix(strings.TrimPrefix(group, "("), ")")
			first, _, _ := strings.Cut(group, "|")
			if first = strings.TrimSpace(first); strings.HasPrefix(first, "-") {
				kept = append(kept, strings.Fields(first)...)
			}
			index = end
		case field == "..." || strings.HasPrefix(field, "]..."):
		default:
			if before, _, ok := strings.Cut(field, "|"); ok && !strings.HasPrefix(field, "<") && before != "" {
				field = before
			}
			kept = append(kept, field)
		}
	}
	return strings.Join(kept, " ")
}

func closeGroup(fields []string, start int, open, closing byte) int {
	depth := 0
	for index := start; index < len(fields); index++ {
		depth += strings.Count(fields[index], string(open)) - strings.Count(fields[index], string(closing))
		if depth <= 0 {
			return index
		}
	}
	return len(fields) - 1
}

// Find returns the deepest node whose path is a prefix of words.
func (t Tree) Find(words []string) (Node, bool) {
	best, found := Node{}, false
	for _, node := range t.Nodes {
		if len(node.Path) <= len(words) && slices.Equal(node.Path, words[:len(node.Path)]) && len(node.Path) > len(best.Path) {
			best, found = node, true
		}
	}
	return best, found
}

// LeadingWords returns the operation words of args before the first option,
// after skipping preamble options such as --root PATH.
func LeadingWords(args []string, preamble map[string]bool) []string {
	index := 0
	for index < len(args) && preamble[strings.SplitN(args[index], "=", 2)[0]] {
		if strings.Contains(args[index], "=") {
			index++
		} else {
			index += 2
		}
	}
	var words []string
	for ; index < len(args) && OperationWord(args[index]); index++ {
		words = append(words, args[index])
	}
	return words
}

// Render is a node's own help: its summary, synopses, flags, required inputs
// and example, and where the machine-readable tree is.
func Render(tree Tree, node Node) string {
	var text strings.Builder
	text.WriteString(node.Command)
	if node.Summary != "" {
		text.WriteString(": " + node.Summary)
	}
	text.WriteString("\n\nUsage:\n")
	for _, form := range node.Usage {
		text.WriteString("  " + form + "\n")
	}
	text.WriteString("\nFlags: " + listOrNone(node.Flags) + "\n")
	text.WriteString("Required: " + listOrNone(node.Required) + "\n")
	text.WriteString("\nExample:\n  " + node.Example + "\n")
	if tree.HelpJSON != "" {
		text.WriteString("\nEvery command of " + tree.Binary + ": " + tree.HelpJSON + "\n")
	}
	return text.String()
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// Problems lists each node that lacks a synopsis, an example or its help text.
func Problems(tree Tree) []string {
	var problems []string
	for _, node := range tree.Nodes {
		if len(node.Usage) == 0 {
			problems = append(problems, node.Command+": no usage synopsis")
		}
		if strings.TrimSpace(node.Example) == "" {
			problems = append(problems, node.Command+": no example")
		}
		if strings.TrimSpace(node.Help) == "" {
			problems = append(problems, node.Command+": no help")
		}
	}
	for _, entry := range tree.Internal {
		if strings.TrimSpace(entry.Word) == "" || strings.TrimSpace(entry.Reason) == "" {
			problems = append(problems, fmt.Sprintf("internal entry %q: a word and its reason are both required", entry.Word))
		}
	}
	return problems
}

// Encode is the canonical document: two-space indented, HTML characters not
// escaped, one trailing newline.
func Encode(tree Tree) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tree); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Words returns every path word and every token of every synopsis, the
// vocabulary a dispatched literal must belong to (CAP-V0-043).
func (t Tree) Words() map[string]bool {
	words := map[string]bool{}
	for _, node := range t.Nodes {
		for _, word := range node.Path {
			words[word] = true
		}
		for _, form := range node.Usage {
			for _, piece := range strings.FieldsFunc(form, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.')
			}) {
				words[piece] = true
			}
		}
	}
	for _, internal := range t.Internal {
		words[internal.Word] = true
	}
	return words
}

// Answer serves a binary's help from its tree before any flag parsing (CAP-V0-931): `help --json`
// prints the encoded tree; `help`, a lone --help or -h prints root; `help PATH...` and an
// invocation whose leading words name a node and whose arguments carry --help or -h before `--`
// print that node's Render. It reports false for every other invocation, which the binary's own
// dispatch then handles unchanged.
func Answer(tree Tree, args []string, root string, stdout io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	isHelp := func(arg string) bool { return arg == "--help" || arg == "-h" }
	var words []string
	switch {
	case len(args) == 2 && args[0] == "help" && args[1] == "--json":
		encoded, err := Encode(tree)
		if err != nil {
			return true, err
		}
		_, err = stdout.Write(encoded)
		return true, err
	case args[0] == "help":
		words = slices.DeleteFunc(slices.Clone(args[1:]), isHelp)
	case len(args) == 1 && isHelp(args[0]):
	default:
		asked := false
		for _, arg := range args {
			if arg == "--" {
				break
			}
			asked = asked || isHelp(arg)
		}
		if !asked {
			return false, nil
		}
		words = LeadingWords(args, nil)
		if len(words) == 0 {
			return false, nil
		}
	}
	if len(words) == 0 {
		_, err := io.WriteString(stdout, root)
		return true, err
	}
	node, ok := tree.Find(words)
	if !ok || (args[0] == "help" && len(node.Path) != len(words)) {
		return true, fmt.Errorf("unknown command %q; run `%s help`", strings.Join(words, " "), tree.Binary)
	}
	_, err := io.WriteString(stdout, Render(tree, node))
	return true, err
}
