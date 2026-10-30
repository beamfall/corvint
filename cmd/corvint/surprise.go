package main

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/touchsurprise"
)

// parseSurpriseInvocation recognizes `[--root PATH] surprise --task TEXT --base
// REV --target REV [--subject PATH] [--limit N]`, following observations.go's
// and depsource.go's argument conventions.
func parseSurpriseInvocation(arguments []string) (touchsurprise.Options, bool, error) {
	options := touchsurprise.Options{Limit: touchsurprise.DefaultLimit}
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
	if index >= len(arguments) || arguments[index] != "surprise" {
		return options, false, nil
	}
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if !surpriseFlags[name] {
			return options, true, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, true, argumentError("argument " + name + ": expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		if err := assignSurpriseFlag(&options, name, value); err != nil {
			return options, true, err
		}
	}
	if err := requireSurpriseArguments(options); err != nil {
		return options, true, err
	}
	resolved, err := resolveSurpriseRoot(root)
	if err != nil {
		return options, true, err
	}
	options.Root = resolved
	return options, true, nil
}

var surpriseFlags = map[string]bool{"--task": true, "--base": true, "--target": true, "--subject": true, "--limit": true}

func assignSurpriseFlag(options *touchsurprise.Options, name, value string) error {
	switch name {
	case "--task":
		options.Task = value
	case "--subject":
		options.Subject = value
	case "--base":
		options.Base = value
	case "--target":
		options.Target = value
	default:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > touchsurprise.MaxLimit {
			return argumentError("invalid --limit")
		}
		options.Limit = parsed
	}
	return nil
}

// requireSurpriseArguments refuses the arguments a revision comparison cannot
// do without, before any repository read. A revision spelled like an option is
// refused rather than passed to Git.
func requireSurpriseArguments(options touchsurprise.Options) error {
	if strings.TrimSpace(options.Task) == "" {
		return argumentError("argument --task: expected one non-empty task text")
	}
	for name, value := range map[string]string{"--base": options.Base, "--target": options.Target} {
		if value == "" {
			return argumentError("argument " + name + ": expected one revision")
		}
		if strings.HasPrefix(value, "-") {
			return argumentError("argument " + name + ": a revision must not begin with '-'")
		}
	}
	return nil
}

func resolveSurpriseRoot(root string) (string, error) {
	if root == "" {
		return normalizeRoot(".")
	}
	return resolveExplicitRoot(root)
}

// runSurprise reports how much of one committed change the task-context packet
// never named. It is read-only: it writes no repository or trace state, and an
// argument, revision or repository failure is exit status 2. ctx is main's
// signal context, so SIGINT and SIGTERM cancel the load like every other
// packet verb.
func runSurprise(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	options, isSurprise, err := parseSurpriseInvocation(arguments)
	if !isSurprise {
		return 2
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	index, hit, err := loadSnapshot(ctx, options.Root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if !hit {
		index, err = contextindex.BuildContext(ctx, options.Root, "")
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := touchsurprise.Render(ctx, index, options, stdout); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

const surpriseHelp = `
Touch-set surprise (experimental, TSS-V0 proposed), read-only, JSON:

  corvint [--root PATH] surprise --task TEXT --base REV --target REV
           [--subject PATH] [--limit N]

Compares the predicted touch set for one completed task -- the task-context
packet's paths plus the impact expansion of those paths -- against the paths the
committed range REV..REV actually changed. Reports predicted-only paths,
actual-only paths (the misses), the intersection, the symmetric-difference size
and surprise = |actual-only| / |actual| (0 with empty_actual for an empty
range). Each miss says whether it is tracked at the target revision and whether
its spelling looks like a test or doc path (heuristics, not extractor claims).

--limit defaults to 20 and is bounded at 50. Both revisions must resolve to a
commit; a dirty worktree is refused with unsupported-surprise-dirty-worktree,
since this is committed evidence only.
`
