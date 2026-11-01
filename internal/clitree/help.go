package clitree

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
)

// CompanionTree is a companion binary's tree derived from its synopsis text (CAP-V0-972): every
// synopsis line of usage, a summary per node path and an optional example per node path. Its
// dispatch source is the binary's own cmd/ directory.
func CompanionTree(binary, usage string, summaries, examples map[string]string, global []string, internal []Internal) Tree {
	builder := NewBuilder(binary)
	builder.AddText(usage, "")
	for path, summary := range summaries {
		builder.Declare(strings.Fields(path), summary)
	}
	for path, example := range examples {
		builder.Example(strings.Fields(path), example)
	}
	return builder.Tree(binary+" help --json", "cmd/"+binary+" usageText", []string{"cmd/" + binary}, global, internal)
}

// Help serves a companion binary's help from its tree (CAP-V0-973).
type Help struct {
	Tree Tree
	// Title opens the generated root help; it is unused when Intro is set.
	Title string
	// RootForms are synopses of a mode with no subcommand word, which no node holds; the generated
	// root help lists them first.
	RootForms []string
	// Intro writes the binary's authored root usage, if it has one.
	Intro func(io.Writer)
	// Detail writes the binary's authored help for a node after its Render, if it has one.
	Detail func(path []string, w io.Writer)
}

// Root is the root help: the authored usage, or a title and every synopsis, then every node with
// its summary and the help --json pointer.
func (h Help) Root() string {
	var text strings.Builder
	if h.Intro != nil {
		h.Intro(&text)
		if !strings.HasSuffix(text.String(), "\n") {
			text.WriteString("\n")
		}
	} else {
		text.WriteString(h.Tree.Binary + ": " + h.Title + "\n\nUsage:\n")
		for _, form := range h.RootForms {
			text.WriteString("  " + form + "\n")
		}
		for _, node := range h.Tree.Nodes {
			for _, form := range node.Usage {
				text.WriteString("  " + form + "\n")
			}
		}
	}
	text.WriteString("\nCommands:\n")
	for _, node := range h.Tree.Nodes {
		text.WriteString("  " + strings.Join(node.Path, " ") + ": " + node.Summary + "\n")
	}
	text.WriteString("\nRun `" + h.Tree.Binary + " COMMAND --help` for one command's usage, flags, required inputs and example.\n")
	text.WriteString("Every command of " + h.Tree.Binary + ": " + h.Tree.HelpJSON + "\n")
	return text.String()
}

// Serve answers the invocations Answer answers, with exit 0, or exit 2 and one stderr line for an
// unknown help path. A node's help is its Render followed by its authored Detail. handled is false
// for every other invocation, which the binary's own dispatch then handles unchanged.
func (h Help) Serve(args []string, stdout, stderr io.Writer) (exit int, handled bool) {
	var answer bytes.Buffer
	handled, err := Answer(h.Tree, args, h.Root(), &answer)
	if !handled {
		return 0, false
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2, true
	}
	if _, err := stdout.Write(answer.Bytes()); err != nil {
		return 1, true
	}
	if h.Detail != nil {
		for _, node := range h.Tree.Nodes {
			if answer.String() == Render(h.Tree, node) {
				h.Detail(slices.Clone(node.Path), stdout)
				break
			}
		}
	}
	return 0, true
}
