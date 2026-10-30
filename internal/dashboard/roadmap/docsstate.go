package roadmap

import (
	"encoding/json"
	"os"
)

// docsStateEntry is one generated-doc page's recorded state: its path, the
// sha256 of its page bytes, and its last docsbridge consume state (READY or
// SOURCE_REDERIVED — internal/mcp/docsbridge, docs/specs/
// source-documentation-draft-v0.md).
type docsStateEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	State  string `json:"state"`
}

// readDocsState opens path without blocking and reads it only as a regular
// file of at most maxInputFileBytes.
func readDocsState(path string) ([]byte, error) {
	file, err := os.OpenFile(path, inputOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readRegularBounded(file)
}

// LoadDocState reads localID's entry out of the configured docs-state JSON
// file (a map keyed by ticket local id, e.g. "IPR-10"). No repository
// convention for this file exists yet; it is a proposed minimal contract
// (see the IPR-10 dashboard evidence note). An absent path, unreadable,
// non-regular or oversize file, malformed JSON, or missing entry reports NOT_OBSERVED with a
// reason rather than an invented ready state.
func LoadDocState(path, localID string) DocState {
	if path == "" {
		return DocState{State: notObserved, Reason: "no docs-state file configured"}
	}
	data, err := readDocsState(path)
	if err != nil {
		return DocState{State: notObserved, Reason: "docs-state file unreadable"}
	}
	var all map[string]docsStateEntry
	if err := json.Unmarshal(data, &all); err != nil {
		return DocState{State: notObserved, Reason: "docs-state file did not parse as JSON"}
	}
	entry, ok := all[localID]
	if !ok {
		return DocState{State: notObserved, Reason: "no docs-state entry for " + localID}
	}
	if entry.State == "" {
		return DocState{Path: entry.Path, SHA256: entry.SHA256, State: notObserved, Reason: "docs-state entry has no state"}
	}
	return DocState{Path: entry.Path, SHA256: entry.SHA256, State: entry.State}
}
