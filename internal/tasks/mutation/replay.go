package mutation

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
)

// IndexEntry is one recorded request: the request ID, the SHA-256 of the
// canonical mutation bytes and the outcome recorded with it (TM-V0-006).
type IndexEntry struct {
	RequestID      string
	MutationSha256 wire.Digest
	Outcome        Outcome
}

// RequestIndex answers whether a request ID has already been recorded. The
// durable implementation is the journal's requests/ index (TCP-02); this
// package only consults whatever it is given. Only false,nil proves absence;
// a lookup error refuses before a fresh plan is computed.
type RequestIndex interface {
	Lookup(requestID string) (IndexEntry, bool, error)
}

// MemoryIndex is the explicit in-memory fake used by TCP-01 tests to exercise
// the TM-V0-006 replay and conflict rules (AS-03). It is never durable, never
// shared and never evidence of a commit: a test that records an entry here
// is simulating the transaction the journal would perform, nothing more.
type MemoryIndex struct {
	entries map[string]IndexEntry
}

// NewMemoryIndex returns an empty fake index.
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{entries: map[string]IndexEntry{}}
}

// Lookup implements RequestIndex.
func (m *MemoryIndex) Lookup(requestID string) (IndexEntry, bool, error) {
	if m == nil || m.entries == nil {
		return IndexEntry{}, false, wire.Errorf(wire.CodeMalformed, "/requests", "missing memory index")
	}
	e, ok := m.entries[requestID]
	return e, ok, nil
}

// Record stores an entry as the journal would in the same transaction as
// its receipt. A second record under the same request ID is refused: the
// index is append-only.
func (m *MemoryIndex) Record(e IndexEntry) error {
	if m.entries == nil {
		m.entries = map[string]IndexEntry{}
	}
	if _, dup := m.entries[e.RequestID]; dup {
		return wire.Errorf(wire.CodeRequestIDConflict, "/requestId", "request %q is already recorded", e.RequestID)
	}
	if e.Outcome.Codes != nil {
		e.Outcome.Codes = append([]string(nil), e.Outcome.Codes...)
	}
	m.entries[e.RequestID] = e
	return nil
}

// Len returns the number of recorded requests.
func (m *MemoryIndex) Len() int { return len(m.entries) }

// replay applies TM-V0-006 to one request: a recorded entry with the same
// digest returns the original outcome with replayed:true; the same request
// ID with different bytes is REQUEST_ID_CONFLICT; an unknown request ID
// returns nil and the caller proceeds. A nil index is never consulted here:
// Context.checkInputs refuses it before any replay question is asked, so
// this guard only documents that "absent" is not "empty".
func replay(index RequestIndex, requestID string, digest wire.Digest) *Outcome {
	if missingIndex(index) {
		return &Outcome{RequestID: requestID, Outcome: OutcomeValidationFailed, Codes: []string{wire.CodeMalformed}}
	}
	e, ok, err := index.Lookup(requestID)
	if err != nil {
		return &Outcome{RequestID: requestID, Outcome: OutcomeStorageFailed, Codes: []string{wire.CodeOf(err)}}
	}
	if !ok {
		return nil
	}
	if e.MutationSha256 == digest {
		out := e.Outcome
		out.Replayed = true
		out.Codes = append([]string(nil), e.Outcome.Codes...)
		return &out
	}
	return &Outcome{RequestID: requestID, Outcome: OutcomeRequestIDConflict, Codes: []string{wire.CodeRequestIDConflict}}
}

// A nil interface and any typed nil implementation both mean absent authority.
func missingIndex(index RequestIndex) bool {
	if index == nil {
		return true
	}
	v := reflect.ValueOf(index)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
