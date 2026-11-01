package journal

import (
	"maps"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealedCapture is a complete walk's state as it passed the sealed sequence
// S of a segmented store: the latest entries and operator-note references at
// S, and the chain position there (JSG-V0-780). Both maps are copies, since
// the walk replaces their entries as it continues.
type sealedCapture struct {
	seq        wire.Size
	last       wire.Digest
	generation uint64
	canonical  map[string]latest
	notes      *noteState
}

// captureSealed snapshots st and result right after the walk stepped the
// receipt at the sealed sequence.
func (st *chain) captureSealed(result *Result) *sealedCapture {
	c := &sealedCapture{seq: result.LastSeq, last: result.LastReceiptSha256, generation: st.generation, canonical: maps.Clone(st.canonical)}
	if st.notes != nil {
		c.notes = &noteState{refs: maps.Clone(st.notes.refs), policyWalked: st.notes.policyWalked}
	}
	return c
}

// SealedWriterCheckpoint is the writer checkpoint a complete audit retains
// (JSG-V0-780). On a segmented store whose head is past the sealed sequence S
// it is WriterCheckpoint rebased, under JSG-V0-009's rule, on the checkpoint
// derived from the walk's state at S, so FullSeq is S and the base is at S, as
// a validated writer seal leaves it; the sealed resume of widen, reap and the
// pool verbs can then start from it. Any other result, and any derivation or
// rebase that fails, returns WriterCheckpoint unchanged: the base is derived
// state, and its absence only costs a later write the complete audit.
func (res *Result) SealedWriterCheckpoint(physical map[string]PhysicalFile) (*WriterCheckpoint, error) {
	w, err := res.WriterCheckpoint(physical)
	if err != nil || w == nil {
		return w, err
	}
	if nb := res.sealedBaseCheckpoint(physical); nb != nil {
		if rebased, err := rebase(w, nb); err == nil {
			return rebased, nil
		}
	}
	return w, nil
}

// sealedBaseCheckpoint derives the base-free checkpoint at the sealed sequence
// from the walk's capture there, or nil when the result has none.
func (res *Result) sealedBaseCheckpoint(physical map[string]PhysicalFile) *WriterCheckpoint {
	if res == nil || res.Mode != ModeFull || res.chain == nil || res.chain.atSealed == nil || res.Head == nil || res.Head.Segments == nil || res.manifest == nil {
		return nil
	}
	c := res.chain.atSealed
	if res.manifest.Through != c.seq.Uint64() || res.Head.Segments.Through != c.seq {
		return nil
	}
	at := *res.Head
	at.LastSeq, at.Generation = c.seq, wire.SizeOf(c.generation)
	last := c.last
	at.LastReceiptSha256 = &last
	tmp := &Result{Head: &at, Mode: ModeFull, StructuralConsistency: res.StructuralConsistency, SemanticCoverage: res.SemanticCoverage,
		LastSeq: c.seq, LastReceiptSha256: c.last, manifest: res.manifest, chain: &chain{canonical: c.canonical, notes: c.notes}}
	nb, err := tmp.WriterCheckpoint(physical)
	if err != nil {
		return nil
	}
	return nb
}
