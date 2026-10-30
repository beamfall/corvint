package main

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/depsource"
)

// parseDepsourceInvocation recognizes `[--root PATH] depsource <module>[@version]
// [--file REL] [--limit N]` and mirrors observations.go's argument conventions.
func parseDepsourceInvocation(arguments []string) (depsource.Options, bool, error) {
	options := depsource.Options{Limit: depsource.DefaultLimit}
	root := ""
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return options, false, nil
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "depsource" {
		return options, false, nil
	}
	resolved, err := resolveDepsourceRoot(root)
	if err != nil {
		return options, true, err
	}
	options.Root = resolved
	index++
	if index >= len(arguments) || strings.HasPrefix(arguments[index], "-") {
		return options, true, argumentError("argument module: expected one module path")
	}
	options.Module, options.Version = splitModuleVersion(arguments[index])
	if options.Module == "" {
		return options, true, argumentError("argument module: expected one module path")
	}
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--file" && name != "--limit" {
			return options, true, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, true, argumentError("argument " + name + ": expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		if name == "--file" {
			if value == "" {
				return options, true, argumentError("invalid --file")
			}
			options.File = value
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 2000 {
			return options, true, argumentError("invalid --limit")
		}
		options.Limit = parsed
	}
	return options, true, nil
}

func resolveDepsourceRoot(root string) (string, error) {
	if root == "" {
		return normalizeRoot(".")
	}
	return resolveExplicitRoot(root)
}

// splitModuleVersion splits `example.com/mod@v1.2.3`. A version is optional; a
// module path never contains '@', so the last '@' is the separator.
func splitModuleVersion(value string) (string, string) {
	index := strings.LastIndex(value, "@")
	if index <= 0 {
		return value, ""
	}
	return value[:index], value[index+1:]
}

// runDepsource emits pinned third-party module evidence. It is read-only: an
// unprovable pin is an abstention on stdout with exit status 0, and only an
// argument or repository failure is exit status 2. ctx is main's signal
// context, so SIGINT and SIGTERM cancel the hash verification like every
// other packet verb.
func runDepsource(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	options, isDepsource, err := parseDepsourceInvocation(arguments)
	if !isDepsource {
		return 2
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := depsource.Render(ctx, options, stdout); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

const depsourceHelp = `
Pinned dependency source (experimental, DSE-V0 proposed), read-only, JSON:

  corvint [--root PATH] depsource MODULE [--file PATH] [--limit N]

Reads the committed go.mod and go.sum, recomputes the module cache entry's
"h1:" directory hash, and cites the extracted source only when the recomputed
hash equals the committed one. Never fetches and never writes.

  --file PATH  Excerpt one file from the verified module directory.
  --limit N    Bound the listed file count.

A module the committed go.mod does not require, a cache entry that is absent,
and a hash that does not match are abstentions with a named reason, not errors.
`
