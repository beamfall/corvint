package main

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// contextLookupOptions is one parsed `context defs|refs|grep` invocation
// (TCP-V0-017, proposed): the structural query surface of the context verb.
type contextLookupOptions struct {
	root, mode string
	terms      []string
	limit      int
}

var contextLookupModes = map[string]bool{"defs": true, "refs": true, "grep": true, "authority": true}

// parseContextLookupInvocation recognises `[--root PATH] context (defs|refs)
// IDENTIFIER [--limit N]` and `[--root PATH] context grep TERM... [--limit N]`;
// any other shape, including `context --task`, is not a lookup invocation.
func parseContextLookupInvocation(arguments []string) (contextLookupOptions, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return contextLookupOptions{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "context" || position+1 >= len(arguments) || !contextLookupModes[arguments[position+1]] {
		return contextLookupOptions{}, false, nil
	}
	options := contextLookupOptions{root: ".", mode: arguments[position+1], limit: contextindex.LookupDefaultLimit}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	rest := arguments[position+2:]
	for index := 0; index < len(rest); index++ {
		if !strings.HasPrefix(rest[index], "--") {
			options.terms = append(options.terms, rest[index])
			continue
		}
		flag, value, inline := strings.Cut(rest[index], "=")
		if flag != "--limit" {
			return options, true, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, true, argumentError("argument --limit: expected one argument")
			}
			value, index = rest[index+1], index+1
		}
		limit, err := strconv.Atoi(value)
		if err != nil {
			return options, true, argumentError("argument --limit: invalid int value: " + strconv.Quote(value))
		}
		options.limit = limit
	}
	if options.mode != "grep" && options.mode != "authority" && len(options.terms) > 1 {
		return options, true, argumentError("argument " + options.mode + ": expected one identifier")
	}
	resolved, err := resolveExplicitRoot(options.root)
	if err != nil {
		return options, true, err
	}
	options.root = resolved
	return options, true, nil
}

// runContextLookup answers one lookup from the tree's snapshot when
// `corvint index` wrote one, else from one index build over the committed
// tree, exactly as `context --task` does; nothing is written on any path.
func runContextLookup(ctx context.Context, options contextLookupOptions, stdout, stderr io.Writer) int {
	index, hit, err := loadSnapshot(ctx, options.root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if !hit {
		index, err = contextindex.BuildContext(ctx, options.root, "")
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := contextLookup(index, options)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(result)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write context lookup result"})
		return 2
	}
	return 0
}

func contextLookup(index *contextindex.Index, options contextLookupOptions) (map[string]any, error) {
	identifier := ""
	if len(options.terms) == 1 {
		identifier = options.terms[0]
	}
	switch options.mode {
	case "defs":
		return contextindex.LookupDefinitions(index, identifier, options.limit)
	case "refs":
		return contextindex.LookupReferences(index, identifier, options.limit)
	case "authority":
		return contextindex.LookupAuthorityTriggers(index, options.terms, options.limit)
	}
	return contextindex.LookupGrep(index, options.terms, options.limit)
}

const contextLookupHelp = `
Structural lookups (experimental, TCP-V0-017 proposed), read-only, JSON:

  corvint [--root PATH] context defs IDENTIFIER [--limit N]
  corvint [--root PATH] context refs IDENTIFIER [--limit N]
  corvint [--root PATH] context grep TERM... [--limit N]

  defs   the symbols defining IDENTIFIER: exact-name matches first, then
         case-insensitive ones, rarest name first; each with kind, path and
         declaration line (end line when the extractor reports one)
  refs   the files naming IDENTIFIER as a whole word or importing a file that
         defines it, with whole-word counts; the defining files are excluded
  grep   the files holding the whole tokens of TERM..., ranked by BM25 over
         body and path tokens, each with up to five matching lines; substring
         and regular-expression matching are not offered here

--limit defaults to 20. An empty identifier or term, or one over 256 bytes,
is refused with unsupported-context-lookup-identifier. Excluded paths never
appear: every mode reads the tables the packet reads.
`
