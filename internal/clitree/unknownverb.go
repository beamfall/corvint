package clitree

import (
	"encoding/json"
	"fmt"
	"io"
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
	line, err := json.Marshal(map[string]any{
		"binary": binary,
		"code":   UnknownVerbCode,
		"error":  fmt.Sprintf("unknown command %q", verb),
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
	return RefuseUnknownVerb(stderr, tree.Binary, verb), true
}
