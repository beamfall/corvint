package transaction

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// verifyLaneUntouched is deliberately stricter than compatible handoff. It
// checks native logical evidence only; external use remains the operator's claim.
func (c leaseContext) verifyLaneUntouched(a *snapshot.Attempt) error {
	missing := func(detail string) error {
		return wire.Errorf(wire.CodeMissingEvidence, "lane-untouched", "%s", detail)
	}
	if c.r.Actor.Role != "OWNER" && c.r.Actor.Role != "OPERATOR" {
		return wire.Errorf(wire.CodeMissingEvidence, "lane-untouched", "OWNER or OPERATOR required")
	}
	rec, _ := c.st.tickets.Get(a.TicketID.Raw)
	if rec == nil || rec.AcceptanceRevision != a.TicketRevision {
		return wire.Errorf(wire.CodeStaleTicket, "lane-untouched", "acceptance changed")
	}
	if a.CapabilityProfileSha256 != a.PolicySha256 || a.ConfigSha256 != a.PolicySha256 || a.PolicySha256 != wire.Sum(c.st.policy.Raw) {
		return wire.Errorf(wire.CodeStalePolicy, "lane-untouched", "exact current policy/config required")
	}
	if a.RuntimeID != snapshot.RuntimeExternalAgent || a.Phase != "RUNNING" || a.Quiescence != "UNPROVED" || a.Supervisor != nil || a.Supervision != nil || a.Lane != nil || a.WorktreePath != nil || a.NoExec != nil || a.SpawnNoExecCount.Int() != 0 || a.CandidateTreeOid != nil || a.ManifestSha256 != nil || len(a.GateResults)+len(a.Reviews)+len(a.PendingEffects) != 0 || a.ScopeCheck != "UNKNOWN" || a.RetryAccounting == nil || a.RetryAccounting.FailedOrUnknown || a.RetryAccounting.Disposition != "NONE" || a.HandoffEvidence != "" || a.LaneUntouchedAttestation != nil || a.RetryCount.Int() != 0 || a.RepairRound.Int() != 0 || len(a.PriorGenerations) != 0 {
		return missing("known or uncertain generation use")
	}
	origin := a.DirectPoolAdmission
	if origin == nil || a.PoolAllocation == nil || a.Lease == nil || origin.Allocation == nil || origin.AttemptID != a.AttemptID || origin.Generation != a.Generation || origin.Holder != a.Lease.Holder || origin.Stage != a.Stage || !sameAllocation(origin.Allocation, a.PoolAllocation) || origin.OriginalAdmissionSeq.Uint64() == 0 || origin.OriginalAdmissionSeq != a.PoolAllocation.AllocatedSeq || origin.OriginalAdmissionSeq != a.Lease.GrantedSeq || origin.OriginalAdmissionSeq != a.PhaseSinceSeq {
		return missing("fresh direct origin missing, renewed or changed")
	}
	if c.fenced(a) != "" {
		return missing("generation fenced")
	}
	pool := c.st.policy.Pool(a.PoolAllocation.PoolID)
	if pool == nil || c.st.policy.MemberDefinition(pool.ID, a.PoolAllocation.MemberID) != a.PoolAllocation.DefinitionSha256 || pool.MemberConfig[a.PoolAllocation.MemberID].Health != nil {
		return missing("direct no-health member definition changed")
	}
	found := false
	for _, en := range c.st.pools.Entries {
		if en.AllocationID != a.PoolAllocation.AllocationID {
			continue
		}
		if found || en.State != "ALLOCATED" || en.AttemptID != a.AttemptID || en.Generation != a.Generation || en.Holder != a.Lease.Holder || en.Stage != a.Stage || !sameAllocation(&en.PoolAllocation, a.PoolAllocation) || en.ChangedSeq != origin.OriginalAdmissionSeq || en.PolicySha256 != a.PolicySha256 || en.RunnerPID.Int() != 0 || en.RunnerStarted != "" || en.CommandKind != "" || en.CommandRevision != "" || en.ObservationSha256 != nil || en.CleanupPassed || en.Reason != "" {
			return missing("occupancy has changed or recorded command use")
		}
		found = true
	}
	if !found {
		return missing("exact occupancy absent")
	}
	return c.untouchedPrograms(a)
}

// Absence is meaningful only in the audited inventory of this same input.
// No private Dispatcher files are read. Even pre-ATTACH ADMITTED state counts.
func (c leaseContext) untouchedPrograms(a *snapshot.Attempt) error {
	fail := func() error {
		return wire.Errorf(wire.CodeMissingEvidence, "programs", "missing, unbound, malformed or associated native program evidence")
	}
	_, present := c.in.Inventory.files["programs.json"]
	if len(c.in.Programs) == 0 {
		if present {
			return fail()
		}
		return nil
	}
	if !present || !c.in.Inventory.matches("programs.json", c.in.Programs) {
		return fail()
	}
	p, err := snapshot.DecodePrograms(c.in.Programs)
	if err != nil || p.QueueID != c.r.QueueID {
		return fail()
	}
	raw, err := p.Encode()
	if err != nil || !bytes.Equal(raw, c.in.Programs) {
		return fail()
	}
	for _, x := range p.Entries {
		// Same-attempt missing or different generation is uncertain, never absence.
		if x.CurrentAttempt == a.AttemptID || x.CurrentGeneration == string(a.Generation) {
			return fail()
		}
		if (x.CurrentAttempt == "") != (x.CurrentGeneration == "") {
			return fail()
		}
		if x.CurrentAttempt != "" {
			q, err := snapshot.AttemptQueue(x.CurrentAttempt)
			if err != nil || q.Raw != c.r.QueueID {
				return fail()
			}
			if _, err := wire.ParseSize("program generation", x.CurrentGeneration); err != nil {
				return fail()
			}
		}
	}
	return nil
}
