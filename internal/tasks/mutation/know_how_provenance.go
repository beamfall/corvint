package mutation

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// AttemptProvenance is one journal-audited attempt record as the know-how
// provenance check sees it (KHN-V0-008): its home ticket, its current
// generation, every prior generation it records, whether it is live, and
// the binding holding its unexpired lease, empty when unleased or expired
// (KHN-V0-022).
type AttemptProvenance struct {
	TicketID   wire.TicketID
	Generation wire.Size
	Prior      []wire.Size
	Live       bool
	Holder     string
}

// AttemptLedger maps an attempt ID to its audited provenance. A nil ledger
// means the attempt inventory was not observed; the check then refuses.
type AttemptLedger map[string]AttemptProvenance

// KnowHowNamesAttempt reports a KNOWHOW_ADD or KNOWHOW_RECONFIRM envelope, or
// an OBLIGATIONS_WITNESS envelope (TOL-V0-015), that names an attempt or a
// generation, so the writer must audit the
// complete attempt inventory and hand the mutation an AttemptLedger
// (KHN-V0-008, KHN-V0-018).
func KnowHowNamesAttempt(env *Envelope) bool {
	if env == nil {
		return false
	}
	switch p := env.Payload.(type) {
	case *KnowHowAddPayload:
		return env.Operation == OpKnowHowAdd && (p.Attempt != nil || p.Generation != nil)
	case *KnowHowReconfirmPayload:
		return env.Operation == OpKnowHowReconfirm && (p.Attempt != nil || p.Generation != nil)
	}
	return ObligationsNameAttempt(env)
}

// CheckAttemptProvenance is CheckKnowHowProvenance under a ledger-neutral
// name, so the TOL-V0-015 obligation witness reuses the one check without
// naming the know-how ledger (KHN-V0-017 reader set).
func CheckAttemptProvenance(ledger AttemptLedger, home wire.TicketID, attempt *string, generation *wire.Size) error {
	return CheckKnowHowProvenance(ledger, home, attempt, generation)
}

// AttemptLedgerOf returns the context's audited attempt ledger.
func (c Context) AttemptLedgerOf() AttemptLedger { return c.KnowHowAttempts }

// WithAttemptLedger returns c with its audited attempt ledger set.
func (c Context) WithAttemptLedger(l AttemptLedger) Context {
	c.KnowHowAttempts = l
	return c
}

// CheckKnowHowProvenance is the one reusable attempt and generation check
// for a know-how write (KHN-V0-008). Both absent passes. Otherwise the
// attempt must be in the observed ledger with home ticket home, and a
// generation must be the attempt's current or a recorded prior generation;
// a generation without an attempt, an unobserved (nil) ledger, an unknown
// attempt, another ticket's attempt or an unrecorded generation refuses
// PROVENANCE_UNVERIFIED. The error never repeats the asserted values.
// Liveness is not required here; a caller that needs a live attempt checks
// ledger[*attempt].Live after this check passes.
func CheckKnowHowProvenance(ledger AttemptLedger, home wire.TicketID, attempt *string, generation *wire.Size) error {
	if attempt == nil && generation == nil {
		return nil
	}
	if attempt == nil {
		return wire.Errorf(wire.CodeProvenanceUnverified, "/payload/generation", "a know-how generation requires the attempt it belongs to")
	}
	if ledger == nil {
		return wire.Errorf(wire.CodeProvenanceUnverified, "/payload/attempt", "the attempt inventory was not observed; the asserted attempt cannot be verified")
	}
	a, ok := ledger[*attempt]
	if !ok {
		return wire.Errorf(wire.CodeProvenanceUnverified, "/payload/attempt", "the asserted attempt is not in the audited attempt inventory")
	}
	if a.TicketID.Raw != home.Raw {
		return wire.Errorf(wire.CodeProvenanceUnverified, "/payload/attempt", "the asserted attempt belongs to another ticket")
	}
	if generation == nil || *generation == a.Generation {
		return nil
	}
	for _, g := range a.Prior {
		if *generation == g {
			return nil
		}
	}
	return wire.Errorf(wire.CodeProvenanceUnverified, "/payload/generation", "the asserted generation is not a recorded generation of the attempt")
}
