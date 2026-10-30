package gitauth

import (
	"context"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	memoMaxEntries = 256
	memoMaxPayload = 16 << 20
)

type memoOperation uint8

const (
	memoResolve memoOperation = iota
	memoLookup
	memoBlob
	memoDiff
)

type memoKey struct {
	operation     memoOperation
	first, second string
}

type memoValue struct {
	oid    string
	entry  TreeEntry
	exists bool
	data   []byte
}

type memoRecord struct {
	key   memoKey
	value memoValue
	bytes int
}

// RequestReadMemo shares successful immutable primitive values within one
// sequential request. It is not an authority-admission API. The protected
// consumer constructs it only after independently auditing its object view.
// Ordinary repositories and live currentness readers do not use this memo.
// Reader budgets and distinct-blob accounting remain independent.
type RequestReadMemo struct {
	root, gitDir, commonDir, format string
	closed                          bool
	entries                         map[memoKey]*memoRecord
	fifo                            [memoMaxEntries]*memoRecord
	head, count, payload            int
}

// NewRequestReadMemo captures a previously audited immutable view. This
// constructor does not perform, substitute for, or attest that external audit.
func NewRequestReadMemo(view *Repository) (*RequestReadMemo, error) {
	if view == nil || view.objectView != nil || view.requestMemo != nil ||
		(view.ObjectFormat != "sha1" && view.ObjectFormat != "sha256") ||
		view.Root == "" || view.GitDir == "" || view.CommonDir == "" {
		return nil, unavailable("invalid request memo view")
	}
	return &RequestReadMemo{root: view.Root, gitDir: view.GitDir, commonDir: view.CommonDir,
		format: view.ObjectFormat, entries: make(map[memoKey]*memoRecord)}, nil
}

// Open preserves the ordinary repository boundary checks but takes no caller
// root. Each caller supplies its own verification budget, started at the same
// point as an ordinary Open. No mutable reader accounting is shared.
func (m *RequestReadMemo) Open(budget *gitrun.Budget) (*Repository, error) {
	if m == nil || m.closed {
		return nil, unavailable("request memo is closed")
	}
	r, err := Open(m.root, budget)
	if err != nil {
		return nil, err
	}
	if err := m.matches(r); err != nil {
		return nil, err
	}
	r.requestMemo = m
	return r, nil
}

// Release drops every retained payload and closes the session. Already-open
// readers subsequently bypass both lookup and insertion; they cannot refill it.
func (m *RequestReadMemo) Release() {
	if m == nil {
		return
	}
	m.closed = true
	m.entries = nil
	m.fifo = [memoMaxEntries]*memoRecord{}
	m.head, m.count, m.payload = 0, 0, 0
}

func (m *RequestReadMemo) matches(r *Repository) error {
	if r.Root != m.root || r.GitDir != m.gitDir || r.CommonDir != m.commonDir ||
		r.objectView != nil || (r.ObjectFormat != "" && r.ObjectFormat != m.format) {
		return unavailable("request memo repository identity changed")
	}
	return nil
}

func (r *Repository) requestKey(operation memoOperation, first, second string) (memoKey, bool, error) {
	key := memoKey{operation, first, second}
	m := r.requestMemo
	if m == nil || m.closed {
		return key, false, nil
	}
	if err := m.matches(r); err != nil {
		return key, false, err
	}
	width := 40
	if m.format == "sha256" {
		width = 64
	}
	if len(first) != width || !wire.IsGitOid(first) ||
		(operation == memoDiff && (len(second) != width || !wire.IsGitOid(second))) {
		return key, false, nil
	}
	return key, true, nil
}

func memoProgress(ctx context.Context, deadline time.Time) error {
	if ctx.Err() != nil {
		return cemcode.New(cemcode.GitCancelled, "Git operation cancelled")
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return cemcode.New(cemcode.GitTimeout, "Git operation timed out")
	}
	return nil
}

func copyMemoBytes(ctx context.Context, deadline time.Time, raw []byte) ([]byte, error) {
	if err := memoProgress(ctx, deadline); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	out := make([]byte, len(raw))
	for start := 0; start < len(raw); start += 64 << 10 {
		if err := memoProgress(ctx, deadline); err != nil {
			return nil, err
		}
		copy(out[start:min(start+(64<<10), len(raw))], raw[start:min(start+(64<<10), len(raw))])
	}
	if err := memoProgress(ctx, deadline); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) recalled(ctx context.Context, key memoKey) (memoValue, bool, error) {
	m := r.requestMemo
	if m == nil || m.closed {
		return memoValue{}, false, nil
	}
	if err := m.matches(r); err != nil {
		return memoValue{}, false, err
	}
	record, ok := m.entries[key]
	if !ok {
		return memoValue{}, false, nil
	}
	span, err := r.budget.ReserveOperation(0)
	if err != nil {
		return memoValue{}, true, err
	}
	value := record.value
	value.data, err = copyMemoBytes(ctx, time.Now().Add(span), value.data)
	return value, true, err
}

func (r *Repository) remember(ctx context.Context, key memoKey, value memoValue) error {
	m := r.requestMemo
	if m == nil || m.closed {
		return nil
	}
	if err := m.matches(r); err != nil {
		return err
	}
	size := len(key.first) + len(key.second) + len(value.oid) + len(value.entry.Mode) +
		len(value.entry.Type) + len(value.entry.OID) + len(value.data)
	if size > memoMaxPayload {
		return nil
	}
	if _, exists := m.entries[key]; exists {
		return nil
	}
	data, err := copyMemoBytes(ctx, time.Time{}, value.data)
	if err != nil {
		return err
	}
	key.first, key.second = strings.Clone(key.first), strings.Clone(key.second)
	value.oid = strings.Clone(value.oid)
	value.entry = TreeEntry{strings.Clone(value.entry.Mode), strings.Clone(value.entry.Type), strings.Clone(value.entry.OID)}
	value.data = data
	for m.count >= memoMaxEntries || m.payload+size > memoMaxPayload {
		old := m.fifo[m.head]
		delete(m.entries, old.key)
		m.fifo[m.head] = nil
		m.head = (m.head + 1) % memoMaxEntries
		m.count--
		m.payload -= old.bytes
	}
	record := &memoRecord{key, value, size}
	m.fifo[(m.head+m.count)%memoMaxEntries] = record
	m.entries[key] = record
	m.count++
	m.payload += size
	return nil
}
