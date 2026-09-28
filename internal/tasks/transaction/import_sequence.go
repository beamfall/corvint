package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Publication is sealed to transaction-produced immutable bytes. It grants no
// physical authority; store still performs every publication and CAS check.
type Publication interface {
	Descriptor() []byte
	Receipt() []byte
	Head() []byte
	Artifacts() []Artifact
	publication()
}

func (*Plan) publication() {}

// ImportBatch is detached from its sequence's mutable inventory. It cannot be
// used as a generic Plan or claim a complete final inventory.
type ImportBatch struct {
	plan     *Plan
	prefixes []Prefix
}

func (b *ImportBatch) Descriptor() []byte    { return b.plan.Descriptor() }
func (b *ImportBatch) Receipt() []byte       { return b.plan.Receipt() }
func (b *ImportBatch) Head() []byte          { return b.plan.Head() }
func (b *ImportBatch) Artifacts() []Artifact { return b.plan.Artifacts() }
func (*ImportBatch) publication()            {}
func (b *ImportBatch) Prefixes() []Prefix {
	out := append([]Prefix(nil), b.prefixes...)
	for n := range out {
		if out[n].AbortToUnpause != nil {
			c := *out[n].AbortToUnpause
			out[n].AbortToUnpause = &c
		}
	}
	return out
}

// ImportSequence owns one validated, process-local IMPORT_APPLY computation.
// Its store owner must close it on any physical failure/cancellation/session end
// and advance only after the exact pending publication succeeds. No state is
// serialized or authenticated, and Coverage remains NOT_OBSERVED.
type ImportSequence struct {
	input          Input
	state          inputState
	actor          mutation.Binding
	records        map[string]*ticket.Record
	folded         map[string]string
	pending        *ImportBatch
	pendingRecords map[string]*ticket.Record
	prefixes       []Prefix
	closed         bool
}

func NewImportSequence(queueID string, actor mutation.Binding, in Input) (*ImportSequence, error) {
	if in.Inventory == nil {
		return nil, malformed("complete inventory required")
	}
	owned := in
	owned.Inventory = in.Inventory.clone()
	owned.Head = bytes.Clone(in.Head)
	owned.HeadReceipt = bytes.Clone(in.HeadReceipt)
	owned.Queue = bytes.Clone(in.Queue)
	owned.Policy = bytes.Clone(in.Policy)
	owned.Barrier = bytes.Clone(in.Barrier)
	owned.Reservations = bytes.Clone(in.Reservations)
	copyRecords := func(records [][]byte) [][]byte {
		out := make([][]byte, len(records))
		for n, raw := range records {
			out[n] = bytes.Clone(raw)
		}
		return out
	}
	owned.CanonicalTickets = copyRecords(in.CanonicalTickets)
	owned.CanonicalReleases = copyRecords(in.CanonicalReleases)
	st, err := validateInput(Request{Operation: ImportApply, QueueID: queueID}, owned)
	if err != nil {
		return nil, err
	}
	if st.head == nil {
		return nil, malformed("import sequence requires initialized head")
	}
	seq := &ImportSequence{input: owned, state: st, actor: actor, records: map[string]*ticket.Record{}, folded: map[string]string{}}
	for _, id := range st.tickets.IDs() {
		rec, _ := st.tickets.Get(id)
		seq.records[id] = rec
		seq.folded[wire.FoldToken(rec.TicketID.Local)] = id
	}
	cost := &importCost{}
	for _, f := range owned.Inventory.files {
		if err = cost.replace(f, false, f); err != nil {
			return nil, err
		}
	}
	owned.Inventory.incremental = cost
	// Only owned decoded records remain necessary after the one full validation.
	seq.input.CanonicalTickets = nil
	seq.input.CanonicalReleases = nil
	seq.state.tickets = nil
	return seq, nil
}

func (s *ImportSequence) Close() {
	if s != nil {
		s.closed = true
		s.pending = nil
		s.pendingRecords = nil
	}
}
func (s *ImportSequence) Has(path string) bool {
	return s != nil && !s.closed && s.input.Inventory.Has(path)
}

// Prepare refuses further use after any refusal or an outstanding publication.
// The returned Result deliberately has neither a public Plan nor Final inventory.
func (s *ImportSequence) Prepare(r Request, in Input) (Result, *ImportBatch) {
	if s == nil || s.closed {
		return failed(r.RequestID, malformed("closed import sequence")), nil
	}
	if s.pending != nil || r.Operation != ImportApply || r.Actor != s.actor {
		s.Close()
		return failed(r.RequestID, malformed("import sequence binding")), nil
	}
	r = cloneRequest(r)
	in.Inventory = s.input.Inventory
	s.pendingRecords = map[string]*ticket.Record{}
	result := model(r, in, s)
	if result.Kind != "Transaction" || result.Plan == nil {
		s.Close()
		return result, nil
	}
	p := result.Plan
	p.base = nil
	p.baseHead = nil
	batch := &ImportBatch{plan: p, prefixes: s.prefixes}
	result.Plan = nil
	result.Final = nil
	s.pending = batch
	return result, batch
}

func (s *ImportSequence) validate(r Request, in Input) (inputState, error) {
	expected := s.input
	if r.QueueID != s.state.queue.QueueID.Raw || !bytes.Equal(in.Head, expected.Head) || !bytes.Equal(in.HeadReceipt, expected.HeadReceipt) || !bytes.Equal(in.Queue, expected.Queue) || !bytes.Equal(in.Policy, expected.Policy) || !bytes.Equal(in.Barrier, expected.Barrier) || !bytes.Equal(in.Reservations, expected.Reservations) {
		return inputState{}, malformed("import sequence observation changed")
	}
	return s.state, nil
}
func (s *ImportSequence) posts(r Request, posts map[string][]byte) error {
	if s.state.queue.CanonicalWriter == "NATIVE" {
		return wire.Errorf(wire.CodeUnsupported, "canonicalWriter", "import into a NATIVE-written queue would make imported records eligible without a cutover")
	}
	additions := 0
	for _, raw := range r.Records {
		rec, err := ticket.Decode(raw)
		if err != nil {
			return err
		}
		if err = canonical(raw); err != nil {
			return err
		}
		if rec.TicketID.QueueID() != s.state.queue.QueueID.Raw {
			return malformed("import ticket queue")
		}
		folded := wire.FoldToken(rec.TicketID.Local)
		if id, ok := s.folded[folded]; ok && id != rec.TicketID.Raw {
			return wire.Errorf(wire.CodeDuplicateID, "ticketId", "case-folded import collision")
		}
		for id, p := range s.pendingRecords {
			if wire.FoldToken(p.TicketID.Local) == folded && id != rec.TicketID.Raw {
				return wire.Errorf(wire.CodeDuplicateID, "ticketId", "case-folded batch collision")
			}
		}
		pre := s.records[rec.TicketID.Raw]
		if p, ok := s.pendingRecords[rec.TicketID.Raw]; ok {
			pre = p
		}
		if err = importChain(rec, pre, s.input.Inventory); err != nil {
			return err
		}
		if pre == nil {
			additions++
		}
		if len(s.records)+additions > wire.MaxTicketsPerQueue {
			return limit("ticket inventory")
		}
		s.pendingRecords[rec.TicketID.Raw] = rec
		posts["intent/tickets/"+rec.TicketID.Local+".json"] = bytes.Clone(raw)
	}
	return nil
}

// Advance consumes only this sequence's exact pending batch, once. The caller
// attests physical success; the model does not authenticate that assertion.
func (s *ImportSequence) Advance(b *ImportBatch) error {
	if s == nil {
		return malformed("nil import sequence")
	}
	if s.closed || b == nil || s.pending != b {
		s.Close()
		return malformed("wrong or consumed import batch")
	}
	for _, a := range b.plan.artifacts {
		if err := s.input.Inventory.put(entry(a), false); err != nil {
			s.Close()
			return err
		}
	}
	for id, rec := range s.pendingRecords {
		s.records[id] = rec
		s.folded[wire.FoldToken(rec.TicketID.Local)] = id
	}
	s.input.Head = b.Head()
	s.input.HeadReceipt = b.Receipt()
	h, err := snapshot.DecodeHead(s.input.Head)
	if err != nil {
		s.Close()
		return err
	}
	s.state.head = h
	s.pending = nil
	s.pendingRecords = nil
	return nil
}
