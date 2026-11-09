package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// taskContextOptions is the parsed `context` invocation (task-context-packet-v0).
type taskContextOptions struct {
	root, task, subject                                                     string
	instructionHost, instructionVersion, instructionCWD, instructionProfile string
	instructionFlags                                                        map[string]bool
	limit                                                                   int
	limitSet                                                                bool
	// overlay is --worktree-overlay (DIRTY-CACHE-940); absent, the wire is unchanged.
	overlay bool
}

const taskContextDefaultLimit = 20

// parseTaskContextInvocation recognises `[--root PATH] context (--task TEXT | TEXT)
// [--subject PATH] [--limit N]`; any other shape is not a context invocation.
func parseTaskContextInvocation(arguments []string) (taskContextOptions, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return taskContextOptions{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "context" {
		return taskContextOptions{}, false, nil
	}
	options := taskContextOptions{root: ".", limit: taskContextDefaultLimit}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	taskSet, flagTaskSet := false, false
	positionals := []string{}
	rest := arguments[position+1:]
	for index := 0; index < len(rest); index++ {
		if !argparseOptionLike(rest[index]) {
			positionals = append(positionals, rest[index])
			continue
		}
		if rest[index] == worktreeOverlayFlag {
			options.overlay = true
			continue
		}
		flag, value, inline := strings.Cut(rest[index], "=")
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, true, argumentError("argument " + flag + ": expected one argument")
			}
			value, index = rest[index+1], index+1
		}
		switch flag {
		case "--instruction-host", "--instruction-host-version", "--instruction-cwd", "--instruction-profile":
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
		case "--task":
			options.task, taskSet, flagTaskSet = value, true, true
		case "--subject":
			options.subject = value
		case "--limit":
			limit, err := strconv.Atoi(value)
			if err != nil {
				return options, true, argumentError("argument --limit: invalid int value: " + strconv.Quote(value))
			}
			options.limit, options.limitSet = limit, true
		default:
			return options, true, argumentError("unrecognized arguments: " + rest[index])
		}
	}
	// TCP-V0-062 (V1-0993): one positional is the task when --task is absent.
	switch {
	case len(positionals) > 1:
		return options, true, argumentError(contextPositionalTaskRefusal("context accepts at most one positional task"))
	case len(positionals) == 1 && flagTaskSet:
		return options, true, argumentError(contextPositionalTaskRefusal("argument --task: not allowed with a positional task"))
	case len(positionals) == 1:
		options.task, taskSet = positionals[0], true
	}
	if err := checkContextViewArguments(options, taskSet); err != nil {
		return options, true, err
	}
	resolved, err := resolveExplicitRoot(options.root)
	if err != nil {
		return options, true, err
	}
	options.root = resolved
	return options, true, nil
}

// contextPositionalTaskRefusal appends the one copyable form to a refused
// positional-task shape (TCP-V0-062).
func contextPositionalTaskRefusal(reason string) string {
	return reason + "; quote the task once or pass --task, for example: corvint context --task \"fix the parser\""
}

// checkContextViewArguments refuses incomplete instruction-prediction flags and a missing task.
func checkContextViewArguments(options taskContextOptions, taskSet bool) error {
	if len(options.instructionFlags) > 0 {
		if len(options.instructionFlags) != 4 || options.subject == "" {
			return argumentError("instruction prediction requires --instruction-host, --instruction-host-version, --instruction-cwd, --instruction-profile default and --subject")
		}
		if options.instructionProfile != "default" {
			return argumentError("argument --instruction-profile: requires default, an explicit conditional assumption")
		}
	}
	if !taskSet {
		return argumentError("the following arguments are required: --task")
	}
	return nil
}

// runTaskContext compiles one packet and prints it. Read-only: the tree's
// snapshot when `corvint index` wrote one, else one index build over the
// committed tree; no trace, ledger, or snapshot write on any path.
func runTaskContext(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	load := loadContextSnapshot
	if options.overlay {
		// The overlay reuses every HEAD source, so it loads them eagerly.
		load = contextindex.LoadContextSnapshot
	}
	packet, hit, built, err := compileTaskContext(ctx, options, load)
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		packet, hit, built, err = compileTaskContext(ctx, options, contextindex.LoadContextSnapshot)
	}
	if err == nil && !hit {
		noteSnapshotMissBuild(stderr, "context", options.root, built)
	}
	if runtimeenv.Value("BENCH_SNAPSHOT_TRACE") == "1" {
		fmt.Fprintf(stderr, "corvint-bench-snapshot: hit=%t\n", hit)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(packet)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write task-context packet"})
		return 2
	}
	return 0
}

const taskContextHelp = `Compile the task-context packet: the files to read for one task.

Usage:
  corvint [--root PATH] context --task TEXT [--subject PATH] [--limit N]
  corvint [--root PATH] context --task TEXT --subject PATH
    --instruction-host HOST --instruction-host-version VERSION
    --instruction-cwd DIR --instruction-profile default
  corvint [--root PATH] context TEXT [...]   (one positional TEXT is --task TEXT)

Writes nothing; Go-only (task-context-packet-v0, experimental). The packet lists
files admitted by relations a term search cannot express, each with one
evidence line naming the relation, then fills the remaining slots with the
files a term search would list:

  pair            the test or source counterpart of the subject (or of a path
                  the task names), by naming convention, module directory, or
                  mirrored test/source directory
  mentioned       a tracked path the task names, by full path or unambiguous
                  basename
  definition      a file defining an identifier the task names
  reverse-import  a file importing the subject (Go, Python, web rules of impact)
  cochange        a file that changed in the same recent commits as the subject
                  (the last 200 non-merge commits; commits over a cap ignored,
                  50 paths for a history of 50 commits or fewer, 8 for one
                  filling the window, linear between)
  sibling         the subject's directory, then its parent subtree where the
                  task's identifiers appear
  lexical         distinct task terms in the file or its path

The instruction flags request an experimental conditional project-file prediction
(IID-V0), currently qualified only for Codex 0.153.2 default source rules. All four
flags and a tracked --subject are required together. Actual host configuration, global instructions and session load state remain
UNKNOWN. Predictions never change repository governance or detect semantic conflicts.

--subject names the task's own path (a changed or commented file). It is
carried under "subject" and never appears among "results": it is the subject
of the question, not one of its answers. Without --subject the packet has the
retrieval shape (mentioned, definition, lexical). "state" is READY when at
least one result exists and NO_CANDIDATES otherwise; --limit defaults to 20.

--worktree-overlay (experimental, DIRTY-CACHE-940..942) reads tracked dirty
paths from the worktree over the HEAD snapshot instead of the committed tree.
Rows citing them carry authority worktree-overlay, low confidence and the
worktree blob id; untracked paths are never read. It writes nothing.
`

// loadContextSnapshot is the context verb's snapshot read. On a miss it also
// carries the loader's repository observation to the build, so the miss
// costs one identity/status pair instead of two (IDX-SNAP-V0-002: the
// loader reads the repository exactly as the build's opening observation).
// A pack hit verifies each body when the packet first reads it; a body that
// fails is ErrSnapshotRefused, and runTaskContext recompiles through
// LoadContextSnapshot, which refuses the pack whole (IDX-SNAP-V0-015).
var loadContextSnapshot = contextindex.LoadContextSnapshotDeferred

// compileTaskContext reads the snapshot through load, builds on a miss, and
// compiles the packet. It also returns how long a miss's build took.
func compileTaskContext(ctx context.Context, options taskContextOptions, load func(context.Context, string) (*contextindex.Index, bool, *contextindex.LoaderObservation, error)) (map[string]any, bool, time.Duration, error) {
	index, hit, opening, err := load(ctx, options.root)
	if err != nil {
		return nil, hit, 0, err
	}
	var built time.Duration
	if !hit {
		started := time.Now()
		index, err = contextindex.BuildContextObserved(ctx, options.root, options.subject, opening)
		built = time.Since(started)
	}
	if err != nil {
		return nil, hit, built, err
	}
	if options.overlay {
		if index, err = contextindex.OverlayWorktree(ctx, index); err != nil {
			return nil, hit, built, err
		}
	}
	packet, err := contextindex.TaskContext(ctx, index, options.task, options.subject, options.limit)
	if err == nil && len(options.instructionFlags) > 0 {
		var loadSet map[string]any
		loadSet, err = contextindex.InstructionLoadSet(index, options.subject, contextindex.InstructionLoadOptions{
			Host: options.instructionHost, Version: options.instructionVersion, CWD: options.instructionCWD, Profile: options.instructionProfile,
		})
		if err == nil {
			packet["instruction_load_set"] = loadSet
		}
	}
	// The instruction load set reads bodies after
	// TaskContext's own refusal check; a deferred body that failed there
	// refuses the packet too, so runTaskContext reloads eagerly
	// (IDX-SNAP-V0-814).
	if err == nil {
		err = index.SnapshotRefusal()
	}
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		// The caller reloads eagerly; this read's mapping ends here
		// (IDX-SNAP-V0-815).
		index.Release()
	}
	return packet, hit, built, err
}
