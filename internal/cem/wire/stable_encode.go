package wire

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// StableHunks returns the complete JSON value without dropping optional witnesses.
func (d *StableMap) StableHunks() []any { change := Map(d.Change); return stableHunks(&change) }
func (d *StableMap) StableReferences() map[string]any {
	return map[string]any{"criterionBindings": d.Criteria, "runnerReceipts": d.Receipts, "criterionLinks": d.Links, "artifacts": d.Artifacts}
}

// EncodeStable owns every stable member and revalidates changed documents.
// The immutable OriginalBytes remain independent of this canonical encoding.
func EncodeStable(d *StableMap) ([]byte, error) {
	if d == nil || d.Change.Spec != StableSpec {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "stable encoder profile")
	}
	change := Map(d.Change)
	refs := d.StableReferences()
	refs["spec"] = StableSpec
	refs["baseRevision"] = d.Change.BaseRevision
	refs["patchSha256"] = d.Change.PatchSha256
	refs["excludedPath"] = d.Change.ExcludedPath
	refs["hunks"] = stableHunks(&change)
	evidence := []any{}
	for _, e := range d.Change.Evidence {
		evidence = append(evidence, map[string]any{"id": e.ID, "blobOid": e.BlobOid, "path": e.Path, "span": map[string]any{"start": e.Span.Start, "end": e.Span.End}, "spanSha256": e.SpanSha256})
	}
	refs["evidence"] = evidence
	b, e := json.Marshal(refs)
	if e != nil {
		return nil, e
	}
	v, e := Parse(b)
	if e != nil {
		return nil, e
	}
	encoded := append(CanonicalValue(v), '\n')
	if _, e = ParseStable(encoded); e != nil {
		return nil, e
	}
	return encoded, nil
}

func stableHunks(document *Map) []any {
	hunks := make([]any, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		basis := make([]any, 0, len(hunk.Basis))
		for _, item := range hunk.Basis {
			basis = append(basis, map[string]any{"evidenceId": item.EvidenceID, "relation": item.Relation})
		}
		entry := map[string]any{
			"basis": basis, "disposition": hunk.Disposition, "id": hunk.ID,
			"newRange": stableRangeValue(hunk.NewRange), "oldRange": stableRangeValue(hunk.OldRange),
			"path": hunk.Path, "reason": hunk.Reason,
		}
		if hunk.Coverage != nil {
			entry["coverage"] = stableCoverageValue(hunk.Coverage)
		}
		if hunk.Discriminates != nil {
			entry["discriminates"] = stableDiscriminationValue(hunk.Discriminates)
		}
		hunks = append(hunks, entry)
	}
	return hunks
}

func stableCoverageValue(witness *CoverageWitness) map[string]any {
	covered := make([]any, 0, len(witness.Covered))
	for _, item := range witness.Covered {
		covered = append(covered, stableRangeValue(item))
	}
	return map[string]any{
		"covered": covered, "mode": witness.Mode, "profileSha256": witness.ProfileSha256,
		"state": witness.State, "testRun": witness.TestRun,
	}
}

func stableDiscriminationValue(witness *DiscriminationWitness) map[string]any {
	survivors := make([]any, 0, len(witness.Survivors))
	for _, item := range witness.Survivors {
		survivors = append(survivors, map[string]any{
			"description": item.Description, "line": item.Line, "operator": item.Operator,
		})
	}
	return map[string]any{
		"bounds": map[string]any{
			"maxHunks": witness.Bounds.MaxHunks, "maxMutants": witness.Bounds.MaxMutants,
			"wallTimeSeconds": witness.Bounds.WallTimeSeconds,
		},
		"detail": witness.Detail, "killed": witness.Killed, "mutants": witness.Mutants,
		"selectionSha256": witness.SelectionSha256, "state": witness.State,
		"survived": witness.Survived, "survivors": survivors, "treeRevision": witness.TreeRevision,
	}
}

func stableRangeValue(value Range) map[string]any {
	return map[string]any{"start": value.Start, "count": value.Count}
}
