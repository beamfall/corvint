package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ReviewOCMReader verifies explicitly supplied OCM bytes with the exact retained
// CEM input. The owning adapter returns valid, obligations, claims, and binding
// metadata; CEM does not infer requirement joins or acquire an OCM dependency.
type ReviewOCMReader func(context.Context, string, string, []byte, string, string) (map[string]any, error)

func reviewDigest(value any) string {
	raw, _ := json.Marshal(value) // all projection values are JSON data
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Session) reviewProjection(ctx context.Context, raw []byte, doc *wire.Map, verdict map[string]any, options ReadOptions, retainedPatch []byte) (map[string]any, error) {
	target, _ := verdict["targetRevision"].(string)
	if target == "" && options.Target != "" {
		var err error
		target, err = s.repository.Resolve(ctx, options.Target)
		if err != nil {
			return nil, err
		}
	}
	var patchBytes []byte
	var err error
	if wire.Canonical(doc.Spec) {
		patchBytes, err = s.repository.CanonicalDiff(ctx, doc.BaseRevision, target)
	} else {
		patchBytes = retainedPatch
	}
	if err != nil {
		return nil, err
	}
	parsed := &patch.Patch{}
	// An empty canonical diff has no derived hunks to review. Verification
	// remains authoritative about the map; the shared parser still rejects
	// empty legacy input and every nonempty patch keeps its strict parsing.
	if len(patchBytes) != 0 || !wire.Canonical(doc.Spec) {
		parsed, err = patch.Parse(patchBytes)
		if err != nil {
			return nil, err
		}
	}
	patchSum := sha256.Sum256(patchBytes)
	mapSum := sha256.Sum256(raw)
	projection := map[string]any{
		"profile": "cem-review/0", "warning": reportWarning,
		"mapSha256": hex.EncodeToString(mapSum[:]), "patchSha256": hex.EncodeToString(patchSum[:]),
		"declaredPatchSha256": doc.PatchSha256, "baseRevision": doc.BaseRevision, "targetRevision": nullableText(target),
		"valid": verdict["valid"], "hunks": []any{}, "unmappedObligations": []any{}, "surplusMapHunks": []any{},
		"nonClaims": []string{"structural links do not prove semantic support", "test claims and caller supplied witnesses do not establish a current passing test"},
	}
	ocm := map[string]any{"state": "NOT_PRODUCED", "reason": "ocm-not-supplied", "obligations": []any{}, "claims": []any{}}
	if options.OCMPath != "" {
		if options.ReadOCM == nil {
			ocm["reason"] = "ocm-verifier-unavailable"
			projection["ocmValid"] = false
		} else {
			ocm, err = options.ReadOCM(ctx, s.root, options.OCMPath, append([]byte(nil), raw...), options.ExpectedBase, target)
			if err != nil {
				return nil, err
			}
			projection["ocmValid"] = ocm["valid"] == true
		}
	}
	projection["ocm"] = ocm
	obligations, _ := ocm["obligations"].([]any)
	mapped := map[string][]wire.Hunk{}
	for _, h := range doc.Hunks {
		mapped[h.ID] = append(mapped[h.ID], h)
	}
	evidence := map[string]wire.Evidence{}
	for _, e := range doc.Evidence {
		evidence[e.ID] = e
	}
	parsedIDs := map[string]bool{}
	rows := []any{}
	for ordinal, h := range parsed.Hunks {
		parsedIDs[h.ID] = true
		row := map[string]any{"ordinal": ordinal + 1, "id": h.ID, "hunkSha256": h.ContentSha256, "path": h.DisplayPath,
			"oldRange": rangeValue(h.OldRange), "newRange": rangeValue(h.NewRange), "disposition": "unknown", "reason": "hunk-not-mapped",
			"mapValid": verdict["valid"], "lineRevision": nullableText(target), "lineRange": rangeValue(h.NewRange),
			"evidence": []any{}, "obligations": []any{}, "testObservation": map[string]any{"state": "NOT_PRODUCED", "reason": "no-coverage-witness"},
			"testExecution": map[string]any{"state": "NOT_RUN", "reason": "review-does-not-execute-tests"},
		}
		if h.NewRange.Count == 0 {
			row["lineRevision"], row["lineRange"] = doc.BaseRevision, rangeValue(h.OldRange)
		}
		row["obligationsState"], row["obligationsReason"] = ocm["state"], ocm["reason"]
		matches := mapped[h.ID]
		if len(matches) == 1 {
			m := matches[0]
			row["disposition"], row["reason"] = m.Disposition, m.Reason
			bases := []any{}
			for _, b := range m.Basis {
				e, found := evidence[b.EvidenceID]
				basis := map[string]any{"id": b.EvidenceID, "relation": b.Relation}
				if found {
					basis["path"], basis["blobOid"], basis["spanSha256"] = e.Path, e.BlobOid, e.SpanSha256
					basis["span"] = map[string]any{"start": e.Span.Start, "end": e.Span.End}
					basis["driftState"] = "NOT_PRODUCED"
					if drifts, ok := verdict["drift"].([]any); ok {
						for _, item := range drifts {
							drift, _ := item.(map[string]any)
							if drift["evidenceId"] == e.ID {
								basis["driftState"] = drift["status"]
							}
						}
					}
				} else {
					basis["state"], basis["reason"] = "NOT_PRODUCED", "evidence-id-unresolved"
				}
				bases = append(bases, basis)
			}
			row["evidence"] = bases
			if m.Coverage != nil {
				row["testObservation"] = coverageValue(m.Coverage)
			}
			if m.Discriminates != nil {
				row["discrimination"] = discriminationValue(m.Discriminates)
			}
			if citesTestClaim(m) {
				row["testClaimQualification"] = testClaimOutcome(m)
			}
		} else if len(matches) > 1 {
			row["reason"] = "ambiguous-hunk-mapping"
		}
		if ocm["valid"] == true {
			linked := []any{}
			for _, item := range obligations {
				obligation, _ := item.(map[string]any)
				if containsReviewID(obligation["hunkIds"], h.ID) {
					linked = append(linked, obligation)
				}
			}
			row["obligations"] = linked
		}
		rows = append(rows, row)
	}
	projection["hunks"] = rows
	surplus := []any{}
	for _, h := range doc.Hunks {
		if !parsedIDs[h.ID] {
			surplus = append(surplus, map[string]any{"id": h.ID, "reason": "map-hunk-not-in-derived-patch"})
		}
	}
	projection["surplusMapHunks"] = surplus
	unassigned := []any{}
	if ocm["valid"] == true {
		for _, item := range obligations {
			obligation, _ := item.(map[string]any)
			if obligation["disposition"] != "linked" {
				unassigned = append(unassigned, obligation)
			}
		}
	}
	projection["unmappedObligations"] = unassigned
	projection["recordSetSha256"] = reviewDigest(projection)
	encoded, err := json.Marshal(projection)
	if err != nil || len(encoded) > maxReportBytes {
		return nil, invalidArguments("review projection exceeds report byte limit")
	}
	return projection, nil
}

func containsReviewID(value any, id string) bool {
	switch values := value.(type) {
	case []any:
		for _, value := range values {
			if value == id {
				return true
			}
		}
	case []string:
		for _, value := range values {
			if value == id {
				return true
			}
		}
	}
	return false
}

func renderReviewProjection(projection map[string]any) string {
	var out strings.Builder
	out.WriteString("\n## Hunk review projection\n\n")
	out.WriteString(fmt.Sprintf("- Record set SHA-256: `%s`\n- Map SHA-256: `%s`\n", projection["recordSetSha256"], projection["mapSha256"]))
	ocm := projection["ocm"].(map[string]any)
	if valid, found := ocm["valid"]; found {
		out.WriteString("- OCM validity: " + mdreport.CodeSpan(fmt.Sprint(valid)) + "\n")
	} else {
		out.WriteString("- OCM validity: NOT_PRODUCED\n")
	}
	if state, _ := ocm["state"].(string); state != "" {
		out.WriteString("- OCM state: " + mdreport.CodeSpan(state) + "\n")
	}
	if verification, ok := ocm["verification"].(map[string]any); ok {
		if issues, ok := verification["issues"].([]any); ok {
			for _, item := range issues {
				issue, _ := item.(map[string]any)
				out.WriteString("- OCM issue: " + mdreport.CodeSpan(fmt.Sprint(issue["code"])))
				if message, _ := issue["message"].(string); message != "" {
					out.WriteString(": " + mdreport.CodeSpan(message))
				}
				out.WriteByte('\n')
			}
		}
	}
	if reason, _ := ocm["reason"].(string); reason != "" {
		out.WriteString("- Obligations: NOT_PRODUCED " + mdreport.CodeSpan(reason) + "\n")
	}
	out.WriteString("\nEach row describes the derived patch. Test execution is NOT_RUN; supplied witnesses and structural claims do not establish a current passing test.\n\n")
	for _, entry := range projection["hunks"].([]any) {
		row := entry.(map[string]any)
		out.WriteString(fmt.Sprintf("- %v. %s %s: %s; reason %s\n", row["ordinal"], mdreport.CodeSpan(fmt.Sprint(row["path"])), mdreport.CodeSpan(fmt.Sprint(row["id"])), mdreport.CodeSpan(fmt.Sprint(row["disposition"])), mdreport.CodeSpan(fmt.Sprint(row["reason"]))))
		out.WriteString("  - Line revision " + mdreport.CodeSpan(fmt.Sprint(row["lineRevision"])) + "; range " + mdreport.CodeSpan(fmt.Sprint(row["lineRange"])) + "\n")
		if evidence, ok := row["evidence"].([]any); ok {
			for _, item := range evidence {
				basis := item.(map[string]any)
				out.WriteString("  - Evidence " + mdreport.CodeSpan(fmt.Sprint(basis["id"])) + "; relation " + mdreport.CodeSpan(fmt.Sprint(basis["relation"])) + "; blob " + mdreport.CodeSpan(fmt.Sprint(basis["blobOid"])) + "; span digest " + mdreport.CodeSpan(fmt.Sprint(basis["spanSha256"])) + "; drift " + mdreport.CodeSpan(fmt.Sprint(basis["driftState"])) + "\n")
			}
		}
		if observation, ok := row["testObservation"].(map[string]any); ok {
			out.WriteString("  - Test observation " + mdreport.CodeSpan(fmt.Sprint(observation["state"])) + "; reason " + mdreport.CodeSpan(fmt.Sprint(observation["reason"])) + "; profile " + mdreport.CodeSpan(fmt.Sprint(observation["profileSha256"])) + "\n")
		}
		if witness, ok := row["discrimination"].(map[string]any); ok {
			out.WriteString("  - Discrimination " + mdreport.CodeSpan(fmt.Sprint(witness["state"])) + "; detail " + mdreport.CodeSpan(fmt.Sprint(witness["detail"])) + "; tree " + mdreport.CodeSpan(fmt.Sprint(witness["treeRevision"])) + "; selection " + mdreport.CodeSpan(fmt.Sprint(witness["selectionSha256"])) + "\n")
		}
		for _, item := range row["obligations"].([]any) {
			obligation := item.(map[string]any)
			out.WriteString("  - Obligation " + mdreport.CodeSpan(fmt.Sprint(obligation["id"])) + "\n")
		}
	}
	for _, item := range projection["unmappedObligations"].([]any) {
		obligation := item.(map[string]any)
		out.WriteString("- Unmapped obligation " + mdreport.CodeSpan(fmt.Sprint(obligation["id"])) + ": " + mdreport.CodeSpan(fmt.Sprint(obligation["reason"])) + "\n")
	}
	return out.String()
}

// stableReviewNonClaims extend the 0.x non-claims with the stable axes.
var stableReviewNonClaims = []string{
	"structural links do not prove semantic support",
	"test claims and caller supplied witnesses do not establish a current passing test",
	"an axis the verifier reports as NOT_OBSERVED is not established",
}

// stableReviewProjection is the review of a cem/1.0 report. It is a pure
// function of the stable verifier's complete result: it reads no map,
// repository or artifact, binds the result by digest, and lists the hunks the
// map declares with the drift the verifier observed (CEM-V1-029).
func stableReviewProjection(result verify.StableResult) (map[string]any, error) {
	resultDigest, err := stableResultDigest(result)
	if err != nil {
		return nil, err
	}
	drift := map[string]string{}
	for _, entry := range result.Drift {
		drift[entry.EvidenceID] = string(entry.Status)
	}
	var hunks []map[string]any
	raw, err := json.Marshal(result.Hunks)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &hunks); err != nil {
		return nil, err
	}
	rows := []any{}
	for index, hunk := range hunks {
		evidence := []any{}
		bases, _ := hunk["basis"].([]any)
		for _, item := range bases {
			basis, _ := item.(map[string]any)
			id, _ := basis["evidenceId"].(string)
			state := "NOT_PRODUCED"
			if status, found := drift[id]; found {
				state = status
			}
			evidence = append(evidence, map[string]any{"id": id, "relation": basis["relation"], "driftState": state})
		}
		rows = append(rows, map[string]any{
			"ordinal": index + 1, "id": hunk["id"], "path": hunk["path"],
			"oldRange": hunk["oldRange"], "newRange": hunk["newRange"],
			"disposition": hunk["disposition"], "reason": hunk["reason"], "evidence": evidence,
			"coverageState":       stableClaimState(hunk["coverage"]),
			"discriminationState": stableClaimState(hunk["discriminates"]),
			"testExecution":       map[string]any{"state": "NOT_RUN", "reason": "review-does-not-execute-tests"},
		})
	}
	projection := map[string]any{
		"profile": "cem-stable-review/0", "warning": reportWarning,
		"outcome": result.Outcome, "stage": result.Stage, "code": result.Code,
		"mapSha256": result.MapSha256, "patchSha256": result.PatchSha256,
		"expectedBase": result.ExpectedBase, "targetRevision": result.TargetRevision,
		"stableResultSha256": resultDigest,
		"hunks":              rows, "nonClaims": stableReviewNonClaims,
		"ocm": map[string]any{"state": "NOT_PRODUCED", "reason": "ocm-not-supplied"},
	}
	projection["recordSetSha256"] = reviewDigest(projection)
	return projection, nil
}

// stableResultDigest is the SHA-256 of the result's key-sorted compact JSON
// without HTML escaping, so a consumer holding the embedded result can
// recompute it from that JSON alone.
func stableResultDigest(result verify.StableResult) (string, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:]), nil
}

// stableClaimState names a hunk's declared coverage or discrimination state
// as the map supplied it, or NOT_PRODUCED when the map supplied none.
func stableClaimState(value any) string {
	claim, _ := value.(map[string]any)
	if state, _ := claim["state"].(string); state != "" {
		return state
	}
	return "NOT_PRODUCED"
}

// renderStableReport renders the Markdown of a stable report from the complete
// verifier result and its review projection; every repository-derived string is
// a code span (CEM-V1-030).
func renderStableReport(result verify.StableResult, projection map[string]any) string {
	text := func(value *string) string {
		if value == nil {
			return "NOT_PRODUCED"
		}
		return mdreport.CodeSpan(*value)
	}
	var out strings.Builder
	out.WriteString("# Change Evidence Map review\n\n")
	out.WriteString("> " + reportWarning + "\n\n")
	out.WriteString("- Spec: " + mdreport.CodeSpan(result.Spec) + "\n")
	out.WriteString("- Outcome: " + mdreport.CodeSpan(result.Outcome) + "\n")
	out.WriteString("- Stage: " + mdreport.CodeSpan(result.Stage) + "\n")
	if result.Code != nil {
		out.WriteString("- Code: " + mdreport.CodeSpan(*result.Code) + "\n")
	}
	for _, code := range result.IssueCodes {
		out.WriteString("- Issue: " + mdreport.CodeSpan(code) + "\n")
	}
	out.WriteString("- Assurance: " + text(result.Assurance) + "\n")
	out.WriteString("- Expected base: " + text(result.ExpectedBase) + "\n")
	out.WriteString("- Target revision: " + text(result.TargetRevision) + "\n")
	out.WriteString("- Map SHA-256: " + text(result.MapSha256) + "\n")
	out.WriteString("- Patch SHA-256: " + text(result.PatchSha256) + "\n")
	out.WriteString("- Sidecar: " + mdreport.CodeSpan(result.Sidecar) + "\n")
	out.WriteString("- Runtime: " + mdreport.CodeSpan(result.Runtime.GoVersion) + "; structural qualification " + mdreport.CodeSpan(result.Runtime.StructuralQualification) + "\n")
	out.WriteString("- Stable result SHA-256: " + mdreport.CodeSpan(fmt.Sprint(projection["stableResultSha256"])) + "\n")
	out.WriteString("\n## Assurance axes\n\n")
	axes := make([]string, 0, len(result.Axes))
	for axis := range result.Axes {
		axes = append(axes, axis)
	}
	sort.Strings(axes)
	for _, axis := range axes {
		out.WriteString("- " + mdreport.CodeSpan(axis) + ": " + mdreport.CodeSpan(result.Axes[axis]) + "\n")
	}
	if len(result.ArtifactChecks) != 0 {
		out.WriteString("\n## Artifact checks\n\n")
		for _, check := range result.ArtifactChecks {
			out.WriteString(fmt.Sprintf("- %s %s: SHA-256 %s, %d bytes\n", mdreport.CodeSpan(check.Kind), mdreport.CodeSpan(check.Path), mdreport.CodeSpan(check.Sha256), check.ByteLength))
		}
	}
	if len(result.Drift) != 0 {
		out.WriteString("\n## Evidence drift\n\n")
		for _, entry := range result.Drift {
			out.WriteString("- " + mdreport.CodeSpan(entry.EvidenceID) + " " + mdreport.CodeSpan(entry.Path) + ": " + mdreport.CodeSpan(string(entry.Status)) + "\n")
		}
	}
	out.WriteString("\n## Hunk review projection\n\n")
	out.WriteString("- Record set SHA-256: " + mdreport.CodeSpan(fmt.Sprint(projection["recordSetSha256"])) + "\n")
	out.WriteString("- Obligations: NOT_PRODUCED `ocm-not-supplied`\n")
	out.WriteString("\nEach row describes a hunk the map declares; the outcome above says whether the verifier accepted it. Test execution is NOT_RUN; supplied witnesses and structural claims do not establish a current passing test.\n\n")
	for _, entry := range projection["hunks"].([]any) {
		row := entry.(map[string]any)
		out.WriteString(fmt.Sprintf("- %v. %s %s: %s; reason %s\n", row["ordinal"], mdreport.CodeSpan(fmt.Sprint(row["path"])), mdreport.CodeSpan(fmt.Sprint(row["id"])), mdreport.CodeSpan(fmt.Sprint(row["disposition"])), mdreport.CodeSpan(fmt.Sprint(row["reason"]))))
		for _, item := range row["evidence"].([]any) {
			basis := item.(map[string]any)
			out.WriteString("  - Evidence " + mdreport.CodeSpan(fmt.Sprint(basis["id"])) + "; relation " + mdreport.CodeSpan(fmt.Sprint(basis["relation"])) + "; drift " + mdreport.CodeSpan(fmt.Sprint(basis["driftState"])) + "\n")
		}
		out.WriteString("  - Coverage " + mdreport.CodeSpan(fmt.Sprint(row["coverageState"])) + "; discrimination " + mdreport.CodeSpan(fmt.Sprint(row["discriminationState"])) + "\n")
	}
	out.WriteString("\n## Non-claims\n\n")
	for _, claim := range stableReviewNonClaims {
		out.WriteString("- " + claim + "\n")
	}
	return out.String()
}
