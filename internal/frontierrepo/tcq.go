package frontierrepo

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/tcq"
)

const maxTCQResultBytes = 4 << 20 // TCQ-V0-041 result-verification ceiling.

// Recompute is cascade stage 7. CF-V0-002 forbids accepting a caller-supplied
// TCQ result as authority or as a shortcut, so this runs the real producer
// over the verified inputs and the target Git objects and then binds the
// recomputed identity — in both static and dynamic mode (CF-V0-003).
func (adapter *Adapter) Recompute(request frontier.TCQRequest) (frontier.TCQResult, error) {
	adapter.pythonClaims = nil
	repository, err := adapter.tree()
	if err != nil {
		return frontier.TCQResult{}, translate(err)
	}
	result, err := tcq.Evaluate(repository, adapter, tcq.Request{
		CEM:          request.CEMBytes,
		OCM:          request.OCMBytes,
		ExpectedBase: request.BaseRevision,
		Target:       request.TargetRevision,
		Command:      request.Command,
		Observation:  request.Observation,
		Report:       request.JUnitReport,
	})
	if err != nil {
		return frontier.TCQResult{}, translate(err)
	}
	projected, err := applyClaimAbstentions(result, adapter.pythonClaims)
	if err != nil {
		return frontier.TCQResult{}, translate(err)
	}
	return projected, nil
}

// claims carries the producer's per-edge results across the frozen seam field
// for field. Every selected edge crosses it: CF-V0-015 requires each one to be
// evaluated, so a filter here would fabricate a closure.
func claims(results []tcq.ClaimResult) []frontier.TCQClaimResult {
	projected := make([]frontier.TCQClaimResult, 0, len(results))
	for _, result := range results {
		projected = append(projected, frontier.TCQClaimResult{
			ObligationID:   result.ObligationID,
			ClaimID:        result.ClaimID,
			Reasons:        result.Reasons,
			Relation:       result.Relation,
			AuthorityClass: result.AuthorityClass,
		})
	}
	return projected
}

// applyClaimAbstentions turns the source evidence produced by the shared
// verifier into TCQ-V0-047's edge-local result. The TCQ identity is rebound to
// the adjusted claims and issues; returning the old identity beside changed
// claims would make the projection unverifiable. Units are unchanged because
// only an already-abstained claim with no unit can reach this repair.
func applyClaimAbstentions(result tcq.Result, evidence []lrfrepo.PythonClaimEvidence) (frontier.TCQResult, error) {
	byClaim := make(map[string]lrfrepo.PythonClaimEvidence, len(evidence))
	for _, item := range evidence {
		byClaim[item.ClaimID] = item
	}
	adjusted := result.Claims()
	changed := false
	for index, claim := range adjusted {
		item, found := byClaim[claim.ClaimID]
		if !found || !grammarFailureLostProfile(claim) {
			continue
		}
		adjusted[index] = abstainedTCQClaim(claim, item)
		changed = true
	}
	if !changed {
		return frontier.TCQResult{ID: result.ID(), Claims: claims(adjusted)}, nil
	}
	identity, err := rebindTCQIdentity(result.Raw(), adjusted)
	if err != nil {
		return frontier.TCQResult{}, err
	}
	return frontier.TCQResult{ID: identity, Claims: claims(adjusted)}, nil
}

// A Python grammar failure can stop TCQ before it has candidates, so its
// generic association step reports unsupported-anchor-profile first. The
// shared verifier has already re-derived this exact Python anchor profile; only
// that otherwise-impossible combination is rewritten to the grammar reason.
func grammarFailureLostProfile(claim tcq.ClaimResult) bool {
	return claim.AnchorProfile == "" &&
		claim.AssociationState == tcq.AssociationAbstained &&
		len(claim.Reasons) == 1 && claim.Reasons[0] == "unsupported-anchor-profile"
}

func abstainedTCQClaim(claim tcq.ClaimResult, evidence lrfrepo.PythonClaimEvidence) tcq.ClaimResult {
	return tcq.ClaimResult{
		ObligationID:     claim.ObligationID,
		ClaimID:          claim.ClaimID,
		AnchorProfile:    evidence.AnchorProfile,
		AssociationState: tcq.AssociationAbstained,
		HygieneState:     tcq.HygieneAbstained,
		ReportState:      tcq.ReportNotMatched,
		RowIDs:           []string{},
		Reasons:          []string{evidence.Reason},
		AuthorityClass:   tcq.AuthorityClass,
	}
}

func rebindTCQIdentity(raw []byte, claims []tcq.ClaimResult) (string, error) {
	document, err := wire.Parse(raw)
	if err != nil || document.Kind != wire.KindObject {
		return "", &tcq.Error{Code: tcq.CodeInvalidTCQ}
	}
	document.Obj.Values["claims"] = wire.Value{Kind: wire.KindArray, Arr: tcqClaimValues(claims)}
	document.Obj.Values["issues"] = wire.Value{Kind: wire.KindArray, Arr: tcqIssueValues(claims)}
	delete(document.Obj.Values, "id")
	document.Obj.Keys = keysWithout(document.Obj.Keys, "id")
	body := wire.CanonicalValue(document)
	digest := sha256.New()
	digest.Write([]byte("corvint-tcq/0"))
	digest.Write([]byte{0})
	digest.Write(body)
	identity := "tcq:sha256:" + hex.EncodeToString(digest.Sum(nil))
	document.Obj.Keys = append(document.Obj.Keys, "id")
	document.Obj.Values["id"] = tcqString(identity)
	if len(wire.CanonicalValue(document))+1 > maxTCQResultBytes {
		return "", &tcq.Error{Code: tcq.CodeResourceExhausted}
	}
	return identity, nil
}

func tcqClaimValues(claims []tcq.ClaimResult) []wire.Value {
	values := make([]wire.Value, 0, len(claims))
	for _, claim := range claims {
		values = append(values, tcqObject(
			"anchorProfile", tcqNullable(claim.AnchorProfile),
			"associationKind", tcqNullable(claim.AssociationKind),
			"associationState", tcqString(claim.AssociationState),
			"authorityClass", tcqString(claim.AuthorityClass),
			"claimId", tcqString(claim.ClaimID),
			"executionKeySha256", tcqNullable(claim.ExecutionKeySha256),
			"hygieneState", tcqString(claim.HygieneState),
			"obligationId", tcqString(claim.ObligationID),
			"reasons", tcqStrings(claim.Reasons),
			"relation", tcqNullable(claim.Relation),
			"reportState", tcqString(claim.ReportState),
			"rowIds", tcqStrings(claim.RowIDs),
			"testUnitId", tcqNullable(claim.TestUnitID),
		))
	}
	return values
}

func tcqIssueValues(claims []tcq.ClaimResult) []wire.Value {
	var values []wire.Value
	for _, claim := range claims {
		for _, reason := range claim.Reasons {
			values = append(values, tcqObject(
				"claimId", tcqString(claim.ClaimID),
				"obligationId", tcqString(claim.ObligationID),
				"reason", tcqString(reason),
			))
		}
	}
	return values
}

func tcqObject(fields ...any) wire.Value {
	object := &wire.Object{Values: map[string]wire.Value{}}
	for index := 0; index < len(fields); index += 2 {
		key := fields[index].(string)
		object.Keys = append(object.Keys, key)
		object.Values[key] = fields[index+1].(wire.Value)
	}
	return wire.Value{Kind: wire.KindObject, Obj: object}
}

func tcqString(value string) wire.Value { return wire.Value{Kind: wire.KindString, Str: value} }

func tcqNullable(value string) wire.Value {
	if value == "" {
		return wire.Value{Kind: wire.KindNull}
	}
	return tcqString(value)
}

func tcqStrings(values []string) wire.Value {
	items := make([]wire.Value, 0, len(values))
	for _, value := range values {
		items = append(items, tcqString(value))
	}
	return wire.Value{Kind: wire.KindArray, Arr: items}
}

func keysWithout(keys []string, omitted string) []string {
	kept := keys[:0]
	for _, key := range keys {
		if key != omitted {
			kept = append(kept, key)
		}
	}
	return kept
}

// VerifyOCM implements tcq.UpstreamVerifier. TCQ-V0-001 requires the same
// bounded raw byte copies and both revision inputs to pass through the shared
// CEM/OCM verifier, and TCQ-V0-042 forbids reordering anything internal to it.
//
// Within one Frontier invocation this returns the outcome of the stage-5 call
// rather than making a second one, because CF-V0-021 admits exactly ONE shared
// verification call. That is a memo and not a bypass: the hit requires the
// exact same CEM and OCM bytes by digest and both revisions to name the same
// two commits, so bytes the shared call never saw fall through to a real
// verification below. A standalone TCQ caller therefore also gets a real one.
func (adapter *Adapter) VerifyOCM(cemRaw, ocmRaw []byte, expectedBase, target string) (tcq.Resolved, error) {
	if shared := adapter.shared; shared != nil && shared.covers(cemRaw, ocmRaw, expectedBase, target) {
		adapter.pythonClaims = append([]lrfrepo.PythonClaimEvidence(nil), shared.pythonClaims...)
		return shared.resolved, nil
	}
	universe, err := lrfrepo.VerifyUniverse(adapter.ctx, adapter.root, cemRaw, ocmRaw, expectedBase, target)
	if err != nil {
		return tcq.Resolved{}, translate(err)
	}
	adapter.pythonClaims = append([]lrfrepo.PythonClaimEvidence(nil), universe.PythonClaims...)
	return tcq.Resolved{BaseRevision: universe.BaseRevision, TargetRevision: universe.TargetRevision}, nil
}

// covers reports whether the shared call already verified exactly these
// artifacts against exactly these two revisions. A revision argument matches
// when it is the string the shared call was given or the full OID that call
// resolved it to — Frontier hands TCQ the resolved OIDs, not its own inputs.
func (outcome *sharedOutcome) covers(cemRaw, ocmRaw []byte, expectedBase, target string) bool {
	return outcome.cemSHA256 == digest(cemRaw) &&
		outcome.ocmSHA256 == digest(ocmRaw) &&
		namesRevision(expectedBase, outcome.expectedBase, outcome.resolved.BaseRevision) &&
		namesRevision(target, outcome.target, outcome.resolved.TargetRevision)
}

func namesRevision(candidate, requested, resolved string) bool {
	return candidate == requested || candidate == resolved
}

// treeReader is the inherited bounded, sanitized, no-fetch Git boundary TCQ
// reads source through (TCQ-V0-001). It adds nothing: worktree bytes, sibling
// repositories, implicit fetch, and additional-repository discovery are all
// refused by gitauth.Open and by the pinned, environment-scrubbed Git
// invocations underneath it, and this type deliberately widens none of that.
type treeReader struct {
	adapter    *Adapter
	repository *gitauth.Repository
}

func (adapter *Adapter) tree() (tcq.Repository, error) {
	repository, err := gitauth.Open(adapter.root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	return treeReader{adapter: adapter, repository: repository}, nil
}

// Blob returns one target-tree blob by OID through Git object identity.
func (reader treeReader) Blob(oid string) ([]byte, error) {
	return reader.repository.BlobBytes(reader.adapter.ctx, oid)
}

// TreeEntry resolves one normalized repository-relative path at revision. An
// absent path is the empty entry with no error, which is the distinction the
// seam draws: "missing" is an outcome TCQ reasons about, an error is not.
func (reader treeReader) TreeEntry(revision, path string) (tcq.TreeEntry, error) {
	entry, exists, err := reader.repository.LookupTreeEntry(reader.adapter.ctx, revision, path)
	if err != nil {
		return tcq.TreeEntry{}, err
	}
	if !exists {
		return tcq.TreeEntry{}, nil
	}
	return tcq.TreeEntry{Mode: entry.Mode, Type: entry.Type, Found: true}, nil
}
