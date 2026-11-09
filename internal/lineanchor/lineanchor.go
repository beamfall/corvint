// Package lineanchor builds the source anchors that the documentation corpus checks and that the
// static test discovery (internal/testsource) and the application-flow coverage ingest emit. It is
// split from internal/doccorpus (decision 0594) so test selection does not import the corpus.
package lineanchor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

// MaxSpanBytes bounds one evidence excerpt.
const MaxSpanBytes = 64 << 10

// Anchor pins one line span of one Git blob.
type Anchor struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	SHA256     string `json:"sha256"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256"`
	Symbol     string `json:"symbol"`
	Region     string `json:"region"`
	Authority  string `json:"authority"`
	Kind       string `json:"evidence_kind"`
	Reason     string `json:"reason"`
}

// Error is a coded refusal.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Fail is the corpus-refused refusal with message.
func Fail(message string) error { return &Error{Code: "corpus-refused", Message: message} }

// Digest is the lowercase hex SHA-256 of data.
func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// LineIndex locates 1-based line spans of one pinned Git blob so producers outside the corpus
// (the static test discovery and the coverage ingest, DCX-V1-370 and AFU-V1-112) build anchors
// exactly as the corpus compiler checks them: the whole blob's digest, the span's digest, and the
// same non-vacuous, MaxSpanBytes-bounded excerpt rule.
type LineIndex struct {
	repository, revision, path, blob, sha string
	data                                  []byte
	starts                                []int // byte offset of each line's first byte
}

// NewLineIndex indexes data, the bytes of blob at path in revision of repository.
func NewLineIndex(repository, revision, path, blob string, data []byte) *LineIndex {
	starts := []int{0}
	for i, c := range data {
		if c == '\n' && i+1 < len(data) {
			starts = append(starts, i+1)
		}
	}
	if len(data) == 0 {
		starts = nil
	}
	return &LineIndex{repository: repository, revision: revision, path: path, blob: blob, sha: Digest(data), data: data, starts: starts}
}

// Lines is the blob's line count; a final line without a newline counts.
func (x *LineIndex) Lines() int { return len(x.starts) }

// SHA256 is the whole blob's digest.
func (x *LineIndex) SHA256() string { return x.sha }

// Line is line n without its line terminator, or "" outside the blob.
func (x *LineIndex) Line(n int) string {
	if n < 1 || n > len(x.starts) {
		return ""
	}
	end := len(x.data)
	if n < len(x.starts) {
		end = x.starts[n]
	}
	return string(bytes.TrimRight(x.data[x.starts[n-1]:end], "\r\n"))
}

// Excerpt returns lines start..end inclusive, refusing exactly the spans the corpus compiler refuses.
func (x *LineIndex) Excerpt(start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, Fail("invalid anchor span")
	}
	if end > len(x.starts) {
		return nil, Fail("anchor span out of range")
	}
	stop := len(x.data)
	if end < len(x.starts) {
		stop = x.starts[end]
	}
	s := x.data[x.starts[start-1]:stop]
	if len(s) > MaxSpanBytes || len(bytes.TrimSpace(s)) == 0 {
		return nil, Fail("vacuous or oversized anchor")
	}
	return s, nil
}

// Anchor is the external-provider anchor of lines start..end with evidence kind and reason.
func (x *LineIndex) Anchor(start, end int, kind, reason string) (Anchor, error) {
	excerpt, err := x.Excerpt(start, end)
	if err != nil {
		return Anchor{}, err
	}
	return Anchor{Repository: x.repository, Revision: x.revision, Path: x.path, Blob: x.blob, SHA256: x.sha, Start: start, End: end,
		SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: kind, Reason: reason}, nil
}

// Bytes is the indexed blob's content; callers must not modify it.
func (x *LineIndex) Bytes() []byte { return x.data }
