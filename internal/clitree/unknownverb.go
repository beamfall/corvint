package clitree

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// UnknownVerbCode is the stable code of a companion binary's unknown-verb refusal (ACX-V0-904).
const UnknownVerbCode = "unknown-verb"

// UnknownVerb returns the first argument when it is an operation word that names no top-level
// node and no internal entry point of tree. An option, a value that is not an operation word or
// an empty argument list is not an unknown verb.
func UnknownVerb(tree Tree, args []string) (string, bool) {
	if len(args) == 0 || !OperationWord(args[0]) {
		return "", false
	}
	for _, node := range tree.Nodes {
		if len(node.Path) > 0 && node.Path[0] == args[0] {
			return "", false
		}
		// Single-mode tools can take a positional input (for example GOROOT).
		// A word in that slot belongs to the native parser, not verb discovery.
		if tree.Source == "companion root contract" && len(node.Path) == 0 {
			for _, input := range node.Required {
				if !strings.HasPrefix(input, "-") && !strings.HasPrefix(input, "stdin:") {
					return "", false
				}
			}
		}
	}
	for _, entry := range tree.Internal {
		if entry.Word == args[0] {
			return "", false
		}
	}
	return args[0], true
}

// RefuseUnknownVerb writes the one shared unknown-verb envelope as a single JSON line on stderr
// and returns exit 2 (ACX-V0-904): {"binary", "code": "unknown-verb", "error", "help", "ok": false}.
func RefuseUnknownVerb(stderr io.Writer, binary, verb string) int {
	word, hint, _ := strings.Cut(verb, "\n")
	message := fmt.Sprintf("unknown command %q", word)
	if hint != "" {
		message += "; " + hint
	}
	line, err := json.Marshal(map[string]any{
		"binary": binary,
		"code":   UnknownVerbCode,
		"error":  message,
		"help":   binary + " --help",
		"ok":     false,
	})
	if err == nil {
		_, _ = fmt.Fprintf(stderr, "%s\n", line)
	}
	return 2
}

// RefuseIfUnknownVerb refuses args with RefuseUnknownVerb when UnknownVerb finds an unknown verb.
func RefuseIfUnknownVerb(tree Tree, args []string, stderr io.Writer) (exit int, refused bool) {
	verb, unknown := UnknownVerb(tree, args)
	if !unknown {
		return 0, false
	}
	return RefuseUnknownVerb(stderr, tree.Binary, Suggestion(tree, verb)), true
}

// Suggestion uses the actual command tree, with a bounded edit distance and one
// deterministic result. The newline is an internal separator, never user text.
func Suggestion(tree Tree, word string) string {
	if len(word) > 64 {
		return word
	}
	best, choice := 3, ""
	var names []string
	for _, node := range tree.Nodes {
		if len(node.Path) > 0 {
			names = append(names, node.Path[0])
		}
	}
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		d := distance(word, name)
		if d < best {
			best, choice = d, name
		}
	}
	if choice != "" {
		return word + "\n" + "did you mean `" + tree.Binary + " " + choice + "`?"
	}
	return word
}
func distance(a, b string) int {
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(a); i++ {
		next := make([]int, len(b)+1)
		next[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			next[j] = min(row[j]+1, next[j-1]+1, row[j-1]+cost)
		}
		row = next
	}
	return row[len(b)]
}
