package contextindex

import "sort"

// ExtractionNote records that a source was admitted and indexed as text, but
// that its symbol extraction did not run cleanly over the whole file. It is the
// observable trace of a partial walk.
//
// The index already reports Exclusion for a path it refused outright. A note is
// the weaker, and more dangerous, case: the path IS in the index, its text IS
// searchable, and a caller reading only the source count would conclude the
// file was fully understood. Recording the reason here is what stops a capped
// or lexically broken extraction from being indistinguishable from a file that
// genuinely declares nothing.
type ExtractionNote struct{ Path, Reason string }

// Reasons a symbol walk stops short. Each names a condition the extractor can
// detect at the moment it gives up, so no caller has to infer truncation from a
// suspiciously round symbol count.
const (
	// noteSymbolCap: the file declares more symbols than one file may
	// contribute. The symbols reported are a prefix, not the whole set.
	noteSymbolCap = "symbol cap reached; symbols truncated"
	// noteLineCap: the file is longer than the scanner walks. Declarations
	// past the cap were never examined.
	noteLineCap = "line cap reached; tail of file not scanned"
	// noteUnterminatedComment: the file ended inside a block comment. Every
	// line after the opener was treated as comment text, so any declaration
	// among them was skipped.
	noteUnterminatedComment = "file ends inside an unterminated block comment"
	// noteUnterminatedString: the file ended inside a multi-line string
	// literal, with the same consequence for the lines it swallowed.
	noteUnterminatedString = "file ends inside an unterminated string literal"
)

// maxSymbolsPerSource bounds one file's contribution to the symbol table. The
// bound exists so a single generated or vendored file cannot dominate the
// index; reaching it is always recorded as a note.
const maxSymbolsPerSource = 5_000

// maxSourceLines bounds the scan length of one file for the same reason.
const maxSourceLines = 100_000

// sortNotes orders notes by path then reason so a merge across the concurrent
// source chunks is deterministic, matching how Exclusion is ordered.
func sortNotes(notes []ExtractionNote) {
	sort.Slice(notes, func(left, right int) bool {
		if notes[left].Path != notes[right].Path {
			return notes[left].Path < notes[right].Path
		}
		return notes[left].Reason < notes[right].Reason
	})
}
