package doccorpus

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strconv"
)

// Anchor states a reader reports for each cited span (DCP-V1-282).
const (
	AnchorStateLive    = "live"
	AnchorStateMoved   = "moved"
	AnchorStateChanged = "changed"
	AnchorStateGone    = "gone"
	AnchorStateUnknown = "unknown"
)

// MaxGapKinds bounds a docs_gaps kind filter (DCP-V1-284).
const MaxGapKinds = 32

// Per-request bounds on the live bytes read and hashed to place cited spans;
// a span past either bound is unknown, never live (DCP-V1-282).
const (
	anchorStateMaxFileBytes   = 8 << 20
	anchorStateMaxReadBytes   = 32 << 20
	anchorStateMaxHashedBytes = 64 << 20
)

// CitationState is one envelope citation with its read-time anchor state.
type CitationState struct {
	Anchor      `json:",inline"`
	AnchorState string `json:"anchor_state"`
}

// inputDrift is a declared input whose live content differs from its pinned
// blob: the checkout it is read from, and the committed live blob when clean.
type inputDrift struct {
	dir, oid, objectFormat string
	dirty, exists          bool
}

// anchorDrift is what a live freshness overlay observed per declared input:
// every pinned blob, and the inputs whose live content differs.
type anchorDrift struct {
	pinned  map[string]string
	changed map[string]inputDrift
}

// anchorStates places cited spans in the live checkouts for one request.
type anchorStates struct {
	drift        *anchorDrift
	states       map[string]string
	files        map[string][]byte
	read, hashed int
}

func (s *anchorStates) state(anchor Anchor) string {
	if s == nil || s.drift == nil {
		return AnchorStateUnknown
	}
	key := anchorKey(anchor) + ":" + anchor.Blob + ":" + strconv.Itoa(anchor.Start) + ":" + strconv.Itoa(anchor.End)
	if state, ok := s.states[key]; ok {
		return state
	}
	state := s.resolve(anchor)
	s.states[key] = state
	return state
}

func (s *anchorStates) resolve(anchor Anchor) string {
	input := inputKey(anchor.Revision, anchor.Path)
	blob, declared := s.drift.pinned[input]
	if !declared || blob != anchor.Blob || anchor.Start < 1 || anchor.End < anchor.Start || anchor.SpanSHA256 == "" {
		return AnchorStateUnknown
	}
	drift, changed := s.drift.changed[input]
	if !changed {
		return AnchorStateLive
	}
	if !drift.dirty && !drift.exists {
		return AnchorStateGone
	}
	data, cached := s.files[input]
	if !cached {
		var state string
		if data, state = s.readLive(drift, anchor.Path); state != "" {
			return state
		}
		s.files[input] = data
	}
	return s.locate(data, anchor)
}

// readLive reads the live content of a changed input inside its checkout. A
// clean path must still hash to the committed live blob.
func (s *anchorStates) readLive(drift inputDrift, path string) ([]byte, string) {
	root, err := os.OpenRoot(drift.dir)
	if err != nil {
		return nil, AnchorStateUnknown
	}
	defer root.Close()
	info, err := root.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) && drift.dirty {
		return nil, AnchorStateGone
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > anchorStateMaxFileBytes || s.read+int(info.Size()) > anchorStateMaxReadBytes {
		return nil, AnchorStateUnknown
	}
	data, err := root.ReadFile(path)
	if err != nil || len(data) > anchorStateMaxFileBytes || !drift.dirty && liveBlobID(data, drift.objectFormat) != drift.oid {
		return nil, AnchorStateUnknown
	}
	s.read += len(data)
	return data, ""
}

// locate reports live when the cited lines still hold the pinned span, moved
// when the span is intact at other lines, and changed when it is absent.
func (s *anchorStates) locate(data []byte, anchor Anchor) string {
	offsets := []int{0}
	for i, b := range data {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	if offsets[len(offsets)-1] != len(data) {
		offsets = append(offsets, len(data))
	}
	lines, width := len(offsets)-1, anchor.End-anchor.Start+1
	window := func(first int) string {
		s.hashed += offsets[first+width] - offsets[first]
		return Digest(data[offsets[first]:offsets[first+width]])
	}
	if anchor.End <= lines && window(anchor.Start-1) == anchor.SpanSHA256 {
		return AnchorStateLive
	}
	for first := 0; first+width <= lines; first++ {
		if first == anchor.Start-1 {
			continue
		}
		if s.hashed > anchorStateMaxHashedBytes {
			return AnchorStateUnknown
		}
		if window(first) == anchor.SpanSHA256 {
			return AnchorStateMoved
		}
	}
	return AnchorStateChanged
}

// citationStates pairs each citation with its state; a receipt that cites a
// moved, changed or gone span is stale even when no other input changed.
func citationStates(citations []Anchor, s *anchorStates) ([]CitationState, bool) {
	out := make([]CitationState, 0, len(citations))
	drifted := false
	for _, anchor := range citations {
		state := s.state(anchor)
		drifted = drifted || slices.Contains([]string{AnchorStateMoved, AnchorStateChanged, AnchorStateGone}, state)
		out = append(out, CitationState{Anchor: anchor, AnchorState: state})
	}
	return out, drifted
}

func liveBlobID(data []byte, objectFormat string) string {
	header := []byte("blob " + strconv.Itoa(len(data)) + "\x00")
	if objectFormat == "sha256" {
		sum := sha256.Sum256(append(header, data...))
		return hex.EncodeToString(sum[:])
	}
	sum := sha1.Sum(append(header, data...))
	return hex.EncodeToString(sum[:])
}

// ValidGapKinds bounds a docs_gaps kind filter: nil (absent) or 1 to
// MaxGapKinds distinct non-empty kinds of at most 128 bytes (DCP-V1-284).
func ValidGapKinds(kinds []string) bool {
	if kinds == nil {
		return true
	}
	if len(kinds) == 0 || len(kinds) > MaxGapKinds {
		return false
	}
	for i, k := range kinds {
		if k == "" || len(k) > 128 || slices.Contains(kinds[:i], k) {
			return false
		}
	}
	return true
}
