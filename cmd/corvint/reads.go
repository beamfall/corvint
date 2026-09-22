package main

import (
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/unplannedread"
)

// isReadsInvocation is the dispatch predicate: `reads` after any number of
// leading --root forms.
func isReadsInvocation(arguments []string) bool {
	_, index := readsRoot(arguments)
	return index < len(arguments) && arguments[index] == "reads"
}

func readsRoot(arguments []string) (string, int) {
	root, index := "", 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return root, index
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	return root, index
}

// parseReadsInvocation returns the resolved root, the render limit, and the
// mode: "digest", "enable" or "disable".
func parseReadsInvocation(arguments []string) (string, int, string, error) {
	root, index := readsRoot(arguments)
	if root == "" {
		resolved, err := normalizeRoot(".")
		if err != nil {
			return "", 0, "", err
		}
		root = resolved
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", 0, "", err
		}
		root = resolved
	}
	limit, mode, limitToken := 120, "digest", ""
	for index++; index < len(arguments); index++ {
		if arguments[index] == "enable" || arguments[index] == "disable" {
			if mode != "digest" {
				return "", 0, "", argumentError("unrecognized arguments: " + arguments[index])
			}
			if limitToken != "" {
				return "", 0, "", argumentError("unrecognized arguments: " + limitToken)
			}
			mode = arguments[index]
			continue
		}
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--limit" || mode != "digest" {
			return "", 0, "", argumentError("unrecognized arguments: " + arguments[index])
		}
		limitToken = arguments[index]
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return "", 0, "", argumentError("argument --limit: expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 120 {
			return "", 0, "", argumentError("invalid --limit")
		}
		limit = parsed
	}
	return root, limit, mode, nil
}

// runReads prints the unplanned-read digest, or toggles the opt-in marker.
// The digest path is read-only; enable/disable are explicit mutations.
func runReads(arguments []string, stdout, stderr io.Writer) int {
	root, limit, mode, err := parseReadsInvocation(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	switch mode {
	case "enable":
		err = unplannedread.Enable(root)
	case "disable":
		err = unplannedread.Disable(root)
	default:
		err = unplannedread.Render(root, limit, stdout)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

const readsHelp = `
Unplanned reads (experimental, URE-V0 proposed), JSON:

  corvint [--root PATH] reads [--limit N]
  corvint [--root PATH] reads enable
  corvint [--root PATH] reads disable

The digest reports how many tool reads opened a project file the delivered
packet did not already carry. The ledger is private local derived state and is
never an input to ranking, learning, evidence, or authority.

enable and disable are the only mutations in this verb: they create and remove
the opt-in marker .corvint/unplanned-reads.enabled. Without that marker nothing
is recorded and no ledger file is created. The digest itself never writes.
`
