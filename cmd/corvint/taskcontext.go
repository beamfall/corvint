package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// taskContextOptions is the parsed `context` invocation: the path brief (TBC-V1-009) or the
// instruction doctor (IID-V0-001). The task packet is removed (decision 0597).
type taskContextOptions struct {
	root, subject                                                           string
	instructionHost, instructionVersion, instructionCWD, instructionProfile string
	instructionFlags                                                        map[string]bool
	// paths is --path, the path brief (TBC-V1-009); set, no other field but root applies.
	paths []string
}

// contextTaskRemoved is the refusal for the removed task packet (decision 0597).
const contextTaskRemoved = "corvint context no longer compiles a task packet (decision 0597); for a change run corvint affected --base <sha> or corvint impact PATH, and for one file run corvint context --path PATH"

// parseTaskContextInvocation recognises `[--root PATH] context ...`; any other shape is not a
// context invocation.
func parseTaskContextInvocation(arguments []string) (taskContextOptions, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return taskContextOptions{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "context" {
		return taskContextOptions{}, false, nil
	}
	options := taskContextOptions{root: "."}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	rest := arguments[position+1:]
	if isPathContext(rest) {
		options, err := parsePathContextArguments(options, rest)
		if err != nil {
			return options, true, err
		}
		options.root, err = resolveExplicitRoot(options.root)
		return options, true, err
	}
	for index := 0; index < len(rest); index++ {
		if !argparseOptionLike(rest[index]) {
			return options, true, argumentError(contextTaskRemoved)
		}
		flag, value, inline := strings.Cut(rest[index], "=")
		switch flag {
		case "--task", "--limit", "--worktree-overlay":
			return options, true, argumentError(contextTaskRemoved)
		case "--subject", "--instruction-host", "--instruction-host-version", "--instruction-cwd", "--instruction-profile":
		default:
			return options, true, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, true, argumentError("argument " + flag + ": expected one argument")
			}
			value, index = rest[index+1], index+1
		}
		if flag == "--subject" {
			options.subject = value
			continue
		}
		if options.instructionFlags == nil {
			options.instructionFlags = make(map[string]bool)
		}
		if options.instructionFlags[flag] {
			return options, true, argumentError("argument " + flag + ": may not be repeated")
		}
		options.instructionFlags[flag] = true
		switch flag {
		case "--instruction-host":
			options.instructionHost = value
		case "--instruction-host-version":
			options.instructionVersion = value
		case "--instruction-cwd":
			options.instructionCWD = value
		case "--instruction-profile":
			options.instructionProfile = value
		}
	}
	if err := checkContextViewArguments(options); err != nil {
		return options, true, err
	}
	resolved, err := resolveExplicitRoot(options.root)
	if err != nil {
		return options, true, err
	}
	options.root = resolved
	return options, true, nil
}

// checkContextViewArguments refuses incomplete instruction-prediction flags and a bare context.
func checkContextViewArguments(options taskContextOptions) error {
	if len(options.instructionFlags) == 0 && options.subject == "" {
		return argumentError("the following arguments are required: --path")
	}
	if len(options.instructionFlags) != 4 || options.subject == "" {
		return argumentError("instruction prediction requires --instruction-host, --instruction-host-version, --instruction-cwd, --instruction-profile default and --subject")
	}
	if options.instructionProfile != "default" {
		return argumentError("argument --instruction-profile: requires default, an explicit conditional assumption")
	}
	return nil
}

// runTaskContext prints the path brief or the instruction load set. Read-only: the tree's
// snapshot when `corvint index` wrote one, else one index build over the committed tree.
func runTaskContext(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	if len(options.paths) > 0 {
		return runPathContext(ctx, options, stdout, stderr)
	}
	loadSet, err := compileInstructionLoadSet(ctx, options, contextindex.LoadContextSnapshotDeferred)
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		loadSet, err = compileInstructionLoadSet(ctx, options, contextindex.LoadContextSnapshot)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := gokernel.WriteCanonicalJSON(stdout, map[string]any{"instruction_load_set": loadSet, "mutates": false}); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write instruction load set"})
		return 2
	}
	return 0
}

const taskContextHelp = `Print the path brief of a file or directory, or the instruction load set.

Usage:
  corvint [--root PATH] context --path PATH [--path PATH ...]
  corvint [--root PATH] context --subject PATH
    --instruction-host HOST --instruction-host-version VERSION
    --instruction-cwd DIR --instruction-profile default

The --path form prints the path brief (corvint-path-brief/0, non-blocking): for
each file or directory, read from the committed tree at HEAD, six sections in a
fixed order: instructions, owners, tests, flows, requirements, tickets. Each
section states OBSERVED, INCOMPLETE or NOT_OBSERVED with a reason, a total and
whether it was cut at 64 items, and every item names the link that put it
there. Nothing is ranked. A missing optional source (corvint-tasks, CODEOWNERS,
the flows file, test evidence, the requirement-link file) is named in its
section and never fails the command. --path takes no task text, ticket or
change range; the report for a change range is the corvint impact command.

The instruction flags print an experimental conditional project-file prediction
(IID-V0), currently qualified only for Codex 0.153.2 default source rules. All four
flags and a tracked --subject are required together. Actual host configuration,
global instructions and session load state remain UNKNOWN. Predictions never
change repository governance or detect semantic conflicts.

Writes nothing. The task packet (--task) is removed (decision 0597): for a
change run corvint affected --base SHA or corvint impact PATH.
`

// compileInstructionLoadSet reads the snapshot through load, builds on a miss, and returns the
// instruction load set for the subject.
func compileInstructionLoadSet(ctx context.Context, options taskContextOptions, load func(context.Context, string) (*contextindex.Index, bool, *contextindex.LoaderObservation, error)) (map[string]any, error) {
	index, hit, opening, err := load(ctx, options.root)
	if err != nil {
		return nil, err
	}
	if !hit {
		index, err = contextindex.BuildContextObserved(ctx, options.root, options.subject, opening)
		if err != nil {
			return nil, err
		}
	}
	loadSet, err := contextindex.InstructionLoadSet(index, options.subject, contextindex.InstructionLoadOptions{
		Host: options.instructionHost, Version: options.instructionVersion, CWD: options.instructionCWD, Profile: options.instructionProfile,
	})
	if err == nil {
		// A deferred pack body that failed verification refuses the read; the caller reloads
		// eagerly (IDX-SNAP-V0-814, IDX-SNAP-V0-815).
		err = index.SnapshotRefusal()
	}
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		index.Release()
	}
	return loadSet, err
}
