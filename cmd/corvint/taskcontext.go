package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// taskContextOptions is the parsed `context` invocation (task-context-packet-v0).
type taskContextOptions struct {
	root, task, subject string
	limit               int
	// summary, summaryBytes, expand and maxBytes are the opt-in experimental
	// consumers (TCP-V0-024, ESV-V0-008/009); absent, the wire is unchanged.
	summary                 bool
	summaryBytes, maxBytes  int
	expand                  string
	summarySet, limitSet    bool
	summaryBytesSet, maxSet bool
}

const taskContextDefaultLimit = 20

// parseTaskContextInvocation recognises `[--root PATH] context --task TEXT
// [--subject PATH] [--limit N] [--summary [--summary-bytes N]]` and
// `[--root PATH] context --expand HANDLE [--max-bytes N]`; any other shape is
// not a context invocation.
func parseTaskContextInvocation(arguments []string) (taskContextOptions, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return taskContextOptions{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "context" {
		return taskContextOptions{}, false, nil
	}
	options := taskContextOptions{root: ".", limit: taskContextDefaultLimit, summaryBytes: contextSummaryDefaultBytes, maxBytes: contextExpandDefaultBytes}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	taskSet := false
	rest := arguments[position+1:]
	for index := 0; index < len(rest); index++ {
		if rest[index] == "--summary" {
			options.summary, options.summarySet = true, true
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
		case "--task":
			options.task, taskSet = value, true
		case "--subject":
			options.subject = value
		case "--limit":
			limit, err := strconv.Atoi(value)
			if err != nil {
				return options, true, argumentError("argument --limit: invalid int value: " + strconv.Quote(value))
			}
			options.limit, options.limitSet = limit, true
		case "--summary-bytes", "--max-bytes":
			bytes, err := strconv.Atoi(value)
			if err != nil {
				return options, true, argumentError("argument " + flag + ": invalid int value: " + strconv.Quote(value))
			}
			if flag == "--max-bytes" {
				options.maxBytes, options.maxSet = bytes, true
				continue
			}
			options.summaryBytes, options.summaryBytesSet = bytes, true
		case "--expand":
			options.expand = value
		default:
			return options, true, argumentError("unrecognized arguments: " + rest[index])
		}
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

// checkContextViewArguments refuses mixed or orphaned view flags: --expand
// stands alone except --max-bytes, and --summary-bytes needs --summary.
func checkContextViewArguments(options taskContextOptions, taskSet bool) error {
	if options.expand != "" && (taskSet || options.subject != "" || options.limitSet || options.summarySet || options.summaryBytesSet) {
		return argumentError("argument --expand: not allowed with --task, --subject, --limit, --summary or --summary-bytes")
	}
	if options.expand == "" && options.maxSet {
		return argumentError("argument --max-bytes: requires --expand")
	}
	if options.summaryBytesSet && !options.summarySet {
		return argumentError("argument --summary-bytes: requires --summary")
	}
	if options.expand == "" && !taskSet {
		return argumentError("the following arguments are required: --task")
	}
	return nil
}

// runTaskContext compiles one packet and prints it. Read-only: the tree's
// snapshot when `corvint index` wrote one, else one index build over the
// committed tree; no trace, ledger, or snapshot write on any path.
func runTaskContext(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	if options.expand != "" {
		return runContextExpand(ctx, options, stdout, stderr)
	}
	// CPUPROFILE (V1-0051): operator env var, off by default, documented in
	// cpuProfileHelpNote (help.go) and task-context-packet-v0.md's Non-goals
	// and authority section; it writes a local diagnostic file and does not
	// widen what this read-only command reads, returns, or mutates.
	if profilePath := runtimeenv.Value("CPUPROFILE"); profilePath != "" {
		profile, err := os.Create(profilePath)
		if err != nil {
			emitError(stderr, err)
			return 2
		}
		if err := pprof.StartCPUProfile(profile); err != nil {
			profile.Close()
			emitError(stderr, err)
			return 2
		}
		defer func() {
			pprof.StopCPUProfile()
			profile.Close()
		}()
	}
	packet, hit, err := compileTaskContext(ctx, options, loadContextSnapshot)
	if errors.Is(err, contextindex.ErrSnapshotRefused) {
		packet, hit, err = compileTaskContext(ctx, options, contextindex.LoadContextSnapshot)
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
	if options.summary {
		encoded, err = summarizeContextPacket(append(encoded, '\n'), options.summaryBytes)
		if err != nil {
			emitError(stderr, err)
			return 2
		}
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

Read-only and Go-only (task-context-packet-v0, experimental). The packet lists
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

--subject names the task's own path (a changed or commented file). It is
carried under "subject" and never appears among "results": it is the subject
of the question, not one of its answers. Without --subject the packet has the
retrieval shape (mentioned, definition, lexical). "state" is READY when at
least one result exists and NO_CANDIDATES otherwise; --limit defaults to 20.

Experimental opt-in views (experimental-source-views-v0, ESV-V0-008..010);
without these flags the packet bytes are unchanged:

  corvint [--root PATH] context --task TEXT [--subject PATH] [--limit N]
    --summary [--summary-bytes N]
  corvint [--root PATH] context --expand HANDLE [--max-bytes N]

--summary prints the same packet with compact result rows, at most
--summary-bytes (default 8192, 1024..65536) including the newline. Every other
member, coverage included, is kept verbatim; a budget that cannot hold every
critical row refuses. "summary" gives the full packet's sha256, the row totals
and the continuation route. Each pinned row carries a handle
cv1:TREE:BLOB:RANGE:PATH (RANGE is all or START-END). --expand prints that
handle's exact bytes from Git objects with the verified blob identity, at most
--max-bytes (default 65536, up to 1048576); an invalid, stale, missing or
ambiguous handle refuses and never substitutes current content.
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
// compiles the packet.
func compileTaskContext(ctx context.Context, options taskContextOptions, load func(context.Context, string) (*contextindex.Index, bool, *contextindex.LoaderObservation, error)) (map[string]any, bool, error) {
	index, hit, opening, err := load(ctx, options.root)
	if err != nil {
		return nil, hit, err
	}
	if !hit {
		index, err = contextindex.BuildContextObserved(ctx, options.root, options.subject, opening)
	}
	if err != nil {
		return nil, hit, err
	}
	admitted, err := contextindex.LoadAdmittedSlotWeights(options.root)
	if err != nil {
		return nil, hit, err
	}
	packet, err := contextindex.TaskContextWeighted(ctx, index, options.task, options.subject, options.limit, admitted)
	return packet, hit, err
}
