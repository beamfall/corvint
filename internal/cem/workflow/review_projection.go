package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

func reviewDigest(value any) string {
	raw, _ := json.Marshal(value) // all projection values are JSON data
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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

// writeOCMSummary writes the OCM validity, state, issues and NOT_PRODUCED
// reason lines a 0.x report, and a stable report with --ocm, render.
func writeOCMSummary(out *strings.Builder, ocm map[string]any) {
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
}

// writeRowObligations writes one "Obligation" line per obligation joined to a
// hunk row; a stable row without a supplied OCM carries none.
func writeRowObligations(out *strings.Builder, row map[string]any) {
	obligations, _ := row["obligations"].([]any)
	for _, item := range obligations {
		obligation := item.(map[string]any)
		out.WriteString("  - Obligation " + mdreport.CodeSpan(fmt.Sprint(obligation["id"])) + "\n")
	}
}

// writeUnmappedObligations writes the obligations no hunk links.
func writeUnmappedObligations(out *strings.Builder, projection map[string]any) {
	unmapped, _ := projection["unmappedObligations"].([]any)
	for _, item := range unmapped {
		obligation := item.(map[string]any)
		out.WriteString("- Unmapped obligation " + mdreport.CodeSpan(fmt.Sprint(obligation["id"])) + ": " + mdreport.CodeSpan(fmt.Sprint(obligation["reason"])) + "\n")
	}
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
// map declares with the drift the verifier observed (CEM-V1-034).
func stableReviewProjection(result verify.StableResult) (map[string]any, error) {
	resultDigest, err := stableResultDigest(result)
	if err != nil {
		return nil, err
	}
	drift := map[string]verify.StableDrift{}
	for _, entry := range result.Drift {
		drift[entry.EvidenceID] = entry
	}
	citations := map[string]verify.CitationHunk{}
	if result.Citation != nil {
		for _, row := range result.Citation.Hunks {
			citations[row.HunkID] = row
		}
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
			state, location := "NOT_PRODUCED", map[string]any{"path": nil, "targetSpan": nil}
			if entry, found := drift[id]; found {
				state = string(entry.Status)
				location = map[string]any{"path": entry.Path, "targetSpan": stableSpan(entry.TargetSpan)}
			}
			evidence = append(evidence, map[string]any{"id": id, "relation": basis["relation"], "driftState": state, "location": location})
		}
		hunkID, _ := hunk["id"].(string)
		hunkPath, _ := hunk["path"].(string)
		tier, anchors := "NOT_PRODUCED", []any{}
		if row, found := citations[hunkID]; found {
			tier = row.Tier
			for _, anchor := range row.Anchors {
				anchors = append(anchors, anchor)
			}
		}
		rows = append(rows, map[string]any{
			"tier": tier, "anchors": anchors, "selfOnly": stableSelfOnly(evidence, hunkPath),
			"ordinal": index + 1, "id": hunk["id"], "path": hunk["path"],
			"oldRange": hunk["oldRange"], "newRange": hunk["newRange"],
			"disposition": hunk["disposition"], "reason": hunk["reason"], "evidence": evidence,
			"coverageState":       stableClaimState(hunk["coverage"]),
			"discriminationState": stableClaimState(hunk["discriminates"]),
			"testExecution":       map[string]any{"state": "NOT_RUN", "reason": "review-does-not-execute-tests"},
		})
	}
	sortStableRows(rows)
	projection := map[string]any{
		"citationTiers": stableCitationTiers(result.Citation),
		"profile":       "cem-stable-review/0", "warning": reportWarning,
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

// stableSpan copies a drift target span, or nil when the verifier located none.
func stableSpan(span map[string]int64) any {
	if span == nil {
		return nil
	}
	return map[string]any{"start": span["start"], "end": span["end"]}
}

// stableSelfOnly reports a hunk whose every cited evidence span lies in the
// hunk's own file: a citation that restates the change rather than supporting it.
func stableSelfOnly(evidence []any, hunkPath string) bool {
	if len(evidence) == 0 {
		return false
	}
	for _, item := range evidence {
		location, _ := item.(map[string]any)["location"].(map[string]any)
		if path, _ := location["path"].(string); path != hunkPath {
			return false
		}
	}
	return true
}

// sortStableRows orders the weakest support first (CEM-V1-368): unknown or
// unproduced tiers, then self-only citations, then anchor-less citations, then
// the rest; map order breaks ties, and each row keeps its map ordinal.
func sortStableRows(rows []any) {
	rank := func(row map[string]any) int {
		switch {
		case row["tier"] == verify.TierUnknown || row["tier"] == "NOT_PRODUCED":
			return 0
		case row["selfOnly"] == true:
			return 1
		case row["tier"] == verify.TierCited && len(row["anchors"].([]any)) == 0:
			return 2
		}
		return 3
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rank(rows[i].(map[string]any)) < rank(rows[j].(map[string]any))
	})
}

// stableCitationTiers is the verifier's per-tier hunk count and same-path
// citation count, or NOT_PRODUCED when the verifier derived no citation.
func stableCitationTiers(citation *verify.Citation) any {
	if citation == nil {
		return "NOT_PRODUCED"
	}
	return map[string]any{
		verify.TierHeld: citation.HunksByClass.Held, verify.TierMechanicalProved: citation.HunksByClass.MechanicalProved,
		verify.TierAnchored: citation.HunksByClass.Anchored, verify.TierCited: citation.HunksByClass.Cited,
		verify.TierUnknown: citation.HunksByClass.Unknown, "samePathCitations": citation.SamePathCitations,
		"hunks": len(citation.Hunks),
	}
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
// a code span (CEM-V1-035).
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
	if _, supplied := projection["ocmValid"]; supplied {
		writeOCMSummary(&out, projection["ocm"].(map[string]any))
	} else {
		out.WriteString("- Obligations: NOT_PRODUCED `ocm-not-supplied`\n")
	}
	writeStableTiers(&out, projection["citationTiers"])
	out.WriteString("\nEach row describes a hunk the map declares; the outcome above says whether the verifier accepted it. Test execution is NOT_RUN; supplied witnesses and structural claims do not establish a current passing test.\n\n")
	for _, entry := range projection["hunks"].([]any) {
		row := entry.(map[string]any)
		out.WriteString(fmt.Sprintf("- %v. %s %s: %s; reason %s\n", row["ordinal"], mdreport.CodeSpan(fmt.Sprint(row["path"])), mdreport.CodeSpan(fmt.Sprint(row["id"])), mdreport.CodeSpan(fmt.Sprint(row["disposition"])), mdreport.CodeSpan(fmt.Sprint(row["reason"]))))
		out.WriteString("  - Tier " + mdreport.CodeSpan(fmt.Sprint(row["tier"])) + "; anchors " + stableAnchors(row["anchors"].([]any)))
		if row["selfOnly"] == true {
			out.WriteString("; self-only citation")
		}
		out.WriteString("\n")
		for _, item := range row["evidence"].([]any) {
			basis := item.(map[string]any)
			out.WriteString("  - Evidence " + mdreport.CodeSpan(fmt.Sprint(basis["id"])) + " at " + stableLocation(basis["location"].(map[string]any)) + "; relation " + mdreport.CodeSpan(fmt.Sprint(basis["relation"])) + "; drift " + mdreport.CodeSpan(fmt.Sprint(basis["driftState"])) + "\n")
		}
		out.WriteString("  - Coverage " + mdreport.CodeSpan(fmt.Sprint(row["coverageState"])) + "; discrimination " + mdreport.CodeSpan(fmt.Sprint(row["discriminationState"])) + "\n")
		writeRowObligations(&out, row)
	}
	writeUnmappedObligations(&out, projection)
	writeDeferredIntents(&out, projection["deferredIntents"])
	out.WriteString("\n## Non-claims\n\n")
	for _, claim := range stableReviewNonClaims {
		out.WriteString("- " + claim + "\n")
	}
	return out.String()
}

// writeStableTiers renders the verifier's citation tier counts so a map whose
// hunks are all merely cited reads differently from an anchored one.
func writeStableTiers(out *strings.Builder, value any) {
	tiers, ok := value.(map[string]any)
	if !ok {
		out.WriteString("- Citation tiers: NOT_PRODUCED\n")
		return
	}
	fmt.Fprintf(out, "- Citation tiers: held %v, mechanical-proved %v, anchored %v, cited %v, unknown %v of %v hunks; same-path citations %v\n",
		tiers[verify.TierHeld], tiers[verify.TierMechanicalProved], tiers[verify.TierAnchored], tiers[verify.TierCited], tiers[verify.TierUnknown], tiers["hunks"], tiers["samePathCitations"])
}

func stableAnchors(anchors []any) string {
	if len(anchors) == 0 {
		return "none"
	}
	spans := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		spans = append(spans, mdreport.CodeSpan(fmt.Sprint(anchor)))
	}
	return strings.Join(spans, ", ")
}

// stableLocation renders an evidence span as path and target byte range; the
// projection reads no repository, so lines and quoted text are not available.
func stableLocation(location map[string]any) string {
	path, _ := location["path"].(string)
	if path == "" {
		return "NOT_PRODUCED"
	}
	span, _ := location["targetSpan"].(map[string]any)
	if span == nil {
		return mdreport.CodeSpan(path) + " bytes NOT_LOCATED"
	}
	return mdreport.CodeSpan(path) + fmt.Sprintf(" bytes %v-%v", span["start"], span["end"])
}

// applyStableOCM joins a supplied OCM projection onto a stable review exactly
// as a 0.x review does: ocm and ocmValid, per-hunk obligations with their state
// and reason, and unmappedObligations, then re-binds the record set digest. A
// report without --ocm never calls it, so its projection is unchanged
// (V1-1305).
func applyStableOCM(projection, ocm map[string]any, verifier bool) {
	valid := verifier && ocm["valid"] == true
	obligations, _ := ocm["obligations"].([]any)
	projection["ocm"] = ocm
	projection["ocmValid"] = valid
	for _, entry := range projection["hunks"].([]any) {
		row := entry.(map[string]any)
		id, _ := row["id"].(string)
		linked := []any{}
		if valid {
			for _, item := range obligations {
				obligation, _ := item.(map[string]any)
				if containsReviewID(obligation["hunkIds"], id) {
					linked = append(linked, obligation)
				}
			}
		}
		row["obligations"] = linked
		row["obligationsState"], row["obligationsReason"] = ocm["state"], ocm["reason"]
	}
	unmapped := []any{}
	if valid {
		for _, item := range obligations {
			obligation, _ := item.(map[string]any)
			if obligation["disposition"] != "linked" {
				unmapped = append(unmapped, obligation)
			}
		}
	}
	projection["unmappedObligations"] = unmapped
	delete(projection, "recordSetSha256")
	projection["recordSetSha256"] = reviewDigest(projection)
}

// writeDeferredIntents renders the change's recorded intent-scope deferral
// (CEM-V1-646). The section appears only when the map's hunks name a record or
// a bound was supplied; every repository-derived string is a code span.
func writeDeferredIntents(out *strings.Builder, value any) {
	member, ok := value.(map[string]any)
	if !ok {
		return
	}
	out.WriteString("\n## Deferred intent scopes\n\n")
	record := "none"
	if path, ok := member["record"].(string); ok {
		record = mdreport.CodeSpan(path)
	}
	fmt.Fprintf(out, "- State: %s; %v deferred of at most %v; record %s\n", mdreport.CodeSpan(fmt.Sprint(member["state"])), member["count"], member["max"], record)
	out.WriteString("- These intent scopes were deferred: their obligations are not mapped in this change's review.\n")
	for _, entry := range member["entries"].([]any) {
		row := entry.(map[string]any)
		base := "absent at base"
		if oid, ok := row["baseBlobOid"].(string); ok {
			base = "base blob " + mdreport.CodeSpan(oid)
		}
		fmt.Fprintf(out, "- %s: %v obligations (%s), %s (%s); reason %s\n",
			mdreport.CodeSpan(fmt.Sprint(row["path"])), row["obligationCount"], mdreport.CodeSpan(fmt.Sprint(row["countState"])),
			base, mdreport.CodeSpan(fmt.Sprint(row["baseState"])), mdreport.CodeSpan(fmt.Sprint(row["reason"])))
	}
	for _, entry := range member["issues"].([]any) {
		item := entry.(map[string]any)
		line := "- Issue: " + mdreport.CodeSpan(fmt.Sprint(item["code"]))
		if path, _ := item["path"].(string); path != "" {
			line += " " + mdreport.CodeSpan(path)
		}
		out.WriteString(line + "\n")
	}
}
