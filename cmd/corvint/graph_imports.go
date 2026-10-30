package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const graphImportsHelp = `usage: corvint [--root DIR] graph imports --revision FULL_COMMIT [--kinds KINDS] [--format jsonl]

Export every resolved indexed file import as deterministic graph-imports/1 JSONL.
Required: --revision is a full existing commit ID (40 or 64 lowercase hex digits).
KINDS is a comma-separated subset of import,type-import,angular-template,
angular-style,ruby-constant; default is all. --kinds removes only edges.
Records include revision, from, to, kind and an immutable evidence site. Scope and
uncertainty records are always emitted, including with --kinds. No result cap.
Go imports expand to files in the imported package; Python and JS/TS use impact's
resolution rules. Literal Angular templateUrl/styleUrl(s) resolve relative to the
component; a Ruby constant is an edge (authority convention) only when Zeitwerk
maps it to exactly one file. Non-literal, missing, zero- or multi-candidate
references, selectors and string-keyed DI stay uncertainty records.
This is syntax and convention evidence, not runtime or ownership closure.
Read-only: a historical commit works with a dirty worktree and does not move HEAD.
Refusals: graph-invalid-revision, graph-unknown-revision, graph-invalid-kinds,
graph-invalid-arguments. Only JSONL is supported. Check the exit status after reading.
MCP: corvint.graph.imports in the import-graph profile returns JSONL pages; follow
nextOffset with the same revision and kinds until done is true.

Example: corvint graph imports --revision 0123456789012345678901234567890123456789 --format jsonl
`

func parseGraphInvocation(arguments []string) (string, []string, bool, error) {
	root, pos := "", 0
	for pos < len(arguments) {
		if arguments[pos] == "--root" && rootPreambleValue(arguments, pos+1) {
			root = arguments[pos+1]
			pos += 2
		} else if strings.HasPrefix(arguments[pos], "--root=") {
			root = strings.TrimPrefix(arguments[pos], "--root=")
			pos++
		} else {
			break
		}
	}
	if pos >= len(arguments) || arguments[pos] != "graph" {
		return "", nil, false, nil
	}
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = normalizeRoot(root)
	}
	return root, arguments[pos+1:], true, err
}

func runGraphImports(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	refuse := func(err error) int { emitError(diagnostic, err); return 2 }
	invalid := func() int {
		return refuse(&contextindex.Error{Code: "graph-invalid-arguments", Message: "expected graph imports --revision FULL_COMMIT [--kinds KINDS] [--format jsonl]"})
	}
	if len(args) == 0 || args[0] != "imports" {
		return invalid()
	}
	values := map[string]string{}
	for i := 1; i < len(args); i++ {
		key, value, equal := strings.Cut(args[i], "=")
		if key != "--revision" && key != "--kinds" && key != "--format" {
			return invalid()
		}
		if _, seen := values[key]; seen {
			return invalid()
		}
		if !equal {
			i++
			if i >= len(args) {
				return invalid()
			}
			value = args[i]
		}
		if value == "" || strings.HasPrefix(value, "--") {
			return invalid()
		}
		values[key] = value
	}
	if format, present := values["--format"]; present && format != "jsonl" {
		return invalid()
	}
	kinds, err := contextindex.ImportGraphKinds(values["--kinds"])
	if err != nil {
		return refuse(err)
	}
	index, err := contextindex.BuildImportGraph(ctx, root, values["--revision"])
	if err != nil {
		return refuse(err)
	}
	encoder := json.NewEncoder(out)
	if err := contextindex.WalkImportGraph(ctx, index, kinds, func(row contextindex.ImportGraphRecord) error { return encoder.Encode(row) }); err != nil {
		return refuse(err)
	}
	return 0
}
