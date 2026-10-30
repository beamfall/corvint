package contextindex

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// LexicalCandidate is one source the task's lexical channel ranks
// (GPK-V0-092): the rendered packet row, the BM25 score behind it and the
// task identifiers the source names as written, so a fused widening row can
// explain its inclusion from the channel's own evidence.
type LexicalCandidate struct {
	Path string
	// Row is the packet row of the hit, rendered as the task-context slot
	// would render it (`kind` lexical or documentation, `evidence[0]`).
	Row map[string]any
	// BM25 is the channel's score over the body, path and identifier fields.
	BM25 float64
	// Identifiers are the task identifiers whose exact spelling the source
	// contains, ascending; empty when the hit rests on prose terms alone.
	Identifiers []string
}

// LexicalChannel is the task's lexical identifier channel: the BM25 ranking
// over the immutable index's body, path and whole-identifier postings for one
// task, in channel order, with what the ranking searched for.
type LexicalChannel struct {
	Candidates []LexicalCandidate
	// Identifiers are the task's identifiers the channel searched for by
	// exact spelling, ascending; empty when the task names none.
	Identifiers []string
	// Terms is the number of lexical task terms the channel walked.
	Terms int
}

// TaskLexicalChannel ranks the index's sources for one task by the lexical
// channel alone, returning at most limit candidates. It runs the same posting
// walk the task-context lexical fill runs, over the same pinned index, so a
// candidate's row is byte-identical to the row the fill would have carried;
// it adds no relation, authority or recency evidence. A cancelled ctx returns
// its error.
func TaskLexicalChannel(ctx context.Context, index *Index, task string, limit int) (LexicalChannel, error) {
	if limit < 1 || limit > maxLimit {
		return LexicalChannel{}, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	task = strings.TrimSpace(task)
	if task == "" {
		return LexicalChannel{}, &Error{Message: "task text must be non-empty"}
	}
	if len(task) > maxTaskContextChars {
		return LexicalChannel{}, &Error{Message: fmt.Sprintf("task text exceeds %d characters", maxTaskContextChars)}
	}
	compiler := newTaskContextCompiler(index, task, "")
	compiler.ctx = ctx
	defer compiler.join()
	hits := compiler.lexicalHits()
	if compiler.stopped("lexical") {
		return LexicalChannel{}, compiler.cancelled
	}
	channel := LexicalChannel{Terms: len(compiler.terms)}
	table := compiler.index.vocabulary()
	named := map[uint32][]string{}
	for _, identifier := range compiler.lexicalIdentifiers() {
		// The same admission the posting walk applies: without a selected
		// term set, only a multi-word identifier is searched by exact spelling.
		if compiler.selectedTerms == nil && (len(identifier.name) < 3 || !hasInnerCamelBoundary(identifier.name) && !strings.Contains(identifier.name, "_")) {
			continue
		}
		channel.Identifiers = append(channel.Identifiers, identifier.name)
		low, high, ok := table.Words.find(identifier.name)
		if !ok {
			continue
		}
		for index := low; index < high; index++ {
			source := table.Words.Sources[index]
			named[source] = append(named[source], identifier.name)
		}
	}
	sort.Strings(channel.Identifiers)
	if compiler.stopped("lexical") {
		// The identifier scan is a second posting walk; a request cancelled
		// during it is refused, not answered.
		return LexicalChannel{}, compiler.cancelled
	}
	strongest := 0.0
	if len(hits) > 0 {
		strongest = hits[0].score
	}
	for _, hit := range hits {
		if len(channel.Candidates) == limit {
			break
		}
		identifiers := append([]string(nil), named[hit.source]...)
		sort.Strings(identifiers)
		channel.Candidates = append(channel.Candidates, LexicalCandidate{
			Path: hit.path, Row: compiler.renderRow(lexicalContextRow(hit, strongest)),
			BM25: hit.score, Identifiers: identifiers,
		})
	}
	return channel, nil
}
