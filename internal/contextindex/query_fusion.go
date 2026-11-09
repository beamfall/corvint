package contextindex

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
