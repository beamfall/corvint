package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// taskContextOptions is the parsed `context` or `instructions` invocation: the path brief
// (TBC-V1-009) or the instruction doctor (IID-V0-001). The task packet is removed (decision 0597).
type taskContextOptions struct {
	root, subject                                                           string
	instructionHost, instructionVersion, instructionCWD, instructionProfile string
	instructionFlags                                                        map[string]bool
	// paths is --path, the path brief (TBC-V1-009); set, no other field but root applies.
	paths []string
	// deprecatedAlias marks the `context --instruction-*` spelling, which prints one stderr line
	// naming `corvint instructions` (IID-V0-001).
	deprecatedAlias bool
}

// instructionAliasDeprecation is the stderr line of the `context --instruction-*` alias.
const instructionAliasDeprecation = "corvint: context --subject --instruction-* is deprecated and stays for one release; run corvint instructions --subject PATH --host HOST --host-version VERSION --cwd DIR --profile default\n"

// contextTaskRemoved is the refusal for the removed task packet (decision 0597).
const contextTaskRemoved = "corvint context no longer compiles a task packet (decision 0597); for a change run corvint affected --base <sha> or corvint impact PATH, and for one file run corvint context --path PATH"

// parseTaskContextInvocation recognises `[--root PATH] context ...`; any other shape is not a
// context invocation.
func parseTaskContextInvocation(arguments []string) (taskContextOptions, bool, error) {
	options, rest, requested := verbInvocation(arguments, "context")
	if !requested {
		return options, false, nil
	}
	if isPathContext(rest) {
		options, err := parsePathContextArguments(options, rest)
		if err != nil {
			return options, true, err
		}
		options.root, err = resolveExplicitRoot(options.root)
		return options, true, err
	}
	options.deprecatedAlias = true
	options, err := parseInstructionArguments(options, rest, "--instruction-")
	return options, true, err
}

// parseInstructionsInvocation recognises `[--root PATH] instructions ...`, the instruction doctor
// (IID-V0-001); any other shape is not an instructions invocation.
func parseInstructionsInvocation(arguments []string) (taskContextOptions, bool, error) {
	options, rest, requested := verbInvocation(arguments, "instructions")
	if !requested {
		return options, false, nil
	}
	options, err := parseInstructionArguments(options, rest, "--")
	return options, true, err
}

// verbInvocation returns the leading --root and the arguments after verb, or false when the
// arguments ask for help or name another verb.
func verbInvocation(arguments []string, verb string) (taskContextOptions, []string, bool) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return taskContextOptions{}, nil, false
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != verb {
		return taskContextOptions{}, nil, false
	}
	options := taskContextOptions{root: "."}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	return options, arguments[position+1:], true
}

// parseInstructionArguments reads --subject and the four instruction flags, each spelled prefix
// plus host, host-version, cwd or profile. The context alias refuses the removed task packet.
func parseInstructionArguments(options taskContextOptions, rest []string, prefix string) (taskContextOptions, error) {
	for index := 0; index < len(rest); index++ {
		if !argparseOptionLike(rest[index]) {
			if options.deprecatedAlias {
				return options, argumentError(contextTaskRemoved)
			}
			return options, argumentError("unrecognized arguments: " + rest[index])
		}
		flag, value, inline := strings.Cut(rest[index], "=")
		name := strings.TrimPrefix(flag, prefix)
		switch {
		case options.deprecatedAlias && (flag == "--task" || flag == "--limit" || flag == "--worktree-overlay"):
			return options, argumentError(contextTaskRemoved)
		case flag == "--subject":
		case strings.HasPrefix(flag, prefix) && (name == "host" || name == "host-version" || name == "cwd" || name == "profile"):
		default:
			return options, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, argumentError("argument " + flag + ": expected one argument")
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
			return options, argumentError("argument " + flag + ": may not be repeated")
		}
		options.instructionFlags[flag] = true
		switch name {
		case "host":
			options.instructionHost = value
		case "host-version":
			options.instructionVersion = value
		case "cwd":
			options.instructionCWD = value
		case "profile":
			options.instructionProfile = value
		}
	}
	if err := checkInstructionArguments(options, prefix); err != nil {
		return options, err
	}
	resolved, err := resolveExplicitRoot(options.root)
	if err != nil {
		return options, err
	}
	options.root = resolved
	return options, nil
}

// checkInstructionArguments refuses incomplete instruction-prediction flags and a bare context.
func checkInstructionArguments(options taskContextOptions, prefix string) error {
	if options.deprecatedAlias && len(options.instructionFlags) == 0 && options.subject == "" {
		return argumentError("the following arguments are required: --path")
	}
	if len(options.instructionFlags) != 4 || options.subject == "" {
		return argumentError("instruction prediction requires " + prefix + "host, " + prefix + "host-version, " + prefix + "cwd, " + prefix + "profile default and --subject")
	}
	if options.instructionProfile != "default" {
		return argumentError("argument " + prefix + "profile: requires default, an explicit conditional assumption")
	}
	return nil
}

// runTaskContext prints the path brief or the instruction load set. Read-only: the tree's
// snapshot when `corvint index` wrote one, else one index build over the committed tree.
func runTaskContext(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	if len(options.paths) > 0 {
		return runPathContext(ctx, options, stdout, stderr)
	}
	if options.deprecatedAlias {
		_, _ = io.WriteString(stderr, instructionAliasDeprecation)
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

const taskContextHelp = `Print the path brief of a file or directory.

Usage:
  corvint [--root PATH] context --path PATH [--path PATH ...]

The path brief (corvint-path-brief/1, non-blocking): for each file or
directory, read from the committed tree at HEAD, three sections in a fixed
order: instructions, tests, requirements. Each section states OBSERVED,
INCOMPLETE or NOT_OBSERVED with a reason, a total and whether it was cut at 64
items, and every item names the link that put it there. Nothing is ranked. A
missing optional source (test evidence, the affected plan, the requirement-link
file) is named in its section and never fails the command. --path takes no
task text, ticket or change range; the report for a change range is the
corvint impact command.

The instruction load set moved to corvint instructions. For one release,
context --subject PATH with --instruction-host, --instruction-host-version,
--instruction-cwd and --instruction-profile default prints the same object and
one deprecation line on stderr.

Writes nothing. The task packet (--task) is removed (decision 0597): for a
change run corvint affected --base SHA or corvint impact PATH.
`

const instructionsHelp = `Print the instruction load set of a tracked file (experimental, IID-V0).

Usage:
  corvint [--root PATH] instructions --subject PATH --host HOST
    --host-version VERSION --cwd DIR --profile default

Prints a conditional project-file prediction: which tracked AGENTS.md and
CLAUDE.md files a host would load for the subject, read from the committed
tree. It is currently qualified only for Codex 0.153.2 default source rules.
All four flags and a tracked --subject are required together; --cwd is
repository-relative and an ancestor of the subject. Actual host configuration,
global instructions and session load state remain UNKNOWN. Predictions never
change repository governance or detect semantic conflicts.

Writes nothing.
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
