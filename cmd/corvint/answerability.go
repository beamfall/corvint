package main

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/disagree"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// answerabilityOptions is one parsed `answerability` invocation
// (retriever-disagreement-v0, proposed).
type answerabilityOptions struct {
	root, task, subject string
	limit               int
}

const answerabilityDefaultLimit = 10

// answerabilityMaxLimit is the packet compiler's own bound. Accepting more here
// would refuse only after a full repository read.
const answerabilityMaxLimit = 50

// parseAnswerabilityInvocation recognises
// `[--root PATH] answerability --task TEXT [--subject PATH] [--limit N]`.
func parseAnswerabilityInvocation(arguments []string) (answerabilityOptions, bool, error) {
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "answerability" {
		return answerabilityOptions{}, false, nil
	}
	options := answerabilityOptions{root: ".", limit: answerabilityDefaultLimit}
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			options.root, index = arguments[index+1], index+1
			continue
		}
		options.root = strings.TrimPrefix(arguments[index], "--root=")
	}
	rest := arguments[position+1:]
	for index := 0; index < len(rest); index++ {
		flag, value, inline := strings.Cut(rest[index], "=")
		if flag != "--task" && flag != "--subject" && flag != "--limit" {
			return options, true, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, true, argumentError("argument " + flag + ": expected one argument")
			}
			value, index = rest[index+1], index+1
		}
		switch flag {
		case "--task":
			options.task = value
		case "--subject":
			options.subject = value
		default:
			limit, err := strconv.Atoi(value)
			if err != nil {
				return options, true, argumentError("argument --limit: invalid int value: " + strconv.Quote(value))
			}
			if limit < 1 || limit > answerabilityMaxLimit {
				return options, true, argumentError("invalid --limit")
			}
			options.limit = limit
		}
	}
	if strings.TrimSpace(options.task) == "" {
		return options, true, argumentError("argument --task: expected one argument")
	}
	resolved, err := resolveExplicitRoot(options.root)
	if err != nil {
		return options, true, err
	}
	options.root = resolved
	return options, true, nil
}

// runAnswerability reports the agreement of the lexical and structural channels
// as a proposed answerability state. It reads the snapshot `corvint index`
// wrote when there is one, else builds one index over the committed tree,
// exactly as `context --task` does; nothing is written on any path. ctx is
// main's signal context, so SIGINT and SIGTERM cancel the load like every
// other packet verb.
func runAnswerability(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	options, isAnswerability, err := parseAnswerabilityInvocation(arguments)
	if !isAnswerability {
		return 2
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	index, hit, err := loadSnapshot(ctx, options.root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if !hit {
		index, err = contextindex.BuildContext(ctx, options.root, options.subject)
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	report, err := disagree.Signal(ctx, index, options.task, options.subject, options.limit)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(report)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write answerability report"})
		return 2
	}
	return 0
}

const answerabilityHelp = `
Retriever disagreement (experimental, RDS-V0 proposed), read-only, JSON:

  corvint [--root PATH] answerability --task TEXT [--subject PATH] [--limit N]

Builds two candidate sets for the task from independent channels and reports
their agreement as an answerability signal:

  channel_l  the ordinary task-context packet's ranked paths (lexical)
  channel_s  paths reached only through index structure: the task's identifier
             tokens that name declared symbols, the files declaring them, and
             one hop of the import graph in each direction

The signal block carries the top-K Jaccard overlap, the overlap-weighted rank
correlation, and agreement high|low|none with proposed_state
answerable|uncertain|unanswerable. A task naming no declared symbol reports
structural_vote absent and never proposes answerable on one channel alone.
This is a proposal for the abstention layer: it changes no threshold, no packet,
and no stored state. --limit defaults to 10.
`
