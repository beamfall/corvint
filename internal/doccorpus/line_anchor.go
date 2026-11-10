package doccorpus

import "github.com/Beamfall/corvint/internal/lineanchor"

// LineIndex and NewLineIndex moved to internal/lineanchor (decision 0594); these names keep the
// corpus's remaining callers compiling until the corpus itself is removed.
type LineIndex = lineanchor.LineIndex

// NewLineIndex indexes data, the bytes of blob at path in revision of repository.
func NewLineIndex(repository, revision, path, blob string, data []byte) *LineIndex {
	return lineanchor.NewLineIndex(repository, revision, path, blob, data)
}
