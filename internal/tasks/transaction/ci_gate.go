package transaction

import (
	"bytes"
	"slices"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CIRunRefusal checks a retained CI run against the policy's ciEvidence
// entry for gateID (TCD-V0-154, TCD-V0-156): the entry and a COMMAND gate
// with an expected exit code exist, the run is of the declared repository,
// it concluded success and every required check passed in it. It returns
// the refusal code and detail, or "" when the run qualifies.
func CIRunRefusal(p *intent.Policy, gateID string, ci *snapshot.CIRun) (string, string) {
	entry, def := p.CIEvidenceGate(gateID), p.Gate(gateID)
	switch {
	case entry == nil || def == nil || def.Kind != "COMMAND" || def.ExpectedExit == nil:
		return wire.CodeUnsupported, "the policy does not admit CI evidence for gate " + gateID
	case ci.Repository != entry.Repository:
		return wire.CodeGateStale, "the CI run is of " + ci.Repository + ", not " + entry.Repository
	case ci.Conclusion != "success":
		return wire.CodeGateFailed, "CI run " + string(ci.RunID) + " concluded " + ci.Conclusion
	}
	for _, check := range entry.RequiredChecks {
		if !slices.Contains(ci.PassedChecks, check) {
			return wire.CodeGateFailed, "required check " + check + " did not pass in CI run " + string(ci.RunID)
		}
	}
	return "", ""
}

// ciEnvironment is the environment digest of a CI-backed result: the empty
// local environment, since nothing ran locally.
var ciEnvironment = wire.Sum(wire.EncodeFile(wire.Array()))

// CIGateResult is the record a CI run (raw, decoded as ci) writes for gate
// gateID of attempt a, whose head commit carries headTree (TCD-V0-155): a
// PASSED result at headTree bound to a, with the current definition digest,
// the run record as its inputs and only evidence, no local environment,
// the run's completion time and reusedFrom naming the record. The store
// builds it, and the model and the receipt audit require the posted record
// to be exactly these bytes.
func CIGateResult(p *intent.Policy, gateID string, ci *snapshot.CIRun, raw []byte, a *snapshot.Attempt, headTree string) snapshot.GateResult {
	from := wire.Sum(raw)
	clean, cwd, tree := true, "WORKTREE", headTree
	var exit *wire.Count
	if def := p.Gate(gateID); def != nil {
		exit = def.ExpectedExit
	}
	return snapshot.GateResult{
		GateID: gateID, AttemptID: a.AttemptID, Generation: a.Generation, TicketRevision: a.TicketRevision,
		DefinitionSha256: p.GateDefinitionSha256(gateID), CandidateTreeOid: headTree, ExecutedTreeOid: &tree,
		ExecutedCwd: &cwd, PorcelainClean: &clean, InputsSha256: from, EnvironmentSha256: ciEnvironment,
		PolicySha256: a.PolicySha256, ConfigSha256: a.ConfigSha256, StartedAt: ci.CompletedAt, EndedAt: ci.CompletedAt,
		State: "PASSED", OutcomeClass: "EXIT", ExitCode: exit,
		Evidence:   []snapshot.GateEvidence{{Label: "ci-run", Sha256: from, Bytes: wire.SizeOf(uint64(len(raw)))}},
		ReusedFrom: &from,
	}
}

// GateCIFacts builds the facts of a gate run satisfied by a retained CI run
// (TCD-V0-155): the CI-backed record, the run record, the tree the store
// resolved for the run's head commit and its ancestry verdict, "" when that
// commit is on the protected intent branch carrying headTree and descends
// from a commit carrying the candidate tree.
func GateCIFacts(record, ciRun []byte, headTree, descent string, results map[wire.Digest][]byte) LeaseFacts {
	f := GateReuseFacts(record, ciRun, results)
	f.CIHeadTree = headTree
	f.ReuseDescends, f.ReuseDescent = descent == "" && headTree != "", descent
	return f
}

// ciRefusal checks a CI-backed gate result under the lock (TCD-V0-155,
// TCD-V0-156). The run record may be new to the store; if it is already
// retained the bytes must match. The run must qualify under the current
// policy, the record must be exactly CIGateResult for the resolved head
// tree, and the head commit must descend from the candidate on the
// protected intent branch even when its tree is the candidate's.
func (c leaseContext) ciRefusal(a *snapshot.Attempt, g *snapshot.GateResult, ci *snapshot.CIRun) *leaseOutcome {
	f := c.in.LeaseFacts
	path := "evidence/" + string(*g.ReusedFrom)
	if _, ok := c.in.Inventory.lookup(path); ok && !c.in.Inventory.matches(path, f.ReuseSource) {
		out := c.fail(malformed("CI run record " + string(*g.ReusedFrom) + " differs from its evidence file"))
		return &out
	}
	refuse := func(code, detail string) *leaseOutcome {
		out := c.refuse(mutation.OutcomeBlocked, code, detail)
		return &out
	}
	if code, detail := CIRunRefusal(c.st.policy, g.GateID, ci); code != "" {
		return refuse(code, detail)
	}
	if f.CIHeadTree == "" {
		return refuse(wire.CodeStaleTree, "the CI run's head commit "+ci.HeadSha+" is not in the local repository")
	}
	want := CIGateResult(c.st.policy, g.GateID, ci, f.ReuseSource, a, f.CIHeadTree)
	raw, e := want.Encode()
	if e != nil || !bytes.Equal(raw, f.GateRecord) {
		out := c.fail(malformed("CI-backed gate result is not the CI run bound to this attempt"))
		return &out
	}
	if !f.ReuseDescends {
		detail := f.ReuseDescent
		if detail == "" {
			detail = "the CI run's head commit " + ci.HeadSha + " was not observed on the protected intent branch"
		}
		return refuse(wire.CodeStaleTree, detail)
	}
	return nil
}
